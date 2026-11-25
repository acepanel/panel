package ipset

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/shell"
)

// tableName 面板独占的 nftables 表，firewalld 与 ufw 的重载都不会动它
const tableName = "acepanel"

// reservedV4 / reservedV6 私有与保留地址，始终免于黑名单拦截，避免 bogon 列表断掉回环、内网和容器
const (
	reservedV4 = "{ 0.0.0.0/8, 10.0.0.0/8, 100.64.0.0/10, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.168.0.0/16, 224.0.0.0/3 }"
	reservedV6 = "{ ::/127, fc00::/7, fe80::/10 }"
)

// forwardChain 只接管从外部经 DNAT 转发进网桥的新连接，也就是容器的映射端口，宿主机自己的端口仍归系统防火墙
// 容器之间经宿主机地址互访时入口也是网桥，不算外部
const forwardChain = `	chain forward {
		type filter hook forward priority filter; policy accept;
		ct state established,related accept
		ct status dnat meta oifkind "bridge" meta iifkind != "bridge" jump mapped
	}
`

// outboundChain 拦截本机发往屏蔽目标的新建出站连接，仅在存在出站屏蔽地址时下发
const outboundChain = `	chain outbound {
		type filter hook output priority filter; policy accept;
		oifname "lo" accept
		ct state established,related accept
		ip daddr ` + reservedV4 + ` accept
		ip6 daddr ` + reservedV6 + ` accept
		ip daddr @block_out4 drop
		ip6 daddr @block_out6 drop
	}
`

// Ruleset 一次下发的全部内容
type Ruleset struct {
	Allow      Set                 // 不受 Block 影响的来源（白名单订阅与全端口放行规则）
	Block      Set                 // 新建入站连接会被丢弃的来源（黑名单订阅与扫描自动屏蔽）
	BlockOut   Set                 // 本机新建出站连接会被丢弃的目标（扫描自动屏蔽出站）
	AllowRules []firewall.FireInfo // 带端口或协议限制的自定义入站放行规则，仅在对应端口豁免黑名单
	// Mapped 为 true 时，从外部访问容器的映射端口也要有 Rules 里的放行规则
	Mapped bool
	Rules  []firewall.FireInfo
}

// Table 面板独占的 nftables 过滤表
// 表里的 accept 只是不再匹配本表，包仍要过系统防火墙和 Docker 自己的规则，所以这张表只能收紧不能放行
type Table struct{}

// Available nft 命令是否存在
func (Table) Available() bool {
	_, err := exec.LookPath("nft")
	return err == nil
}

// Exists 表是否已下发
func (t Table) Exists(ctx context.Context) bool {
	if !t.Available() {
		return false
	}
	// 只列表名，不带 -t 列整张表：旧版 nft 即使不输出也会把集合元素全部读一遍
	out, err := t.exec(ctx, nil, "list", "tables")
	if err != nil {
		return false
	}

	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) == "table inet "+tableName {
			return true
		}
	}
	return false
}

// Apply 整表替换
func (t Table) Apply(ctx context.Context, ruleset Ruleset) error {
	f, err := os.CreateTemp("", "acepanel-nft-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()

	err = t.render(f, ruleset)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	// 整个文件是一次事务，要么全部生效要么保持原样
	_, err = t.exec(ctx, nil, "-f", f.Name())
	return err
}

// Delete 幂等删除整张表，表不存在时也不报错
func (t Table) Delete(ctx context.Context) error {
	script := fmt.Sprintf("table inet %s\ndelete table inet %s\n", tableName, tableName)
	_, err := t.exec(ctx, strings.NewReader(script), "-f", "-")
	return err
}

func (Table) exec(ctx context.Context, stdin io.Reader, args ...string) (string, error) {
	cmd := shell.Command(ctx, "nft", args...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), fmt.Errorf(
			"run nft %s failed, err: %w, stderr: %s",
			strings.Join(args, " "),
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (t Table) render(f *os.File, ruleset Ruleset) error {
	w := bufio.NewWriter(f)

	// 先声明再删除，表不存在时删除也不会报错
	_, _ = fmt.Fprintf(w, "table inet %s\ndelete table inet %s\ntable inet %s {\n", tableName, tableName, tableName)
	t.renderSet(w, "allow4", "ipv4_addr", ruleset.Allow.v4)
	t.renderSet(w, "allow6", "ipv6_addr", ruleset.Allow.v6)
	t.renderSet(w, "block4", "ipv4_addr", ruleset.Block.v4)
	t.renderSet(w, "block6", "ipv6_addr", ruleset.Block.v6)
	t.renderInbound(w, ruleset.AllowRules)
	if ruleset.BlockOut.Len() > 0 {
		t.renderSet(w, "block_out4", "ipv4_addr", ruleset.BlockOut.v4)
		t.renderSet(w, "block_out6", "ipv6_addr", ruleset.BlockOut.v6)
		_, _ = w.WriteString(outboundChain)
	}
	if ruleset.Mapped {
		t.renderMapped(w, ruleset.Rules)
	}
	_, _ = w.WriteString("}\n")

	return w.Flush()
}

// renderInbound 挂在 prerouting 才能连同转发给容器的流量一起拦住，优先级 mangle 排在连接跟踪之后、DNAT 之前
func (t Table) renderInbound(w *bufio.Writer, allowRules []firewall.FireInfo) {
	_, _ = w.WriteString("\tchain inbound {\n" +
		"\t\ttype filter hook prerouting priority mangle; policy accept;\n" +
		"\t\tiifname \"lo\" accept\n" +
		"\t\tct state established,related accept\n" +
		"\t\tip saddr " + reservedV4 + " accept\n" +
		"\t\tip6 saddr " + reservedV6 + " accept\n" +
		"\t\tip saddr @allow4 accept\n" +
		"\t\tip6 saddr @allow6 accept\n")
	for _, line := range t.renderAllowRules(allowRules) {
		_, _ = fmt.Fprintf(w, "\t\t%s accept\n", line)
	}
	_, _ = w.WriteString("\t\tip saddr @block4 drop\n" +
		"\t\tip6 saddr @block6 drop\n" +
		"\t}\n")
}

// renderAllowRules 带端口或协议限制的自定义放行规则，只在对应端口豁免黑名单
func (t Table) renderAllowRules(rules []firewall.FireInfo) []string {
	protocols := map[firewall.Protocol][]string{
		firewall.ProtocolTCP:    {"tcp"},
		firewall.ProtocolUDP:    {"udp"},
		firewall.ProtocolTCPUDP: {"tcp", "udp"},
	}

	seen := make(map[string]struct{})
	var lines []string
	for _, rule := range rules {
		if rule.Direction != firewall.DirectionIn || rule.Strategy != firewall.StrategyAccept || rule.Address == "" {
			continue
		}
		var source string
		if set := Of(rule.Address); len(set.v4) > 0 {
			source = "ip saddr " + set.v4[0].String() + " "
		} else if len(set.v6) > 0 {
			source = "ip6 saddr " + set.v6[0].String() + " "
		} else {
			continue
		}

		allPorts := rule.PortStart == 1 && rule.PortEnd == 65535
		for _, protocol := range protocols[rule.Protocol] {
			var line string
			switch {
			case allPorts:
				line = source + "meta l4proto " + protocol
			case rule.PortStart == rule.PortEnd:
				line = fmt.Sprintf("%s%s dport %d", source, protocol, rule.PortStart)
			default:
				line = fmt.Sprintf("%s%s dport %d-%d", source, protocol, rule.PortStart, rule.PortEnd)
			}
			if _, ok := seen[line]; !ok {
				seen[line] = struct{}{}
				lines = append(lines, line)
			}
		}
	}
	return lines
}

// renderMapped 映射端口的放行链：和系统防火墙一样先拒绝后放行，都没命中的丢弃，拒绝也按丢弃处理
// 对全部端口生效的来源进集合（自动屏蔽的地址可能很多），其余每条规则一行
func (t Table) renderMapped(w *bufio.Writer, rules []firewall.FireInfo) {
	protocols := map[firewall.Protocol][]string{
		firewall.ProtocolTCP:    {"tcp"},
		firewall.ProtocolUDP:    {"udp"},
		firewall.ProtocolTCPUDP: {"tcp", "udp"},
	}

	var trusted, banned, accepts, drops []string
	seenAccept := make(map[string]struct{})
	seenDrop := make(map[string]struct{})
	for _, rule := range rules {
		accept := rule.Strategy == firewall.StrategyAccept
		deny := rule.Strategy == firewall.StrategyDrop || rule.Strategy == firewall.StrategyReject
		if rule.Direction != firewall.DirectionIn || (!accept && !deny) {
			continue
		}

		allPorts := rule.PortStart == 1 && rule.PortEnd == 65535
		if rule.Address != "" && allPorts && rule.Protocol == firewall.ProtocolTCPUDP {
			if accept {
				trusted = append(trusted, rule.Address)
			} else {
				banned = append(banned, rule.Address)
			}
			continue
		}

		// 普通端口规则不分地址族（firewalld 的端口规则同时作用于 IPv4 和 IPv6）；富规则保留其显式地址族
		var source, ports string
		set := Of(rule.Address)
		switch {
		case len(set.v4) > 0:
			source = "ip saddr " + set.v4[0].String() + " "
		case len(set.v6) > 0:
			source = "ip6 saddr " + set.v6[0].String() + " "
		case rule.Address != "":
			continue
		case rule.Type == firewall.TypeRich && (rule.Family == "ipv4" || rule.Family == "ipv6"):
			source = "meta nfproto " + rule.Family + " "
		}
		// 匹配 DNAT 之前的目的端口，也就是宿主机上映射出去的那个端口
		if rule.PortStart == rule.PortEnd {
			ports = fmt.Sprintf(" ct original proto-dst %d", rule.PortStart)
		} else if !allPorts {
			ports = fmt.Sprintf(" ct original proto-dst %d-%d", rule.PortStart, rule.PortEnd)
		}
		for _, protocol := range protocols[rule.Protocol] {
			line := source + "meta l4proto " + protocol + ports
			if accept {
				if _, ok := seenAccept[line]; !ok {
					seenAccept[line] = struct{}{}
					accepts = append(accepts, line)
				}
			} else {
				if _, ok := seenDrop[line]; !ok {
					seenDrop[line] = struct{}{}
					drops = append(drops, line)
				}
			}
		}
	}

	trustedSet, bannedSet := Of(trusted...), Of(banned...)
	t.renderSet(w, "trusted4", "ipv4_addr", trustedSet.v4)
	t.renderSet(w, "trusted6", "ipv6_addr", trustedSet.v6)
	t.renderSet(w, "banned4", "ipv4_addr", bannedSet.v4)
	t.renderSet(w, "banned6", "ipv6_addr", bannedSet.v6)
	_, _ = w.WriteString(forwardChain)

	_, _ = w.WriteString("\tchain mapped {\n\t\tip saddr @banned4 drop\n\t\tip6 saddr @banned6 drop\n")
	for _, line := range drops {
		_, _ = fmt.Fprintf(w, "\t\t%s drop\n", line)
	}
	_, _ = w.WriteString("\t\tip saddr @trusted4 accept\n\t\tip6 saddr @trusted6 accept\n")
	for _, line := range accepts {
		_, _ = fmt.Fprintf(w, "\t\t%s accept\n", line)
	}
	_, _ = w.WriteString("\t\tdrop\n\t}\n")
}

// renderSet 空集合也要声明，链里的规则引用着它
func (t Table) renderSet(w *bufio.Writer, name, typ string, ranges []Range) {
	_, _ = fmt.Fprintf(w, "\tset %s {\n\t\ttype %s\n\t\tflags interval\n", name, typ)
	if len(ranges) > 0 {
		_, _ = w.WriteString("\t\telements = { ")
		for i, rg := range ranges {
			if i > 0 {
				_, _ = w.WriteString(",\n\t\t\t")
			}
			_, _ = w.WriteString(rg.String())
		}
		_, _ = w.WriteString(" }\n")
	}
	_, _ = w.WriteString("\t}\n")
}

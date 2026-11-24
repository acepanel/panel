package ipset

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/shell"
)

// tableName 面板独占的 nftables 表，firewalld 与 ufw 的重载都不会动它
const tableName = "acepanel"

// inboundChain 挂在 prerouting 才能连同转发给容器的流量一起拦住，优先级 mangle 排在连接跟踪之后、DNAT 之前
// 私有与保留地址始终放过：列表里常带 bogon 段，按它丢包会断掉回环、内网和容器
const inboundChain = `	chain inbound {
		type filter hook prerouting priority mangle; policy accept;
		iifname "lo" accept
		ct state established,related accept
		ip saddr { 0.0.0.0/8, 10.0.0.0/8, 100.64.0.0/10, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.168.0.0/16, 224.0.0.0/3 } accept
		ip6 saddr { ::/127, fc00::/7, fe80::/10 } accept
		ip saddr @allow4 accept
		ip6 saddr @allow6 accept
		ip saddr @block4 drop
		ip6 saddr @block6 drop
	}
`

// forwardChain 只接管从外部经 DNAT 转发进网桥的新连接，也就是容器的映射端口，宿主机自己的端口仍归系统防火墙
// 容器之间经宿主机地址互访时入口也是网桥，不算外部
const forwardChain = `	chain forward {
		type filter hook forward priority filter; policy accept;
		ct state established,related accept
		ct status dnat meta oifkind "bridge" meta iifkind != "bridge" jump mapped
	}
`

// Ruleset 一次下发的全部内容
type Ruleset struct {
	Allow Set // 不受 Block 影响的来源
	Block Set // 新建连接会被丢弃的来源
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
	out, err := shell.Execf(ctx, "nft list tables")
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
	_, err = shell.Execf(ctx, "nft -f %s", f.Name())
	return err
}

// Delete 删除整张表
func (t Table) Delete(ctx context.Context) error {
	_, err := shell.Execf(ctx, "nft delete table inet %s", tableName)
	return err
}

func (t Table) render(f *os.File, ruleset Ruleset) error {
	w := bufio.NewWriter(f)

	// 先声明再删除，表不存在时删除也不会报错
	_, _ = fmt.Fprintf(w, "table inet %s\ndelete table inet %s\ntable inet %s {\n", tableName, tableName, tableName)
	t.renderSet(w, "allow4", "ipv4_addr", ruleset.Allow.v4)
	t.renderSet(w, "allow6", "ipv6_addr", ruleset.Allow.v6)
	t.renderSet(w, "block4", "ipv4_addr", ruleset.Block.v4)
	t.renderSet(w, "block6", "ipv6_addr", ruleset.Block.v6)
	_, _ = w.WriteString(inboundChain)
	if ruleset.Mapped {
		t.renderMapped(w, ruleset.Rules)
	}
	_, _ = w.WriteString("}\n")

	return w.Flush()
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

		// 不带地址的规则不分地址族：firewalld 的端口规则本来就同时作用于 IPv4 和 IPv6
		var source, ports string
		if set := Of(rule.Address); len(set.v4) > 0 {
			source = "ip saddr " + set.v4[0].String() + " "
		} else if len(set.v6) > 0 {
			source = "ip6 saddr " + set.v6[0].String() + " "
		} else if rule.Address != "" {
			continue
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
				accepts = append(accepts, line)
			} else {
				drops = append(drops, line)
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

package ipset

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// Table 基于 nftables 的来源地址过滤表
// 表里的 accept 只是不再匹配本表，包仍要过系统防火墙，所以 allow 是豁免而不是放行
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

// Apply 整表替换：新建连接的来源在 block 里且不在 allow 里时丢弃
func (t Table) Apply(ctx context.Context, allow, block Set) error {
	f, err := os.CreateTemp("", "acepanel-nft-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()

	err = t.render(f, allow, block)
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

func (t Table) render(f *os.File, allow, block Set) error {
	w := bufio.NewWriter(f)

	// 先声明再删除，表不存在时删除也不会报错
	_, _ = fmt.Fprintf(w, "table inet %s\ndelete table inet %s\ntable inet %s {\n", tableName, tableName, tableName)
	t.renderSet(w, "allow4", "ipv4_addr", allow.v4)
	t.renderSet(w, "allow6", "ipv6_addr", allow.v6)
	t.renderSet(w, "block4", "ipv4_addr", block.v4)
	t.renderSet(w, "block6", "ipv6_addr", block.v6)
	_, _ = w.WriteString(inboundChain)
	_, _ = w.WriteString("}\n")

	return w.Flush()
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

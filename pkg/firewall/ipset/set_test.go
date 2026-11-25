package ipset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/firewall"
)

// lines 集合写出的各行
func lines(t *testing.T, set Set) []string {
	t.Helper()
	var sb strings.Builder
	_, err := set.WriteTo(&sb)
	must.NoError(t, err)
	return strings.Fields(sb.String())
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "地址、CIDR 与地址段",
			input: "1.2.3.4\n5.6.7.0/24\n9.9.9.1-9.9.9.9\n",
			want:  []string{"1.2.3.4", "5.6.7.0-5.6.7.255", "9.9.9.1-9.9.9.9"},
		},
		{
			name:  "注释、空行与行内附加内容",
			input: "\uFEFF# firehol_level1\n\n; Spamhaus DROP\n\uFEFF1.2.3.4 # scanner\n5.6.7.0/24 ; SBL123\n8.8.8.8\tgoogle\n9.9.9.9,quad9\n10.0.0.1 - 10.0.0.5 # spaced range\n10.0.1.1 - not-an-ip\n",
			want:  []string{"1.2.3.4", "5.6.7.0-5.6.7.255", "8.8.8.8", "9.9.9.9", "10.0.0.1-10.0.0.5", "10.0.1.1"},
		},
		{
			name:  "认不出的行与全地址空间条目跳过",
			input: "<html>\nexample.com\n1.2.3.4/33\n9.9.9.9-1.1.1.1\n1.2.3.4-::1\nfe80::1%eth0\n0.0.0.0/0\n::/0\n0.0.0.0-255.255.255.255\n1.2.3.4\n",
			want:  []string{"1.2.3.4"},
		},
		{
			name:  "合并后覆盖全地址空间的段丢弃",
			input: "0.0.0.0/1\n128.0.0.0/1\n2001:db8::1\n",
			want:  []string{"2001:db8::1"},
		},
		{
			name:  "IPv6 bogon 映射前缀不还原成全 IPv4",
			input: "::ffff:0:0/96\n::ffff:0.0.0.0-::ffff:255.255.255.255\n::ffff:9.9.9.0/120\n",
			want:  []string{"9.9.9.0-9.9.9.255", "::ffff:0.0.0.0-::ffff:255.255.255.255"},
		},
		{
			name:  "超长行跳过不报错",
			input: strings.Repeat("a", 80*1024) + "\n1.2.3.4\n",
			want:  []string{"1.2.3.4"},
		},
		{
			name:  "CIDR 的主机位清零",
			input: "10.1.2.3/8\n",
			want:  []string{"10.0.0.0-10.255.255.255"},
		},
		{
			name:  "重叠与相邻的地址段合并",
			input: "1.1.1.0/24\n1.1.1.128/25\n1.1.2.0/24\n1.1.4.0\n1.1.3.255\n",
			want:  []string{"1.1.1.0-1.1.2.255", "1.1.3.255-1.1.4.0"},
		},
		{
			name:  "IPv4 在前 IPv6 在后",
			input: "2001:db8::/32\n1.2.3.4\n2001:db8::1\n::ffff:5.6.7.8\n::ffff:9.9.9.0/120\n",
			want:  []string{"1.2.3.4", "5.6.7.8", "9.9.9.0-9.9.9.255", "2001:db8::-2001:db8:ffff:ffff:ffff:ffff:ffff:ffff"},
		},
		{
			name:  "到地址空间末尾的段吞掉后面的",
			input: "240.0.0.0/4\n255.255.255.255\n250.0.0.1\n",
			want:  []string{"240.0.0.0-255.255.255.255"},
		},
		{
			name:  "Windows 换行",
			input: "1.2.3.4\r\n5.6.7.8\r\n",
			want:  []string{"1.2.3.4", "5.6.7.8"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := Parse(strings.NewReader(tt.input))
			must.NoError(t, err)
			check.DeepEqual(t, lines(t, set), tt.want)
			check.Equal(t, set.Len(), len(tt.want))
		})
	}
}

func TestParseRoundTrip(t *testing.T) {
	set := Of("1.2.3.4", "5.6.7.0/24", "2001:db8::/48", "9.9.9.1-9.9.9.9")
	var sb strings.Builder
	_, err := set.WriteTo(&sb)
	must.NoError(t, err)

	parsed, err := Parse(strings.NewReader(sb.String()))
	must.NoError(t, err)
	check.DeepEqual(t, lines(t, parsed), lines(t, set))
}

func TestUnion(t *testing.T) {
	set := Union(Of("1.1.1.0/24", "2001:db8::1"), Of("1.1.2.0/24", "8.8.8.8"), Set{})
	check.DeepEqual(t, lines(t, set), []string{"1.1.1.0-1.1.2.255", "8.8.8.8", "2001:db8::1"})
}

// rendered 表渲染出的全文
func rendered(t *testing.T, ruleset Ruleset) string {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "table.nft"))
	must.NoError(t, err)
	must.NoError(t, Table{}.render(f, ruleset))
	must.NoError(t, f.Close())

	content, err := os.ReadFile(f.Name())
	must.NoError(t, err)
	return string(content)
}

func TestTableRender(t *testing.T) {
	// 没开映射端口管控时规则不进表；带端口限制的放行规则只在对应端口豁免黑名单
	content := rendered(t, Ruleset{
		Allow: Of("192.0.2.1"),
		Block: Of("1.2.3.4", "5.6.7.0/24", "2001:db8::/32"),
		AllowRules: []firewall.FireInfo{
			{Address: "1.2.3.4", PortStart: 443, PortEnd: 443, Protocol: firewall.ProtocolTCP, Strategy: firewall.StrategyAccept, Direction: firewall.DirectionIn},
			{Address: "2001:db8::1", PortStart: 8000, PortEnd: 8100, Protocol: firewall.ProtocolUDP, Strategy: firewall.StrategyAccept, Direction: firewall.DirectionIn},
		},
		BlockOut: Of("198.51.100.2", "2001:db8:bad::1"),
		Rules:    []firewall.FireInfo{{PortStart: 80, PortEnd: 80, Protocol: firewall.ProtocolTCP, Strategy: firewall.StrategyAccept, Direction: firewall.DirectionIn}},
	})
	check.Equal(t, content, `table inet acepanel
delete table inet acepanel
table inet acepanel {
	set allow4 {
		type ipv4_addr
		flags interval
		elements = { 192.0.2.1 }
	}
	set allow6 {
		type ipv6_addr
		flags interval
	}
	set block4 {
		type ipv4_addr
		flags interval
		elements = { 1.2.3.4,
			5.6.7.0-5.6.7.255 }
	}
	set block6 {
		type ipv6_addr
		flags interval
		elements = { 2001:db8::-2001:db8:ffff:ffff:ffff:ffff:ffff:ffff }
	}
	chain inbound {
		type filter hook prerouting priority mangle; policy accept;
		iifname "lo" accept
		ct state established,related accept
		ip saddr `+reservedV4+` accept
		ip6 saddr `+reservedV6+` accept
		ip saddr @allow4 accept
		ip6 saddr @allow6 accept
		ip saddr 1.2.3.4 tcp dport 443 accept
		ip6 saddr 2001:db8::1 udp dport 8000-8100 accept
		ip saddr @block4 drop
		ip6 saddr @block6 drop
	}
	set block_out4 {
		type ipv4_addr
		flags interval
		elements = { 198.51.100.2 }
	}
	set block_out6 {
		type ipv6_addr
		flags interval
		elements = { 2001:db8:bad::1 }
	}
`+outboundChain+"}\n")
}

func TestTableRenderMapped(t *testing.T) {
	rule := func(address string, start, end uint, protocol firewall.Protocol, strategy firewall.Strategy) firewall.FireInfo {
		return firewall.FireInfo{Type: firewall.TypeNormal, Address: address, PortStart: start, PortEnd: end, Protocol: protocol, Strategy: strategy, Direction: firewall.DirectionIn}
	}
	richV6Drop := firewall.FireInfo{Type: firewall.TypeRich, Family: "ipv6", PortStart: 9000, PortEnd: 9000, Protocol: firewall.ProtocolTCP, Strategy: firewall.StrategyDrop, Direction: firewall.DirectionIn}
	outbound := rule("9.9.9.9", 1, 65535, firewall.ProtocolTCPUDP, firewall.StrategyDrop)
	outbound.Direction = firewall.DirectionOut

	content := rendered(t, Ruleset{Mapped: true, Rules: []firewall.FireInfo{
		rule("", 80, 80, firewall.ProtocolTCP, firewall.StrategyAccept),
		// ufw 会为同一端口返回 ipv4/ipv6 两条普通规则，应去重
		{Type: firewall.TypeNormal, Family: "ipv6", PortStart: 80, PortEnd: 80, Protocol: firewall.ProtocolTCP, Strategy: firewall.StrategyAccept, Direction: firewall.DirectionIn},
		rule("", 8000, 8100, firewall.ProtocolTCPUDP, firewall.StrategyAccept),
		rule("10.0.0.0/8", 3306, 3306, firewall.ProtocolTCP, firewall.StrategyAccept),
		rule("2001:db8::1", 53, 53, firewall.ProtocolUDP, firewall.StrategyAccept),
		rule("", 8080, 8080, firewall.ProtocolTCP, firewall.StrategyDrop),
		richV6Drop,
		rule("198.51.100.0/24", 443, 443, firewall.ProtocolTCP, firewall.StrategyReject),
		// 对全部端口生效的来源进集合，只限一种协议的仍是单独一行
		rule("192.0.2.1", 1, 65535, firewall.ProtocolTCPUDP, firewall.StrategyAccept),
		rule("203.0.113.0/24", 1, 65535, firewall.ProtocolTCPUDP, firewall.StrategyDrop),
		rule("2001:db8:bad::/48", 1, 65535, firewall.ProtocolTCPUDP, firewall.StrategyReject),
		rule("203.0.113.9", 1, 65535, firewall.ProtocolTCP, firewall.StrategyDrop),
		// 出站、标记、别的协议和认不出的来源都与映射端口无关
		outbound,
		rule("", 22, 22, firewall.ProtocolTCP, firewall.StrategyMark),
		rule("", 1, 65535, "icmp", firewall.StrategyDrop),
		rule("ipset:office", 22, 22, firewall.ProtocolTCP, firewall.StrategyAccept),
	}})

	_, mapped, found := strings.Cut(content, "\t\tip6 saddr @block6 drop\n\t}\n")
	must.True(t, found)
	check.Equal(t, mapped, `	set trusted4 {
		type ipv4_addr
		flags interval
		elements = { 192.0.2.1 }
	}
	set trusted6 {
		type ipv6_addr
		flags interval
	}
	set banned4 {
		type ipv4_addr
		flags interval
		elements = { 203.0.113.0-203.0.113.255 }
	}
	set banned6 {
		type ipv6_addr
		flags interval
		elements = { 2001:db8:bad::-2001:db8:bad:ffff:ffff:ffff:ffff:ffff }
	}
`+forwardChain+`	chain mapped {
		ip saddr @banned4 drop
		ip6 saddr @banned6 drop
		meta l4proto tcp ct original proto-dst 8080 drop
		meta nfproto ipv6 meta l4proto tcp ct original proto-dst 9000 drop
		ip saddr 198.51.100.0-198.51.100.255 meta l4proto tcp ct original proto-dst 443 drop
		ip saddr 203.0.113.9 meta l4proto tcp drop
		ip saddr @trusted4 accept
		ip6 saddr @trusted6 accept
		meta l4proto tcp ct original proto-dst 80 accept
		meta l4proto tcp ct original proto-dst 8000-8100 accept
		meta l4proto udp ct original proto-dst 8000-8100 accept
		ip saddr 10.0.0.0-10.255.255.255 meta l4proto tcp ct original proto-dst 3306 accept
		ip6 saddr 2001:db8::1 meta l4proto udp ct original proto-dst 53 accept
		drop
	}
}
`)
}

package firewall

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestFirewalldParseRichRule(t *testing.T) {
	fw := newFirewalld()
	tests := []struct {
		name string
		rule string
		want FireInfo
	}{
		{
			name: "源地址单端口 accept",
			rule: `rule family="ipv4" source address="192.168.1.100" port port="8080" protocol="tcp" accept`,
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "192.168.1.100", PortStart: 8080, PortEnd: 8080, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "源地址端口范围 drop",
			rule: `rule family="ipv4" source address="10.0.0.0/8" port port="3000-4000" protocol="tcp" drop`,
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "10.0.0.0/8", PortStart: 3000, PortEnd: 4000, Protocol: ProtocolTCP, Strategy: StrategyDrop, Direction: DirectionIn},
		},
		{
			name: "目标地址算出站",
			rule: `rule family="ipv4" destination address="203.0.113.0/24" port port="443" protocol="tcp" reject`,
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "203.0.113.0/24", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyReject, Direction: DirectionOut},
		},
		{
			name: "ipv6",
			rule: `rule family="ipv6" source address="::1" port port="22" protocol="tcp" accept`,
			want: FireInfo{Type: TypeRich, Family: "ipv6", Address: "::1", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "IP 规则无端口，放开全部端口",
			rule: `rule family="ipv4" source address="8.8.8.8" protocol value="tcp" accept`,
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "8.8.8.8", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "无端口无协议默认 tcp/udp",
			rule: `rule family="ipv4" source address="1.2.3.4" drop`,
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "1.2.3.4", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCPUDP, Strategy: StrategyDrop, Direction: DirectionIn},
		},
		{
			name: "udp 且无地址",
			rule: `rule family="ipv4" port port="53" protocol="udp" accept`,
			want: FireInfo{Type: TypeRich, Family: "ipv4", PortStart: 53, PortEnd: 53, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "mark 策略",
			rule: `rule family="ipv4" source address="10.0.0.1" port port="80" protocol="tcp" mark set="1"`,
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "10.0.0.1", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyMark, Direction: DirectionIn},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fw.parseRichRule(tt.rule)
			must.NoError(t, err)
			check.DeepEqual(t, info, tt.want)
		})
	}
}

func TestFirewalldParseRichRuleInvalid(t *testing.T) {
	fw := newFirewalld()
	tests := []struct {
		name string
		rule string
	}{
		{"非规则文本", `this is not a valid rule`},
		{"空字符串", ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fw.parseRichRule(tt.rule)
			check.Error(t, err)
		})
	}
}

func TestFirewalldForwardRegex(t *testing.T) {
	fw := newFirewalld()
	tests := []struct {
		name string
		line string
		want []string // port、proto、toport、toaddr
	}{
		{"标准", "port=8080:proto=tcp:toport=80:toaddr=192.168.1.100", []string{"8080", "tcp", "80", "192.168.1.100"}},
		{"无目标地址", "port=3000:proto=udp:toport=4000:toaddr=", []string{"3000", "udp", "4000", ""}},
		{"ipv6 目标地址", "port=443:proto=tcp:toport=8443:toaddr=::1", []string{"443", "tcp", "8443", "::1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := fw.forwardListRegex.FindStringSubmatch(tt.line)
			must.Len(t, match, 5, must.Msgf("未匹配: %s", tt.line))
			check.DeepEqual(t, match[1:], tt.want)
		})
	}
}

func TestUFWStripProtocolSuffix(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantAddr  string
		wantProto string
	}{
		{"IP 带 tcp 后缀", "8.8.8.8/tcp", "8.8.8.8", "tcp"},
		{"IP 带 udp 后缀", "1.1.1.1/udp", "1.1.1.1", "udp"},
		{"CIDR /24 不剥离", "192.168.1.0/24", "192.168.1.0/24", ""},
		{"CIDR /8 不剥离", "10.0.0.0/8", "10.0.0.0/8", ""},
		{"无斜杠", "Anywhere", "Anywhere", ""},
		{"纯 IP", "1.2.3.4", "1.2.3.4", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, proto := stripProtocolSuffix(tt.in)
			check.Equal(t, addr, tt.wantAddr)
			check.Equal(t, proto, tt.wantProto)
		})
	}
}

// 入参对应 ufw status numbered 输出的一行四列
func TestUFWParseRule(t *testing.T) {
	fw := newUFW()
	tests := []struct {
		name      string
		target    string
		action    string
		direction string
		source    string
		want      FireInfo
	}{
		{
			name: "tcp 端口", target: "22/tcp", action: "ALLOW", direction: "IN", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "udp 端口", target: "53/udp", action: "ALLOW", direction: "IN", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 53, PortEnd: 53, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			// ufw allow 8888 → "8888" 无协议
			name: "无协议端口默认 tcp/udp", target: "8888", action: "ALLOW", direction: "IN", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 8888, PortEnd: 8888, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "端口范围", target: "80:443/tcp", action: "ALLOW", direction: "IN", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "DENY 转 drop", target: "3306/tcp", action: "DENY", direction: "IN", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 3306, PortEnd: 3306, Protocol: ProtocolTCP, Strategy: StrategyDrop, Direction: DirectionIn},
		},
		{
			name: "REJECT 转 reject", target: "25/tcp", action: "REJECT", direction: "IN", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 25, PortEnd: 25, Protocol: ProtocolTCP, Strategy: StrategyReject, Direction: DirectionIn},
		},
		{
			name: "LIMIT 转 accept", target: "22/tcp", action: "LIMIT", direction: "IN", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "出站方向", target: "443/tcp", action: "ALLOW", direction: "OUT", source: "Anywhere",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionOut},
		},
		{
			// 带地址时协议取具体值而非 tcp/udp，且归为 rich
			name: "端口带地址", target: "80/tcp", action: "ALLOW", direction: "IN", source: "1.1.1.1",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "1.1.1.1", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "端口带 CIDR", target: "80/tcp", action: "ALLOW", direction: "IN", source: "192.168.1.0/24",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "192.168.1.0/24", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "udp 端口带地址", target: "80/udp", action: "ALLOW", direction: "IN", source: "2.2.2.2",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "2.2.2.2", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			// [ 9] Anywhere ALLOW IN 8.8.8.8/tcp
			name: "IP 带协议后缀", target: "Anywhere", action: "ALLOW", direction: "IN", source: "8.8.8.8/tcp",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "8.8.8.8", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "IP 不带协议后缀", target: "Anywhere", action: "ALLOW", direction: "IN", source: "5.5.5.5",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "5.5.5.5", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "IP 规则 DENY", target: "Anywhere", action: "DENY", direction: "IN", source: "10.0.0.1/tcp",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "10.0.0.1", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCP, Strategy: StrategyDrop, Direction: DirectionIn},
		},
		{
			name: "IP 规则带行尾注释", target: "Anywhere", action: "REJECT", direction: "IN", source: "195.178.110.231            # by Fail2Ban after 5 attempts against sshd",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "195.178.110.231", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCPUDP, Strategy: StrategyReject, Direction: DirectionIn},
		},
		{
			name: "端口规则带行尾注释", target: "22/tcp", action: "ALLOW", direction: "IN", source: "Anywhere                   # SSH",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "ipv6 端口", target: "22/tcp (v6)", action: "ALLOW", direction: "IN", source: "Anywhere (v6)",
			want: FireInfo{Type: TypeNormal, Family: "ipv6", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "ipv6 无协议端口", target: "8888 (v6)", action: "ALLOW", direction: "IN", source: "Anywhere (v6)",
			want: FireInfo{Type: TypeNormal, Family: "ipv6", PortStart: 8888, PortEnd: 8888, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "ipv6 端口范围", target: "80:443/tcp (v6)", action: "ALLOW", direction: "IN", source: "Anywhere (v6)",
			want: FireInfo{Type: TypeNormal, Family: "ipv6", PortStart: 80, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			// 列里带具体 IPv6 地址时 ufw 不标 (v6)，需按地址推断
			name: "ipv6 地址无标记", target: "22/tcp", action: "ALLOW", direction: "IN", source: "2001:db8::1",
			want: FireInfo{Type: TypeRich, Family: "ipv6", Address: "2001:db8::1", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			name: "ipv6 CIDR 的 IP 规则", target: "Anywhere (v6)", action: "ALLOW", direction: "IN", source: "2001:db8::/32",
			want: FireInfo{Type: TypeRich, Family: "ipv6", Address: "2001:db8::/32", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			// 出站行的 From 列尾部带 (out)
			name: "出站端口带 (out) 后缀", target: "53/udp", action: "ALLOW", direction: "OUT", source: "Anywhere                   (out)",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 53, PortEnd: 53, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionOut},
		},
		{
			// ufw allow out proto tcp to 1.2.3.4 port 443 → 地址与端口都在 To 列
			name: "出站端口带地址", target: "1.2.3.4 443/tcp", action: "ALLOW", direction: "OUT", source: "Anywhere                   (out)",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "1.2.3.4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionOut},
		},
		{
			name: "出站端口范围带地址", target: "1.2.3.4 100:200/udp", action: "ALLOW", direction: "OUT", source: "Anywhere                   (out)",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "1.2.3.4", PortStart: 100, PortEnd: 200, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionOut},
		},
		{
			name: "出站 IP 规则", target: "5.6.7.8", action: "DENY", direction: "OUT", source: "Anywhere                   (out)",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "5.6.7.8", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCPUDP, Strategy: StrategyDrop, Direction: DirectionOut},
		},
		{
			name: "出站 IP 规则带协议", target: "5.6.7.8/tcp", action: "REJECT", direction: "OUT", source: "Anywhere                   (out)",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "5.6.7.8", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCP, Strategy: StrategyReject, Direction: DirectionOut},
		},
		{
			name: "出站 ipv6 端口带地址", target: "2001:db8::2 443/tcp", action: "ALLOW", direction: "OUT", source: "Anywhere (v6)              (out)",
			want: FireInfo{Type: TypeRich, Family: "ipv6", Address: "2001:db8::2", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionOut},
		},
		{
			name: "出站带 (out) 与注释", target: "1.2.3.4 443/tcp", action: "ALLOW", direction: "OUT", source: "Anywhere                   (out) # api",
			want: FireInfo{Type: TypeRich, Family: "ipv4", Address: "1.2.3.4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionOut},
		},
		{
			name: "带 (log) 后缀", target: "84/tcp", action: "ALLOW", direction: "IN", source: "Anywhere                   (log)",
			want: FireInfo{Type: TypeNormal, Family: "ipv4", PortStart: 84, PortEnd: 84, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
		{
			// ufw allow proto tcp from any to any，无地址无端口，由 service 层过滤
			name: "仅协议", target: "Anywhere/tcp", action: "ALLOW", direction: "IN", source: "Anywhere/tcp",
			want: FireInfo{Type: TypeRich, Family: "ipv4", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, fw.parseRule(tt.target, tt.action, tt.direction, tt.source), &tt.want)
		})
	}
}

// 面板模型表示不了的规则应跳过，而不是解析成端口 0 或把整列塞进地址
func TestUFWParseRuleUnsupported(t *testing.T) {
	fw := newUFW()
	tests := []struct {
		name      string
		target    string
		action    string
		direction string
		source    string
	}{
		{"应用配置名", "Web", "ALLOW", "IN", "Anywhere"},
		{"应用配置名带空格", "Nginx Full", "ALLOW", "IN", "Anywhere"},
		{"多端口列表", "80,443/tcp", "ALLOW", "IN", "Anywhere"},
		{"绑定网卡", "81 on eth0", "ALLOW", "IN", "Anywhere"},
		{"ipv6 绑定网卡", "81 (v6) on eth0", "ALLOW", "IN", "Anywhere (v6)"},
		{"出站来源绑定网卡", "9.9.9.9", "DENY", "OUT", "Anywhere on eth0           (out)"},
		{"带来源端口", "9000/tcp", "ALLOW", "IN", "1.1.1.1 1024/tcp"},
		{"入站两侧都带地址", "10.0.0.5 82", "ALLOW", "IN", "1.1.1.1"},
		{"入站仅目的地址", "10.0.0.5 83", "ALLOW", "IN", "Anywhere"},
		{"出站仅来源地址", "443/tcp", "ALLOW", "OUT", "10.0.0.5                   (out)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.Nil(t, fw.parseRule(tt.target, tt.action, tt.direction, tt.source))
		})
	}
}

func TestUFWMergeRules(t *testing.T) {
	tests := []struct {
		name  string
		rules []FireInfo
		want  []FireInfo
	}{
		{
			name: "同端口 tcp+udp 合并",
			rules: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "端口不同不合并",
			rules: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "地址不同不合并",
			rules: []FireInfo{
				{Type: TypeRich, Family: "ipv4", Address: "1.1.1.1", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeRich, Family: "ipv4", Address: "2.2.2.2", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeRich, Family: "ipv4", Address: "1.1.1.1", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeRich, Family: "ipv4", Address: "2.2.2.2", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "地址相同合并",
			rules: []FireInfo{
				{Type: TypeRich, Family: "ipv4", Address: "2.2.2.2", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeRich, Family: "ipv4", Address: "2.2.2.2", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeRich, Family: "ipv4", Address: "2.2.2.2", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "ipv4 与 ipv6 不合并",
			rules: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv6", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv6", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "策略不同不合并",
			rules: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyDrop, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyDrop, Direction: DirectionIn},
			},
		},
		{
			name: "单条规则原样返回",
			rules: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "已是 tcp/udp 不变",
			rules: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 8888, PortEnd: 8888, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 8888, PortEnd: 8888, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "IP 规则 tcp+udp 合并",
			rules: []FireInfo{
				{Type: TypeRich, Family: "ipv4", Address: "6.6.6.6", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeRich, Family: "ipv4", Address: "6.6.6.6", PortStart: 1, PortEnd: 65535, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeRich, Family: "ipv4", Address: "6.6.6.6", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
		{
			name: "保持首次出现顺序",
			rules: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
			want: []FireInfo{
				{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
				{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, mergeRules(tt.rules), tt.want)
		})
	}
}

func TestUFWNATRegex(t *testing.T) {
	fw := newUFW()
	tests := []struct {
		name string
		line string
		want []string // proto、dport、目标地址、目标端口；nil 表示不应匹配
	}{
		{"tcp", "-A PREROUTING -p tcp --dport 8080 -j DNAT --to-destination 192.168.1.100:80", []string{"tcp", "8080", "192.168.1.100", "80"}},
		{"udp", "-A PREROUTING -p udp --dport 53 -j DNAT --to-destination 10.0.0.1:5353", []string{"udp", "53", "10.0.0.1", "5353"}},
		{"POSTROUTING 不匹配", "-A POSTROUTING -d 192.168.1.100 -p tcp --dport 80 -j MASQUERADE", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := fw.natRegex.FindStringSubmatch(tt.line)
			if tt.want == nil {
				check.Empty(t, match, check.Msgf("不应匹配: %s", tt.line))
				return
			}
			must.Len(t, match, 5, must.Msgf("未匹配: %s", tt.line))
			check.DeepEqual(t, match[1:], tt.want)
		})
	}
}

func TestUFWParseRuleFullOutput(t *testing.T) {
	fw := newUFW()
	lines := []struct {
		target    string
		action    string
		direction string
		source    string
	}{
		{"22/tcp", "ALLOW", "IN", "Anywhere"},
		{"80/tcp", "ALLOW", "IN", "Anywhere"},
		{"443/tcp", "ALLOW", "IN", "Anywhere"},
		{"443/udp", "ALLOW", "IN", "Anywhere"},
		{"8888", "ALLOW", "IN", "Anywhere"},
		{"80/tcp", "ALLOW", "IN", "1.1.1.1"},
		{"80/tcp", "ALLOW", "IN", "2.2.2.2"},
		{"80/udp", "ALLOW", "IN", "2.2.2.2"},
		{"Anywhere", "ALLOW", "IN", "8.8.8.8/tcp"},
		{"Anywhere", "ALLOW", "IN", "6.6.6.6/tcp"},
		{"Anywhere", "ALLOW", "IN", "6.6.6.6/udp"},
		{"22/tcp (v6)", "ALLOW", "IN", "Anywhere (v6)"},
		{"80/tcp (v6)", "ALLOW", "IN", "Anywhere (v6)"},
		{"443/tcp (v6)", "ALLOW", "IN", "Anywhere (v6)"},
		{"443/udp (v6)", "ALLOW", "IN", "Anywhere (v6)"},
		{"8888 (v6)", "ALLOW", "IN", "Anywhere (v6)"},
	}

	var rules []FireInfo
	for _, l := range lines {
		info := fw.parseRule(l.target, l.action, l.direction, l.source)
		must.NotNil(t, info)
		rules = append(rules, *info)
	}

	// 同端口的 tcp 与 udp 合并为 tcp/udp，其余按首次出现顺序保留
	check.DeepEqual(t, mergeRules(rules), []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeNormal, Family: "ipv4", PortStart: 8888, PortEnd: 8888, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeRich, Family: "ipv4", Address: "1.1.1.1", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeRich, Family: "ipv4", Address: "2.2.2.2", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeRich, Family: "ipv4", Address: "8.8.8.8", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeRich, Family: "ipv4", Address: "6.6.6.6", PortStart: 1, PortEnd: 65535, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeNormal, Family: "ipv6", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeNormal, Family: "ipv6", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeNormal, Family: "ipv6", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
		{Type: TypeNormal, Family: "ipv6", PortStart: 8888, PortEnd: 8888, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: DirectionIn},
	})
}

func TestIsLocalAddress(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"192.168.1.1", false},
		{"8.8.8.8", false},
		{"localhost", false}, // net.ParseIP 返回 nil
		{"not-an-ip", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			check.Equal(t, isLocalAddress(tt.ip), tt.want)
		})
	}
}

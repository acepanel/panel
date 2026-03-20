package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type FirewalldSuite struct {
	suite.Suite
	fw *firewalld
}

func (s *FirewalldSuite) SetupTest() {
	s.fw = newFirewalld()
}

// --- parseRichRule ---

func (s *FirewalldSuite) TestParseRichRule_AcceptSourcePort() {
	info, err := s.fw.parseRichRule(`rule family="ipv4" source address="192.168.1.100" port port="8080" protocol="tcp" accept`)
	s.NoError(err)
	s.Equal("ipv4", info.Family)
	s.Equal("192.168.1.100", info.Address)
	s.Equal(uint(8080), info.PortStart)
	s.Equal(uint(8080), info.PortEnd)
	s.Equal(ProtocolTCP, info.Protocol)
	s.Equal(StrategyAccept, info.Strategy)
	s.Equal(Direction("in"), info.Direction)
	s.Equal(TypeRich, info.Type)
}

func (s *FirewalldSuite) TestParseRichRule_DropSourcePortRange() {
	info, err := s.fw.parseRichRule(`rule family="ipv4" source address="10.0.0.0/8" port port="3000-4000" protocol="tcp" drop`)
	s.NoError(err)
	s.Equal("10.0.0.0/8", info.Address)
	s.Equal(uint(3000), info.PortStart)
	s.Equal(uint(4000), info.PortEnd)
	s.Equal(ProtocolTCP, info.Protocol)
	s.Equal(StrategyDrop, info.Strategy)
	s.Equal(Direction("in"), info.Direction)
}

func (s *FirewalldSuite) TestParseRichRule_RejectDestination() {
	info, err := s.fw.parseRichRule(`rule family="ipv4" destination address="203.0.113.0/24" port port="443" protocol="tcp" reject`)
	s.NoError(err)
	s.Equal("203.0.113.0/24", info.Address)
	s.Equal(Direction("out"), info.Direction)
	s.Equal(StrategyReject, info.Strategy)
}

func (s *FirewalldSuite) TestParseRichRule_IPv6() {
	info, err := s.fw.parseRichRule(`rule family="ipv6" source address="::1" port port="22" protocol="tcp" accept`)
	s.NoError(err)
	s.Equal("ipv6", info.Family)
	s.Equal("::1", info.Address)
	s.Equal(uint(22), info.PortStart)
}

func (s *FirewalldSuite) TestParseRichRule_NoPort_IPOnly() {
	// IP 规则无端口，解析为 1-65535
	info, err := s.fw.parseRichRule(`rule family="ipv4" source address="8.8.8.8" protocol value="tcp" accept`)
	s.NoError(err)
	s.Equal("8.8.8.8", info.Address)
	s.Equal(uint(1), info.PortStart)
	s.Equal(uint(65535), info.PortEnd)
	s.Equal(ProtocolTCP, info.Protocol)
}

func (s *FirewalldSuite) TestParseRichRule_NoPort_NoProtocol() {
	// 无端口无协议规则
	info, err := s.fw.parseRichRule(`rule family="ipv4" source address="1.2.3.4" drop`)
	s.NoError(err)
	s.Equal("1.2.3.4", info.Address)
	s.Equal(ProtocolTCPUDP, info.Protocol) // 默认 tcp/udp
	s.Equal(uint(1), info.PortStart)
	s.Equal(uint(65535), info.PortEnd)
}

func (s *FirewalldSuite) TestParseRichRule_UDPProtocol() {
	info, err := s.fw.parseRichRule(`rule family="ipv4" port port="53" protocol="udp" accept`)
	s.NoError(err)
	s.Equal(ProtocolUDP, info.Protocol)
	s.Equal(uint(53), info.PortStart)
	s.Equal(uint(53), info.PortEnd)
	s.Empty(info.Address)
}

func (s *FirewalldSuite) TestParseRichRule_MarkStrategy() {
	info, err := s.fw.parseRichRule(`rule family="ipv4" source address="10.0.0.1" port port="80" protocol="tcp" mark set="1"`)
	s.NoError(err)
	s.Equal(Strategy("mark"), info.Strategy)
}

func (s *FirewalldSuite) TestParseRichRule_InvalidFormat() {
	_, err := s.fw.parseRichRule(`this is not a valid rule`)
	s.Error(err)
}

func (s *FirewalldSuite) TestParseRichRule_EmptyString() {
	_, err := s.fw.parseRichRule(``)
	s.Error(err)
}

// --- forward 正则 ---

func (s *FirewalldSuite) TestForwardRegex_Standard() {
	line := "port=8080:proto=tcp:toport=80:toaddr=192.168.1.100"
	s.True(s.fw.forwardListRegex.MatchString(line))
	match := s.fw.forwardListRegex.FindStringSubmatch(line)
	s.Equal("8080", match[1])
	s.Equal("tcp", match[2])
	s.Equal("80", match[3])
	s.Equal("192.168.1.100", match[4])
}

func (s *FirewalldSuite) TestForwardRegex_NoAddr() {
	line := "port=3000:proto=udp:toport=4000:toaddr="
	s.True(s.fw.forwardListRegex.MatchString(line))
	match := s.fw.forwardListRegex.FindStringSubmatch(line)
	s.Equal("3000", match[1])
	s.Equal("udp", match[2])
	s.Equal("4000", match[3])
	s.Equal("", match[4])
}

func (s *FirewalldSuite) TestForwardRegex_IPv6Addr() {
	line := "port=443:proto=tcp:toport=8443:toaddr=::1"
	s.True(s.fw.forwardListRegex.MatchString(line))
	match := s.fw.forwardListRegex.FindStringSubmatch(line)
	s.Equal("::1", match[4])
}

func TestFirewalldSuite(t *testing.T) {
	suite.Run(t, new(FirewalldSuite))
}

// ===================== UFW 解析器测试 =====================

type UFWSuite struct {
	suite.Suite
	fw *ufw
}

func (s *UFWSuite) SetupTest() {
	s.fw = newUFW()
}

// --- stripProtocolSuffix ---

func (s *UFWSuite) TestStrip_IPWithTCP() {
	addr, proto := stripProtocolSuffix("8.8.8.8/tcp")
	s.Equal("8.8.8.8", addr)
	s.Equal("tcp", proto)
}

func (s *UFWSuite) TestStrip_IPWithUDP() {
	addr, proto := stripProtocolSuffix("1.1.1.1/udp")
	s.Equal("1.1.1.1", addr)
	s.Equal("udp", proto)
}

func (s *UFWSuite) TestStrip_CIDR_NotStripped() {
	addr, proto := stripProtocolSuffix("192.168.1.0/24")
	s.Equal("192.168.1.0/24", addr)
	s.Equal("", proto)
}

func (s *UFWSuite) TestStrip_CIDR8_NotStripped() {
	addr, proto := stripProtocolSuffix("10.0.0.0/8")
	s.Equal("10.0.0.0/8", addr)
	s.Equal("", proto)
}

func (s *UFWSuite) TestStrip_NoSlash() {
	addr, proto := stripProtocolSuffix("Anywhere")
	s.Equal("Anywhere", addr)
	s.Equal("", proto)
}

func (s *UFWSuite) TestStrip_PlainIP() {
	addr, proto := stripProtocolSuffix("1.2.3.4")
	s.Equal("1.2.3.4", addr)
	s.Equal("", proto)
}

// --- parseRule 端口规则 ---

func (s *UFWSuite) TestParseRule_SimpleTCP() {
	info := s.fw.parseRule("22/tcp", "ALLOW", "IN", "Anywhere")
	s.Equal(uint(22), info.PortStart)
	s.Equal(uint(22), info.PortEnd)
	s.Equal(ProtocolTCP, info.Protocol)
	s.Equal(StrategyAccept, info.Strategy)
	s.Equal(Direction("in"), info.Direction)
	s.Equal("ipv4", info.Family)
	s.Equal(TypeNormal, info.Type)
	s.Empty(info.Address)
}

func (s *UFWSuite) TestParseRule_SimpleUDP() {
	info := s.fw.parseRule("53/udp", "ALLOW", "IN", "Anywhere")
	s.Equal(ProtocolUDP, info.Protocol)
	s.Equal(uint(53), info.PortStart)
}

func (s *UFWSuite) TestParseRule_NoProtocol() {
	// ufw allow 8888 → "8888" 无协议
	info := s.fw.parseRule("8888", "ALLOW", "IN", "Anywhere")
	s.Equal(uint(8888), info.PortStart)
	s.Equal(uint(8888), info.PortEnd)
	s.Equal(ProtocolTCPUDP, info.Protocol)
	s.Equal(TypeNormal, info.Type)
}

func (s *UFWSuite) TestParseRule_PortRange() {
	info := s.fw.parseRule("80:443/tcp", "ALLOW", "IN", "Anywhere")
	s.Equal(uint(80), info.PortStart)
	s.Equal(uint(443), info.PortEnd)
	s.Equal(ProtocolTCP, info.Protocol)
}

func (s *UFWSuite) TestParseRule_DenyStrategy() {
	info := s.fw.parseRule("3306/tcp", "DENY", "IN", "Anywhere")
	s.Equal(StrategyDrop, info.Strategy)
}

func (s *UFWSuite) TestParseRule_RejectStrategy() {
	info := s.fw.parseRule("25/tcp", "REJECT", "IN", "Anywhere")
	s.Equal(StrategyReject, info.Strategy)
}

func (s *UFWSuite) TestParseRule_LimitStrategy() {
	info := s.fw.parseRule("22/tcp", "LIMIT", "IN", "Anywhere")
	s.Equal(StrategyAccept, info.Strategy)
}

func (s *UFWSuite) TestParseRule_OutDirection() {
	info := s.fw.parseRule("443/tcp", "ALLOW", "OUT", "Anywhere")
	s.Equal(Direction("out"), info.Direction)
}

// --- parseRule 带地址的端口规则 ---

func (s *UFWSuite) TestParseRule_PortWithAddress() {
	// [ 6] 80/tcp ALLOW IN 1.1.1.1
	info := s.fw.parseRule("80/tcp", "ALLOW", "IN", "1.1.1.1")
	s.Equal(uint(80), info.PortStart)
	s.Equal(uint(80), info.PortEnd)
	s.Equal(ProtocolTCP, info.Protocol) // 必须是 tcp，不是 tcp/udp
	s.Equal("1.1.1.1", info.Address)
	s.Equal(TypeRich, info.Type) // 有地址 → rich
}

func (s *UFWSuite) TestParseRule_PortWithCIDR() {
	// 80/tcp ALLOW IN 192.168.1.0/24
	info := s.fw.parseRule("80/tcp", "ALLOW", "IN", "192.168.1.0/24")
	s.Equal(ProtocolTCP, info.Protocol)
	s.Equal("192.168.1.0/24", info.Address) // CIDR 保持不变
	s.Equal(uint(80), info.PortStart)
}

func (s *UFWSuite) TestParseRule_PortUDPWithAddress() {
	// 80/udp ALLOW IN 2.2.2.2
	info := s.fw.parseRule("80/udp", "ALLOW", "IN", "2.2.2.2")
	s.Equal(ProtocolUDP, info.Protocol)
	s.Equal("2.2.2.2", info.Address)
}

// --- parseRule 纯 IP 规则 ---

func (s *UFWSuite) TestParseRule_IPWithProtoSuffix() {
	// [ 9] Anywhere ALLOW IN 8.8.8.8/tcp
	info := s.fw.parseRule("Anywhere", "ALLOW", "IN", "8.8.8.8/tcp")
	s.Equal("8.8.8.8", info.Address)
	s.Equal(Protocol("tcp"), info.Protocol)
	s.Equal(uint(1), info.PortStart)
	s.Equal(uint(65535), info.PortEnd)
	s.Equal(TypeRich, info.Type)
}

func (s *UFWSuite) TestParseRule_IPWithoutProto() {
	// Anywhere ALLOW IN 5.5.5.5
	info := s.fw.parseRule("Anywhere", "ALLOW", "IN", "5.5.5.5")
	s.Equal("5.5.5.5", info.Address)
	s.Equal(ProtocolTCPUDP, info.Protocol)
	s.Equal(uint(1), info.PortStart)
	s.Equal(uint(65535), info.PortEnd)
}

func (s *UFWSuite) TestParseRule_IPDeny() {
	info := s.fw.parseRule("Anywhere", "DENY", "IN", "10.0.0.1/tcp")
	s.Equal("10.0.0.1", info.Address)
	s.Equal(StrategyDrop, info.Strategy)
	s.Equal(Protocol("tcp"), info.Protocol)
}

// --- parseRule IPv6 ---

func (s *UFWSuite) TestParseRule_IPv6Port() {
	// 22/tcp (v6) ALLOW IN Anywhere (v6)
	info := s.fw.parseRule("22/tcp (v6)", "ALLOW", "IN", "Anywhere (v6)")
	s.Equal("ipv6", info.Family)
	s.Equal(uint(22), info.PortStart)
	s.Equal(ProtocolTCP, info.Protocol)
	s.Empty(info.Address)
}

func (s *UFWSuite) TestParseRule_IPv6NoProto() {
	// 8888 (v6) ALLOW IN Anywhere (v6)
	info := s.fw.parseRule("8888 (v6)", "ALLOW", "IN", "Anywhere (v6)")
	s.Equal("ipv6", info.Family)
	s.Equal(uint(8888), info.PortStart)
	s.Equal(ProtocolTCPUDP, info.Protocol)
}

func (s *UFWSuite) TestParseRule_IPv6PortRange() {
	info := s.fw.parseRule("80:443/tcp (v6)", "ALLOW", "IN", "Anywhere (v6)")
	s.Equal("ipv6", info.Family)
	s.Equal(uint(80), info.PortStart)
	s.Equal(uint(443), info.PortEnd)
	s.Equal(ProtocolTCP, info.Protocol)
}

// --- mergeRules ---

func (s *UFWSuite) TestMergeRules_TCPAndUDP() {
	rules := []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
		{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: "in"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 1)
	s.Equal(ProtocolTCPUDP, merged[0].Protocol)
}

func (s *UFWSuite) TestMergeRules_DifferentPorts_NoMerge() {
	rules := []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
		{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 2)
}

func (s *UFWSuite) TestMergeRules_DifferentAddresses_NoMerge() {
	rules := []FireInfo{
		{Type: TypeRich, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in", Address: "1.1.1.1"},
		{Type: TypeRich, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: "in", Address: "2.2.2.2"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 2)
}

func (s *UFWSuite) TestMergeRules_SameAddress_Merge() {
	rules := []FireInfo{
		{Type: TypeRich, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in", Address: "2.2.2.2"},
		{Type: TypeRich, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: "in", Address: "2.2.2.2"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 1)
	s.Equal(ProtocolTCPUDP, merged[0].Protocol)
	s.Equal("2.2.2.2", merged[0].Address)
}

func (s *UFWSuite) TestMergeRules_IPv4AndIPv6_NoMerge() {
	rules := []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
		{Type: TypeNormal, Family: "ipv6", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 2)
}

func (s *UFWSuite) TestMergeRules_DifferentStrategy_NoMerge() {
	rules := []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
		{Type: TypeNormal, Family: "ipv4", PortStart: 80, PortEnd: 80, Protocol: ProtocolUDP, Strategy: StrategyDrop, Direction: "in"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 2)
}

func (s *UFWSuite) TestMergeRules_SingleRule_NoChange() {
	rules := []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 1)
	s.Equal(ProtocolTCP, merged[0].Protocol)
}

func (s *UFWSuite) TestMergeRules_AlreadyTCPUDP_NoChange() {
	rules := []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 8888, PortEnd: 8888, Protocol: ProtocolTCPUDP, Strategy: StrategyAccept, Direction: "in"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 1)
	s.Equal(ProtocolTCPUDP, merged[0].Protocol)
}

func (s *UFWSuite) TestMergeRules_IPRules_TCPAndUDP() {
	rules := []FireInfo{
		{Type: TypeRich, Family: "ipv4", PortStart: 1, PortEnd: 65535, Protocol: Protocol("tcp"), Strategy: StrategyAccept, Direction: "in", Address: "6.6.6.6"},
		{Type: TypeRich, Family: "ipv4", PortStart: 1, PortEnd: 65535, Protocol: Protocol("udp"), Strategy: StrategyAccept, Direction: "in", Address: "6.6.6.6"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 1)
	s.Equal(ProtocolTCPUDP, merged[0].Protocol)
	s.Equal("6.6.6.6", merged[0].Address)
}

func (s *UFWSuite) TestMergeRules_PreservesOrder() {
	rules := []FireInfo{
		{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
		{Type: TypeNormal, Family: "ipv4", PortStart: 22, PortEnd: 22, Protocol: ProtocolTCP, Strategy: StrategyAccept, Direction: "in"},
		{Type: TypeNormal, Family: "ipv4", PortStart: 443, PortEnd: 443, Protocol: ProtocolUDP, Strategy: StrategyAccept, Direction: "in"},
	}
	merged := mergeRules(rules)
	s.Len(merged, 2)
	s.Equal(uint(443), merged[0].PortStart)
	s.Equal(ProtocolTCPUDP, merged[0].Protocol)
	s.Equal(uint(22), merged[1].PortStart)
	s.Equal(ProtocolTCP, merged[1].Protocol)
}

// --- NAT 正则 ---

func (s *UFWSuite) TestNATRegex_Standard() {
	line := "-A PREROUTING -p tcp --dport 8080 -j DNAT --to-destination 192.168.1.100:80"
	s.True(s.fw.natRegex.MatchString(line))
	match := s.fw.natRegex.FindStringSubmatch(line)
	s.Equal("tcp", match[1])
	s.Equal("8080", match[2])
	s.Equal("192.168.1.100", match[3])
	s.Equal("80", match[4])
}

func (s *UFWSuite) TestNATRegex_UDP() {
	line := "-A PREROUTING -p udp --dport 53 -j DNAT --to-destination 10.0.0.1:5353"
	s.True(s.fw.natRegex.MatchString(line))
	match := s.fw.natRegex.FindStringSubmatch(line)
	s.Equal("udp", match[1])
	s.Equal("53", match[2])
	s.Equal("10.0.0.1", match[3])
	s.Equal("5353", match[4])
}

func (s *UFWSuite) TestNATRegex_NoMatch() {
	line := "-A POSTROUTING -d 192.168.1.100 -p tcp --dport 80 -j MASQUERADE"
	s.False(s.fw.natRegex.MatchString(line))
}

func (s *UFWSuite) TestRouteAllowCommand() {
	cmd := s.fw.routeAllowCommand(Forward{
		Protocol:   ProtocolTCP,
		Port:       8080,
		TargetIP:   "192.168.1.100",
		TargetPort: 80,
	}, "tcp")
	s.Equal("ufw route allow proto tcp to 192.168.1.100 port 80", cmd)
}

func (s *UFWSuite) TestRouteDeleteCommand() {
	cmd := s.fw.routeDeleteCommand(Forward{
		Protocol:   ProtocolTCP,
		Port:       8080,
		TargetIP:   "192.168.1.100",
		TargetPort: 80,
	}, "tcp")
	s.Equal("ufw --force delete route allow proto tcp to 192.168.1.100 port 80", cmd)
}

// --- 完整 UFW 输出集成测试 ---

func (s *UFWSuite) TestParseRule_FullOutput() {
	// 模拟用户提供的完整输出
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
		info := s.fw.parseRule(l.target, l.action, l.direction, l.source)
		s.NotNil(info)
		rules = append(rules, *info)
	}

	merged := mergeRules(rules)

	// 构建查找辅助
	find := func(port uint, family string, addr string) *FireInfo {
		for i := range merged {
			r := &merged[i]
			if r.PortStart == port && r.Family == family && r.Address == addr {
				return r
			}
		}
		return nil
	}
	findIP := func(addr string, family string) *FireInfo {
		for i := range merged {
			r := &merged[i]
			if r.Address == addr && r.Family == family && r.PortStart == 1 && r.PortEnd == 65535 {
				return r
			}
		}
		return nil
	}

	// 22/tcp ipv4
	r := find(22, "ipv4", "")
	s.NotNil(r)
	s.Equal(ProtocolTCP, r.Protocol)

	// 443 ipv4 → tcp + udp 合并
	r = find(443, "ipv4", "")
	s.NotNil(r)
	s.Equal(ProtocolTCPUDP, r.Protocol)

	// 8888 ipv4 → 原生 tcp/udp
	r = find(8888, "ipv4", "")
	s.NotNil(r)
	s.Equal(ProtocolTCPUDP, r.Protocol)

	// 80/tcp from 1.1.1.1 → 仅 tcp
	r = find(80, "ipv4", "1.1.1.1")
	s.NotNil(r)
	s.Equal(ProtocolTCP, r.Protocol)

	// 80/tcp+udp from 2.2.2.2 → 合并
	r = find(80, "ipv4", "2.2.2.2")
	s.NotNil(r)
	s.Equal(ProtocolTCPUDP, r.Protocol)

	// IP rule 8.8.8.8/tcp → tcp
	r = findIP("8.8.8.8", "ipv4")
	s.NotNil(r)
	s.Equal(Protocol("tcp"), r.Protocol)

	// IP rule 6.6.6.6 tcp+udp → 合并
	r = findIP("6.6.6.6", "ipv4")
	s.NotNil(r)
	s.Equal(ProtocolTCPUDP, r.Protocol)

	// 443 ipv6 → tcp + udp 合并
	r = find(443, "ipv6", "")
	s.NotNil(r)
	s.Equal(ProtocolTCPUDP, r.Protocol)

	// 8888 ipv6 → 原生 tcp/udp
	r = find(8888, "ipv6", "")
	s.NotNil(r)
	s.Equal(ProtocolTCPUDP, r.Protocol)

	// 22/tcp ipv6
	r = find(22, "ipv6", "")
	s.NotNil(r)
	s.Equal(ProtocolTCP, r.Protocol)
}

func TestUFWSuite(t *testing.T) {
	suite.Run(t, new(UFWSuite))
}

// ===================== isLocalAddress 测试 =====================

func TestIsLocalAddress(t *testing.T) {
	assert.True(t, isLocalAddress("127.0.0.1"))
	assert.True(t, isLocalAddress("::1"))
	assert.True(t, isLocalAddress("0.0.0.0"))
	assert.False(t, isLocalAddress("192.168.1.1"))
	assert.False(t, isLocalAddress("8.8.8.8"))
	assert.False(t, isLocalAddress("localhost")) // net.ParseIP returns nil
	assert.False(t, isLocalAddress("not-an-ip"))
}

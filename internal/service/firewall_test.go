package service

import (
	"testing"

	"github.com/libtnb/assert/check"

	"github.com/acepanel/panel/v3/pkg/firewall"
)

// 端口落在规则范围内、协议有交集的转发才算这条规则的
func TestFirewallForwardsInRange(t *testing.T) {
	s := &FirewallService{}
	forwards := []firewall.FireForwardInfo{
		{Port: 3389, Protocol: firewall.ProtocolTCP, TargetIP: "10.0.0.2", TargetPort: 3389},
		{Port: 3389, Protocol: firewall.ProtocolUDP, TargetPort: 13389},
		{Port: 8080, Protocol: firewall.ProtocolTCP, TargetIP: "10.0.0.3", TargetPort: 80},
	}

	tcp := s.forwardsInRange(firewall.FireInfo{PortStart: 3389, PortEnd: 3389, Protocol: firewall.ProtocolTCP}, forwards)
	check.DeepEqual(t, tcp, forwards[:1])

	both := s.forwardsInRange(firewall.FireInfo{PortStart: 3000, PortEnd: 9000, Protocol: firewall.ProtocolTCPUDP}, forwards)
	check.DeepEqual(t, both, forwards)

	none := s.forwardsInRange(firewall.FireInfo{PortStart: 80, PortEnd: 443, Protocol: firewall.ProtocolTCP}, forwards)
	check.Len(t, none, 0)
	check.NotNil(t, none)
}

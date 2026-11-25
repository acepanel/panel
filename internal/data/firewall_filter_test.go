package data

import (
	"testing"

	"github.com/libtnb/assert/check"

	"github.com/acepanel/panel/v3/pkg/firewall"
)

func TestFirewallFilterAllowedRules(t *testing.T) {
	rule := func(address string, start, end uint, protocol firewall.Protocol) firewall.FireInfo {
		return firewall.FireInfo{Address: address, PortStart: start, PortEnd: end, Protocol: protocol, Strategy: firewall.StrategyAccept, Direction: firewall.DirectionIn}
	}
	drop := rule("192.0.2.9", 1, 65535, firewall.ProtocolTCPUDP)
	drop.Strategy = firewall.StrategyDrop
	outbound := rule("192.0.2.9", 1, 65535, firewall.ProtocolTCPUDP)
	outbound.Direction = firewall.DirectionOut
	scoped := []firewall.FireInfo{
		rule("192.0.2.2", 443, 443, firewall.ProtocolTCP),
		rule("192.0.2.3", 1, 65535, firewall.ProtocolUDP),
	}

	allPorts, got := new(firewallFilterRepo).allowedRules(append([]firewall.FireInfo{
		rule("192.0.2.1", 1, 65535, firewall.ProtocolTCPUDP),
		// 不限来源的放行不算豁免
		rule("", 443, 443, firewall.ProtocolTCP),
		rule("0.0.0.0/0", 443, 443, firewall.ProtocolTCP),
		rule("::/0", 1, 65535, firewall.ProtocolTCPUDP),
		drop,
		outbound,
	}, scoped...))
	check.DeepEqual(t, allPorts, []string{"192.0.2.1"})
	check.DeepEqual(t, got, scoped)
}

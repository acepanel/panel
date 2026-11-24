package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cast"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/pkg/firewall"
)

func (s *CliService) FirewallStatus(ctx context.Context, cmd *cli.Command) error {
	fw := firewall.NewFirewall()
	running, err := fw.Status(ctx)
	if err != nil {
		return err
	}

	statusStr := s.t.Get("stopped")
	if running {
		statusStr = s.t.Get("running")
	}
	fmt.Println(s.t.Get("Firewall status: %s", statusStr))

	if ping, pingErr := fw.PingStatus(ctx); pingErr == nil {
		fmt.Println(s.t.Get("Ping allowed: %t", ping))
	}

	return nil
}

func (s *CliService) FirewallOn(ctx context.Context, cmd *cli.Command) error {
	if err := firewall.NewFirewall().Enable(ctx); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Firewall enabled"))
	return s.syncFirewallFilter(ctx)
}

func (s *CliService) FirewallOff(ctx context.Context, cmd *cli.Command) error {
	if err := firewall.NewFirewall().Disable(ctx); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Firewall disabled"))
	return s.syncFirewallFilter(ctx)
}

func (s *CliService) FirewallList(ctx context.Context, cmd *cli.Command) error {
	rules, err := firewall.NewFirewall().ListRule(ctx)
	if err != nil {
		return err
	}

	return s.printList(cmd, rules, func() {
		for _, rule := range rules {
			fmt.Println(s.t.Get("Port: %d-%d, Protocol: %s, Strategy: %s, Direction: %s, Address: %s", rule.PortStart, rule.PortEnd, string(rule.Protocol), string(rule.Strategy), string(rule.Direction), rule.Address))
		}
	})
}

// FirewallPort 放行或移除端口，端口支持 8888 与 8000-9000 两种写法
func (s *CliService) FirewallPort(ctx context.Context, cmd *cli.Command) error {
	value := cmd.StringArg("port")
	start, end, found := strings.Cut(value, "-")
	if !found {
		end = start
	}

	portStart, portEnd := cast.ToUint(start), cast.ToUint(end)
	startValid := portStart >= 1 && portStart <= 65535
	endValid := portEnd >= 1 && portEnd <= 65535
	if !startValid || !endValid || portStart > portEnd {
		return errors.New(s.t.Get("Port range error"))
	}

	protocol := firewall.Protocol(cmd.String("protocol"))
	if !slices.Contains([]firewall.Protocol{firewall.ProtocolTCP, firewall.ProtocolUDP, firewall.ProtocolTCPUDP}, protocol) {
		return errors.New(s.t.Get("Unsupported protocol: %s", cmd.String("protocol")))
	}

	fw := firewall.NewFirewall()
	if running, _ := fw.Status(ctx); !running {
		return errors.New(s.t.Get("Firewall is not running"))
	}

	operation := firewall.OperationAdd
	if cmd.Bool("remove") {
		operation = firewall.OperationRemove
	}

	if err := fw.Port(ctx, firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: portStart,
		PortEnd:   portEnd,
		Protocol:  protocol,
		Strategy:  firewall.StrategyAccept,
		Direction: firewall.DirectionIn,
	}, operation); err != nil {
		return err
	}

	msg := s.t.Get("Port %s allowed successfully", value)
	if cmd.Bool("remove") {
		msg = s.t.Get("Port %s removed successfully", value)
	}

	fmt.Println(msg)
	return nil
}

// syncFirewallFilter 过滤表随防火墙启停，面板没在运行时也要立刻跟上
// 关掉防火墙是被订阅或容器端口管控误拦之后的自救手段，表不能留着
func (s *CliService) syncFirewallFilter(ctx context.Context) error {
	if err := s.firewallFilterRepo.Sync(ctx); err != nil {
		return errors.New(s.t.Get("failed to apply firewall filter rules: %v", err))
	}

	return nil
}

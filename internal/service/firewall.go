package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"
	"slices"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/firewall"
	"github.com/tnborg/panel/pkg/os"
	"github.com/tnborg/panel/pkg/systemctl"
)

type FirewallService struct {
	firewall *firewall.Firewall
}

func NewFirewallService() *FirewallService {
	return &FirewallService{
		firewall: firewall.NewFirewall(),
	}
}

func (s *FirewallService) GetStatus(c fiber.Ctx) error {
	running, err := s.firewall.Status()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, running)
}

func (s *FirewallService) UpdateStatus(c fiber.Ctx) error {
	req, err := Bind[request.FirewallStatus](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if req.Status {
		err = systemctl.Start("firewalld")
		if err == nil {
			err = systemctl.Enable("firewalld")
		}
	} else {
		err = systemctl.Stop("firewalld")
		if err == nil {
			err = systemctl.Disable("firewalld")
		}
	}

	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FirewallService) GetRules(c fiber.Ctx) error {
	rules, err := s.firewall.ListRule()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	var filledRules []map[string]any
	for rule := range slices.Values(rules) {
		// 去除IP规则
		if rule.PortStart == 1 && rule.PortEnd == 65535 {
			continue
		}
		isUse := false
		for port := rule.PortStart; port <= rule.PortEnd; port++ {
			switch rule.Protocol {
			case firewall.ProtocolTCP:
				isUse = os.TCPPortInUse(port)
			case firewall.ProtocolUDP:
				isUse = os.UDPPortInUse(port)
			default:
				isUse = os.TCPPortInUse(port) || os.UDPPortInUse(port)
			}
			if isUse {
				break
			}
		}
		filledRules = append(filledRules, map[string]any{
			"type":       rule.Type,
			"family":     rule.Family,
			"port_start": rule.PortStart,
			"port_end":   rule.PortEnd,
			"protocol":   rule.Protocol,
			"address":    rule.Address,
			"strategy":   rule.Strategy,
			"direction":  rule.Direction,
			"in_use":     isUse,
		})
	}

	paged, total := Paginate(r, filledRules)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *FirewallService) CreateRule(c fiber.Ctx) error {
	req, err := Bind[request.FirewallRule](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.firewall.Port(firewall.FireInfo{
		Type: firewall.Type(req.Type), Family: req.Family, PortStart: req.PortStart, PortEnd: req.PortEnd, Protocol: firewall.Protocol(req.Protocol), Address: req.Address, Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationAdd); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FirewallService) DeleteRule(c fiber.Ctx) error {
	req, err := Bind[request.FirewallRule](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.firewall.Port(firewall.FireInfo{
		Type: firewall.Type(req.Type), Family: req.Family, PortStart: req.PortStart, PortEnd: req.PortEnd, Protocol: firewall.Protocol(req.Protocol), Address: req.Address, Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationRemove); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FirewallService) GetIPRules(c fiber.Ctx) error {
	rules, err := s.firewall.ListRule()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	var filledRules []map[string]any
	for rule := range slices.Values(rules) {
		// 保留IP规则
		if rule.PortStart != 1 || rule.PortEnd != 65535 || rule.Address == "" {
			continue
		}
		filledRules = append(filledRules, map[string]any{
			"family":    rule.Family,
			"protocol":  rule.Protocol,
			"address":   rule.Address,
			"strategy":  rule.Strategy,
			"direction": rule.Direction,
		})
	}

	paged, total := Paginate(r, filledRules)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *FirewallService) CreateIPRule(c fiber.Ctx) error {
	req, err := Bind[request.FirewallIPRule](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.firewall.RichRules(firewall.FireInfo{
		Family: req.Family, Address: req.Address, Protocol: firewall.Protocol(req.Protocol), Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationAdd); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FirewallService) DeleteIPRule(c fiber.Ctx) error {
	req, err := Bind[request.FirewallIPRule](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.firewall.RichRules(firewall.FireInfo{
		Family: req.Family, Address: req.Address, Protocol: firewall.Protocol(req.Protocol), Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationRemove); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FirewallService) GetForwards(c fiber.Ctx) error {
	forwards, err := s.firewall.ListForward()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(r, forwards)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *FirewallService) CreateForward(c fiber.Ctx) error {
	req, err := Bind[request.FirewallForward](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.firewall.Forward(firewall.Forward{
		Protocol: firewall.Protocol(req.Protocol), Port: req.Port, TargetIP: req.TargetIP, TargetPort: req.TargetPort,
	}, firewall.OperationAdd); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FirewallService) DeleteForward(c fiber.Ctx) error {
	req, err := Bind[request.FirewallForward](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.firewall.Forward(firewall.Forward{
		Protocol: firewall.Protocol(req.Protocol), Port: req.Port, TargetIP: req.TargetIP, TargetPort: req.TargetPort,
	}, firewall.OperationRemove); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

package service

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/os"
)

type FirewallService struct {
	firewall firewall.Firewall
}

func NewFirewallService(i do.Injector) (*FirewallService, error) {
	return &FirewallService{
		firewall: firewall.NewFirewall(),
	}, nil
}

func (s *FirewallService) GetStatus(w http.ResponseWriter, r *http.Request) {
	running, err := s.firewall.Status()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, running)
}

func (s *FirewallService) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallStatus](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if req.Status {
		err = s.firewall.Enable()
	} else {
		err = s.firewall.Disable()
	}

	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallService) GetRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.firewall.ListRule()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
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

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *FirewallService) CreateRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallRule](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.firewall.Port(firewall.FireInfo{
		Type: firewall.Type(req.Type), Family: req.Family, PortStart: req.PortStart, PortEnd: req.PortEnd, Protocol: firewall.Protocol(req.Protocol), Address: req.Address, Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationAdd); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallService) DeleteRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallRule](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.firewall.Port(firewall.FireInfo{
		Type: firewall.Type(req.Type), Family: req.Family, PortStart: req.PortStart, PortEnd: req.PortEnd, Protocol: firewall.Protocol(req.Protocol), Address: req.Address, Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationRemove); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallService) GetIPRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.firewall.ListRule()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	filledRules := lo.FilterMap(rules, func(rule firewall.FireInfo, _ int) (map[string]any, bool) {
		// 保留IP规则
		if rule.PortStart != 1 || rule.PortEnd != 65535 || rule.Address == "" {
			return nil, false
		}
		return map[string]any{
			"family":    rule.Family,
			"protocol":  rule.Protocol,
			"address":   rule.Address,
			"strategy":  rule.Strategy,
			"direction": rule.Direction,
		}, true
	})

	paged, total := Paginate(r, filledRules)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *FirewallService) CreateIPRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallIPRule](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.firewall.RichRules(firewall.FireInfo{
		Family: req.Family, Address: req.Address, Protocol: firewall.Protocol(req.Protocol), Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationAdd); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallService) DeleteIPRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallIPRule](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.firewall.RichRules(firewall.FireInfo{
		Family: req.Family, Address: req.Address, Protocol: firewall.Protocol(req.Protocol), Strategy: firewall.Strategy(req.Strategy), Direction: firewall.Direction(req.Direction),
	}, firewall.OperationRemove); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallService) GetForwards(w http.ResponseWriter, r *http.Request) {
	forwards, err := s.firewall.ListForward()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, forwards)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *FirewallService) CreateForward(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallForward](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.firewall.Forward(firewall.Forward{
		Protocol: firewall.Protocol(req.Protocol), Port: req.Port, TargetIP: req.TargetIP, TargetPort: req.TargetPort,
	}, firewall.OperationAdd); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallService) DeleteForward(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallForward](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.firewall.Forward(firewall.Forward{
		Protocol: firewall.Protocol(req.Protocol), Port: req.Port, TargetIP: req.TargetIP, TargetPort: req.TargetPort,
	}, firewall.OperationRemove); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallService) GetPortUsage(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.ParseUint(r.URL.Query().Get("port"), 10, 32)
	if err != nil || port == 0 || port > 65535 {
		Error(w, http.StatusUnprocessableEntity, "invalid port")
		return
	}

	protocol := r.URL.Query().Get("protocol")
	if protocol != "tcp" && protocol != "udp" {
		protocol = "tcp"
	}

	Success(w, os.GetPortProcess(uint(port), protocol))
}

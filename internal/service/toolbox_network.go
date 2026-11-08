package service

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	panelnetwork "github.com/acepanel/panel/v3/pkg/network"
)

type ToolboxNetworkService struct {
	t       *gotext.Locale
	log     *slog.Logger
	network *panelnetwork.Service
}

func NewToolboxNetworkService(t *gotext.Locale, log *slog.Logger) *ToolboxNetworkService {
	return &ToolboxNetworkService{
		t:   t,
		log: log,
		network: panelnetwork.New(func(name string, err error) {
			if err != nil {
				log.Warn(t.Get("failed to roll back network interface"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", 0), slog.String("name", name), slog.Any("err", err))
				return
			}
			log.Info(t.Get("network interface change rolled back"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", 0), slog.String("name", name), slog.Bool("expired", true))
		}),
	}
}

type networkConnection struct {
	Type    string `json:"type"` // tcp, tcp6, udp, udp6
	PID     int32  `json:"pid"`
	Process string `json:"process"` // 进程名称
	Local   string `json:"local"`   // 本地地址:端口
	Remote  string `json:"remote"`  // 远程地址:端口
	State   string `json:"state"`   // LISTEN, ESTABLISHED 等
}

// Interfaces 获取可管理网卡列表
func (s *ToolboxNetworkService) Interfaces(w http.ResponseWriter, r *http.Request) {
	result, err := s.network.Interfaces(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get network interfaces: %v", err))
		return
	}

	s.log.Info(s.t.Get("network interface list viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, result)
}

// UpdateInterface 更新网卡配置，变更后需在超时前确认，否则自动回滚
func (s *ToolboxNetworkService) UpdateInterface(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[panelnetwork.Config](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if err = s.network.Update(r.Context(), *req); err != nil {
		if errors.Is(err, panelnetwork.ErrValidation) {
			Error(w, http.StatusUnprocessableEntity, "%v", err)
			return
		}
		Error(w, http.StatusInternalServerError, s.t.Get("failed to update network interface: %v", err))
		return
	}

	s.log.Info(s.t.Get("network interface updated"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("name", req.Name), slog.Int("mtu", req.MTU), slog.String("ipv4_mode", req.IPv4.Mode), slog.Any("ipv4_addresses", req.IPv4.Addresses), slog.String("ipv6_mode", req.IPv6.Mode), slog.Any("ipv6_addresses", req.IPv6.Addresses))
	Success(w, chix.M{"confirm_timeout": int(panelnetwork.ConfirmTimeout.Seconds())})
}

// ConfirmInterface 确认保留网卡变更
func (s *ToolboxNetworkService) ConfirmInterface(w http.ResponseWriter, r *http.Request) {
	if err := s.network.Confirm(); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	s.log.Info(s.t.Get("network interface change confirmed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, nil)
}

// RollbackInterface 立即撤销网卡变更
func (s *ToolboxNetworkService) RollbackInterface(w http.ResponseWriter, r *http.Request) {
	if err := s.network.Rollback(r.Context()); err != nil {
		if errors.Is(err, panelnetwork.ErrValidation) {
			Error(w, http.StatusUnprocessableEntity, "%v", err)
			return
		}
		Error(w, http.StatusInternalServerError, s.t.Get("failed to roll back network interface: %v", err))
		return
	}

	s.log.Info(s.t.Get("network interface change rolled back"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, nil)
}

// List 获取网络连接列表
func (s *ToolboxNetworkService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxNetworkList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 50
	}
	if req.Order == "" {
		req.Order = "asc"
	}

	conns, err := net.ConnectionsPid("all", 0)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 缓存 PID → 进程名
	nameCache := make(map[int32]string)
	getName := func(pid int32) string {
		if pid <= 0 {
			return ""
		}
		if name, ok := nameCache[pid]; ok {
			return name
		}
		proc, err := process.NewProcess(pid)
		if err != nil {
			nameCache[pid] = ""
			return ""
		}
		name, err := proc.Name()
		if err != nil {
			nameCache[pid] = ""
			return ""
		}
		nameCache[pid] = name
		return name
	}

	// 状态过滤集合
	stateSet := make(map[string]struct{})
	if req.State != "" {
		for st := range strings.SplitSeq(req.State, ",") {
			st = strings.TrimSpace(st)
			if st != "" {
				stateSet[strings.ToUpper(st)] = struct{}{}
			}
		}
	}

	data := make([]networkConnection, 0, len(conns))
	for conn := range slices.Values(conns) {
		typ := resolveConnType(conn.Family, conn.Type)
		if typ == "" {
			continue
		}

		state := strings.ToUpper(conn.Status)
		procName := getName(conn.Pid)
		local := fmt.Sprintf("%s:%d", conn.Laddr.IP, conn.Laddr.Port)
		remote := fmt.Sprintf("%s:%d", conn.Raddr.IP, conn.Raddr.Port)

		// 状态过滤
		if len(stateSet) > 0 {
			if _, ok := stateSet[state]; !ok {
				continue
			}
		}

		// PID 搜索
		if req.PID != "" {
			pidStr := strconv.FormatInt(int64(conn.Pid), 10)
			if !strings.Contains(pidStr, req.PID) {
				continue
			}
		}

		// 进程名称搜索
		if req.Process != "" {
			if !strings.Contains(strings.ToLower(procName), strings.ToLower(req.Process)) {
				continue
			}
		}

		// 端口搜索
		if req.Port != "" {
			localPort := strconv.FormatUint(uint64(conn.Laddr.Port), 10)
			remotePort := strconv.FormatUint(uint64(conn.Raddr.Port), 10)
			if !strings.Contains(localPort, req.Port) && !strings.Contains(remotePort, req.Port) {
				continue
			}
		}

		data = append(data, networkConnection{
			Type:    typ,
			PID:     conn.Pid,
			Process: procName,
			Local:   local,
			Remote:  remote,
			State:   state,
		})
	}

	// 排序
	s.sortConnections(data, req.Sort, req.Order)

	paged, total := Paginate(r, data)

	s.log.Info(s.t.Get("network connection list viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("state", req.State), slog.String("pid", req.PID), slog.String("process", req.Process), slog.String("port", req.Port), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))
	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// resolveConnType 将 Family + Type 转为可读字符串
func resolveConnType(family, typ uint32) string {
	switch {
	case family == 2 && typ == 1:
		return "tcp"
	case family == 10 && typ == 1:
		return "tcp6"
	case family == 2 && typ == 2:
		return "udp"
	case family == 10 && typ == 2:
		return "udp6"
	default:
		return ""
	}
}

// sortConnections 对连接列表排序
func (s *ToolboxNetworkService) sortConnections(data []networkConnection, sortBy, order string) {
	sort.Slice(data, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "type":
			less = data[i].Type < data[j].Type
		case "pid":
			less = data[i].PID < data[j].PID
		case "process":
			less = strings.ToLower(data[i].Process) < strings.ToLower(data[j].Process)
		default:
			less = data[i].PID < data[j].PID
		}
		if order == "desc" {
			return !less
		}
		return less
	})
}

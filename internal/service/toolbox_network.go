package service

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/acepanel/panel/v3/internal/request"
)

type ToolboxNetworkService struct{}

func NewToolboxNetworkService(i do.Injector) (*ToolboxNetworkService, error) {
	return &ToolboxNetworkService{}, nil
}

type networkConnection struct {
	Type    string `json:"type"` // tcp, tcp6, udp, udp6
	PID     int32  `json:"pid"`
	Process string `json:"process"` // 进程名称
	Local   string `json:"local"`   // 本地地址:端口
	Remote  string `json:"remote"`  // 远程地址:端口
	State   string `json:"state"`   // LISTEN, ESTABLISHED 等
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

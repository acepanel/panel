package service

import (
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/libtnb/chix"
	"github.com/shirou/gopsutil/process"

	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/types"
)

type ProcessService struct {
}

func NewProcessService() *ProcessService {
	return &ProcessService{}
}

func (s *ProcessService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProcessList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Order == "" {
		req.Order = "asc"
	}

	processes, err := process.Processes()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	data := make([]types.ProcessData, 0, len(processes))
	for proc := range slices.Values(processes) {
		procData := s.processProcessBasic(proc)

		// 状态筛选
		if req.Status != "" && procData.Status != req.Status {
			continue
		}

		// 关键词搜索（按 PID 或进程名）
		if req.Keyword != "" {
			keyword := strings.ToLower(req.Keyword)
			pidStr := strconv.FormatInt(int64(procData.PID), 10)
			nameMatch := strings.Contains(strings.ToLower(procData.Name), keyword)
			pidMatch := strings.Contains(pidStr, keyword)
			if !nameMatch && !pidMatch {
				continue
			}
		}

		data = append(data, procData)
	}

	// 排序
	if req.Sort != "" {
		s.sortProcesses(data, req.Sort, req.Order)
	}

	// 分页 - 使用 int64 避免溢出
	total := uint(len(data))
	start := uint64(req.Page-1) * uint64(req.Limit)
	end := uint64(req.Page) * uint64(req.Limit)

	if start > uint64(total) {
		data = []types.ProcessData{}
	} else {
		if end > uint64(total) {
			end = uint64(total)
		}
		data = data[start:end]
	}

	Success(w, chix.M{
		"total": total,
		"items": data,
	})
}

// sortProcesses 对进程列表进行排序
func (s *ProcessService) sortProcesses(data []types.ProcessData, sortBy, order string) {
	sort.Slice(data, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "pid":
			less = data[i].PID < data[j].PID
		case "name":
			less = strings.ToLower(data[i].Name) < strings.ToLower(data[j].Name)
		case "cpu":
			less = data[i].CPU < data[j].CPU
		case "rss":
			less = data[i].RSS < data[j].RSS
		case "start_time":
			less = data[i].StartTime < data[j].StartTime
		case "ppid":
			less = data[i].PPID < data[j].PPID
		case "num_threads":
			less = data[i].NumThreads < data[j].NumThreads
		default:
			less = data[i].PID < data[j].PID
		}
		if order == "desc" {
			return !less
		}
		return less
	})
}

func (s *ProcessService) Kill(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProcessKill](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	proc, err := process.NewProcess(req.PID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = proc.Kill(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// Signal 向进程发送信号
func (s *ProcessService) Signal(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProcessSignal](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	proc, err := process.NewProcess(req.PID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = proc.SendSignal(syscall.Signal(req.Signal)); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// Detail 获取进程详情
func (s *ProcessService) Detail(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProcessDetail](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	proc, err := process.NewProcess(req.PID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	data := s.processProcessFull(proc)

	Success(w, data)
}

// processProcessBasic 处理进程基本数据（用于列表，减少数据获取）
func (s *ProcessService) processProcessBasic(proc *process.Process) types.ProcessData {
	data := types.ProcessData{
		PID: proc.Pid,
	}

	if name, err := proc.Name(); err == nil {
		data.Name = name
	} else {
		data.Name = "<UNKNOWN>"
	}

	if username, err := proc.Username(); err == nil {
		data.Username = username
	}
	data.PPID, _ = proc.Ppid()
	data.Status, _ = proc.Status()
	data.Background, _ = proc.Background()
	if ct, err := proc.CreateTime(); err == nil {
		data.StartTime = time.Unix(ct/1000, 0).Format(time.DateTime)
	}
	data.NumThreads, _ = proc.NumThreads()
	data.CPU, _ = proc.CPUPercent()
	if mem, err := proc.MemoryInfo(); err == nil {
		data.RSS = mem.RSS
	}

	return data
}

// processProcessFull 处理进程完整数据（用于详情）
func (s *ProcessService) processProcessFull(proc *process.Process) types.ProcessData {
	data := s.processProcessBasic(proc)

	// 获取更多内存信息
	if mem, err := proc.MemoryInfo(); err == nil {
		data.RSS = mem.RSS
		data.Data = mem.Data
		data.VMS = mem.VMS
		data.HWM = mem.HWM
		data.Stack = mem.Stack
		data.Locked = mem.Locked
		data.Swap = mem.Swap
	}

	if ioStat, err := proc.IOCounters(); err == nil {
		data.DiskWrite = ioStat.WriteBytes
		data.DiskRead = ioStat.ReadBytes
	}

	data.Nets, _ = proc.NetIOCounters(false)
	data.Connections, _ = proc.Connections()
	data.CmdLine, _ = proc.Cmdline()
	data.OpenFiles, _ = proc.OpenFiles()
	data.Envs, _ = proc.Environ()
	data.OpenFiles = slices.Compact(data.OpenFiles)
	data.Envs = slices.Compact(data.Envs)

	// 获取可执行文件路径和工作目录
	data.Exe, _ = proc.Exe()
	data.Cwd, _ = proc.Cwd()

	return data
}

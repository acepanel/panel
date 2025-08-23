package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"
	"slices"
	"time"

	"github.com/libtnb/chix"
	"github.com/shirou/gopsutil/process"

	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/types"
)

type ProcessService struct {
}

func NewProcessService() *ProcessService {
	return &ProcessService{}
}

func (s *ProcessService) List(c fiber.Ctx) error {
	processes, err := process.Processes()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	data := make([]types.ProcessData, 0)
	for proc := range slices.Values(processes) {
		data = append(data, s.processProcess(proc))
	}

	paged, total := Paginate(c, data)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ProcessService) Kill(c fiber.Ctx) error {
	req, err := Bind[request.ProcessKill](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	proc, err := process.NewProcess(req.PID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = proc.Kill(); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

// processProcess 处理进程数据
func (s *ProcessService) processProcess(proc *process.Process) types.ProcessData {
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

	return data
}

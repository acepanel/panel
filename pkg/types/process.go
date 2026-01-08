package types

import (
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

type ProcessData struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	PPID       int32   `json:"ppid"`
	Username   string  `json:"username"`
	Status     string  `json:"status"`
	Background bool    `json:"background"`
	StartTime  string  `json:"start_time"`
	NumThreads int32   `json:"num_threads"`
	CPU        float64 `json:"cpu"`

	DiskRead  uint64 `json:"disk_read"`
	DiskWrite uint64 `json:"disk_write"`

	CmdLine string `json:"cmd_line"`
	Exe     string `json:"exe"` // 可执行文件路径
	Cwd     string `json:"cwd"` // 工作目录

	RSS    uint64 `json:"rss"`
	VMS    uint64 `json:"vms"`
	HWM    uint64 `json:"hwm"`
	Data   uint64 `json:"data"`
	Stack  uint64 `json:"stack"`
	Locked uint64 `json:"locked"`
	Swap   uint64 `json:"swap"`

	Envs []string `json:"envs"`

	OpenFiles   []process.OpenFilesStat `json:"open_files"`
	Connections []net.ConnectionStat    `json:"connections"`
	Nets        []net.IOCountersStat    `json:"nets"`
}

package benchmark

import (
	"context"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// MachineInfo 被测机器的概况。多核得分要对照物理核心数看：云服务器的两个 vCPU 往往是同一个核心的两个超线程
type MachineInfo struct {
	Model string `json:"model"`
	// Cores 物理核心数，取不到时为 0
	Cores int `json:"cores"`
	// Threads 多线程测试并行的线程数
	Threads int    `json:"threads"`
	Memory  uint64 `json:"memory"`
}

// Machine 采集被测机器的概况，取不到的项留空
func Machine(ctx context.Context) MachineInfo {
	// 不少 ARM 机器的 /proc/cpuinfo 没有型号，至少给出架构
	info := MachineInfo{Model: runtime.GOARCH, Threads: runtime.GOMAXPROCS(0)}
	if cpus, err := cpu.InfoWithContext(ctx); err == nil && len(cpus) > 0 && cpus[0].ModelName != "" {
		info.Model = cpus[0].ModelName
	}
	if cores, err := cpu.CountsWithContext(ctx, false); err == nil {
		// 限制了可用核数的容器里，物理核心数可能比能用的线程还多
		info.Cores = min(cores, info.Threads)
	}
	if stat, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		info.Memory = stat.Total
	}
	return info
}

// Package tools 存放辅助方法
package tools

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	stdnet "net"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

// CurrentInfo 获取监控数据
func CurrentInfo(nets, disks []string) types.CurrentInfo {
	var res types.CurrentInfo
	res.Cpus, _ = cpu.Info()
	res.Percents, _ = cpu.Percent(100*time.Millisecond, true)
	percent, _ := cpu.Percent(100*time.Millisecond, false)
	if len(percent) > 0 {
		res.Percent = percent[0]
	}
	res.Load, _ = load.Avg()
	res.Host, _ = host.Info()
	res.Mem, _ = mem.VirtualMemory()
	res.Swap, _ = mem.SwapMemory()
	// 硬盘IO
	ioCounters, _ := disk.IOCounters(disks...)
	for _, info := range ioCounters {
		res.DiskIO = append(res.DiskIO, info)
	}
	// 硬盘使用
	var excludes = []string{"/dev", "/boot", "/sys", "/dev", "/run", "/proc", "/usr", "/var", "/snap"}
	excludes = append(excludes, "/mnt/cdrom") // CDROM
	excludes = append(excludes, "/mnt/wsl")   // Windows WSL
	res.Disk, _ = disk.Partitions(false)
	res.Disk = slices.DeleteFunc(res.Disk, func(d disk.PartitionStat) bool {
		for _, exclude := range excludes {
			if strings.HasPrefix(d.Mountpoint, exclude) {
				return true
			}
			// 去除内存盘和overlay容器盘
			if slices.Contains([]string{"tmpfs", "overlay"}, d.Fstype) {
				continue
			}
		}
		return false
	})
	// 分区使用
	for _, partition := range res.Disk {
		usage, _ := disk.Usage(partition.Mountpoint)
		res.DiskUsage = append(res.DiskUsage, *usage)
	}
	// 网络
	if len(nets) == 0 {
		netInfo, _ := net.IOCounters(true)
		res.Net = netInfo
	} else {
		var netStats []net.IOCountersStat
		netInfo, _ := net.IOCounters(true)
		for _, state := range netInfo {
			if slices.Contains(nets, state.Name) {
				netStats = append(netStats, state)
			}
		}
		res.Net = netStats
	}

	res.Time = time.Now()
	return res
}

// CollectTopProcesses 采集各指标 Top 5 进程
func CollectTopProcesses() types.TopProcesses {
	procs, err := process.Processes()
	if err != nil {
		return types.TopProcesses{}
	}

	type procMetrics struct {
		pid      int32
		name     string
		username string
		cmdline  string
		cpu      float64
		rss      uint64
		ioRead   uint64
		ioWrite  uint64
	}

	metrics := make([]procMetrics, 0, len(procs))
	for _, p := range procs {
		name, _ := p.Name()
		if name == "" {
			continue
		}

		m := procMetrics{pid: p.Pid, name: name}
		m.username, _ = p.Username()
		m.cmdline, _ = p.Cmdline()
		if len(m.cmdline) > 80 {
			m.cmdline = m.cmdline[:80]
		}
		m.cpu, _ = p.CPUPercent()
		if memInfo, err := p.MemoryInfo(); err == nil && memInfo != nil {
			m.rss = memInfo.RSS
		}
		if ioCounters, err := p.IOCounters(); err == nil && ioCounters != nil {
			m.ioRead = ioCounters.ReadBytes
			m.ioWrite = ioCounters.WriteBytes
		}

		metrics = append(metrics, m)
	}

	var result types.TopProcesses
	topN := 5

	// CPU Top 5
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].cpu > metrics[j].cpu })
	for i := range min(topN, len(metrics)) {
		m := metrics[i]
		if m.cpu <= 0 {
			break
		}
		result.CPU = append(result.CPU, types.ProcessStat{
			PID: m.pid, Name: m.name, Username: m.username, Command: m.cmdline,
			Value: m.cpu,
		})
	}

	// 内存 Top 5
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].rss > metrics[j].rss })
	for i := range min(topN, len(metrics)) {
		m := metrics[i]
		if m.rss == 0 {
			break
		}
		result.Memory = append(result.Memory, types.ProcessStat{
			PID: m.pid, Name: m.name, Username: m.username, Command: m.cmdline,
			Value: float64(m.rss),
		})
	}

	// 磁盘 IO Top 5（按累计读+写排序）
	sort.Slice(metrics, func(i, j int) bool {
		return (metrics[i].ioRead + metrics[i].ioWrite) > (metrics[j].ioRead + metrics[j].ioWrite)
	})
	for i := range min(topN, len(metrics)) {
		m := metrics[i]
		if m.ioRead+m.ioWrite == 0 {
			break
		}
		result.DiskIO = append(result.DiskIO, types.ProcessStat{
			PID: m.pid, Name: m.name, Username: m.username, Command: m.cmdline,
			Value: float64(m.ioRead + m.ioWrite),
			Read:  float64(m.ioRead),
			Write: float64(m.ioWrite),
		})
	}

	return result
}

// StopPanel 停止面板
func StopPanel() {
	_ = shell.ExecfAsync("sleep 1 && systemctl stop acepanel")
}

// RestartPanel 重启面板
func RestartPanel() {
	_ = shell.ExecfAsync("sleep 1 && systemctl restart acepanel")
}

// RestartServer 重启服务器
func RestartServer() {
	_ = shell.ExecfAsync("sleep 1 && reboot")
}

// IsChina 是否中国大陆
func IsChina() bool {
	client := resty.New()
	defer func(client *resty.Client) { _ = client.Close() }(client)
	client.SetLogger(NoopLogger{})
	client.SetLoggerWarnLevel(true)
	client.SetTimeout(3 * time.Second)
	client.SetRetryCount(3)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	resp, err := client.R().Get("https://perfops.cloudflareperf.com/cdn-cgi/trace")
	if err != nil || !resp.IsStatusSuccess() {
		return false
	}

	if strings.Contains(resp.String(), "loc=CN") {
		return true
	}

	return false
}

// GetPublicIPv4 获取公网IPv4
func GetPublicIPv4() (string, error) {
	client := resty.New()
	defer func(client *resty.Client) { _ = client.Close() }(client)
	client.SetLogger(NoopLogger{})
	client.SetLoggerWarnLevel(true)
	client.SetTimeout(3 * time.Second)
	client.SetRetryCount(3)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	client.SetTransport(&http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (stdnet.Conn, error) {
			return (&stdnet.Dialer{}).DialContext(ctx, "tcp4", addr)
		},
	})

	resp, err := client.R().Get("https://perfops.cloudflareperf.com/cdn-cgi/trace")
	if err != nil || !resp.IsStatusSuccess() {
		return "", errors.New("failed to get public ipv4 address")
	}

	return strings.TrimPrefix(strings.Split(resp.String(), "\n")[2], "ip="), nil
}

// GetPublicIPv6 获取公网IPv6
func GetPublicIPv6() (string, error) {
	client := resty.New()
	defer func(client *resty.Client) { _ = client.Close() }(client)
	client.SetLogger(NoopLogger{})
	client.SetLoggerWarnLevel(true)
	client.SetTimeout(3 * time.Second)
	client.SetRetryCount(3)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	client.SetTransport(&http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (stdnet.Conn, error) {
			return (&stdnet.Dialer{}).DialContext(ctx, "tcp6", addr)
		},
	})

	resp, err := client.R().Get("https://perfops.cloudflareperf.com/cdn-cgi/trace")
	if err != nil || !resp.IsStatusSuccess() {
		return "", errors.New("failed to get public ipv6 address")
	}

	return strings.TrimPrefix(strings.Split(resp.String(), "\n")[2], "ip="), nil
}

// GetLocalIPv4 获取本地IPv4
func GetLocalIPv4() (string, error) {
	conn, err := stdnet.Dial("udp", "119.29.29.29:53")
	if err != nil {
		return "", err
	}
	defer func(conn stdnet.Conn) { _ = conn.Close() }(conn)

	local := conn.LocalAddr().(*stdnet.UDPAddr)
	return local.IP.String(), nil
}

// GetLocalIPv6 获取本地IPv6
func GetLocalIPv6() (string, error) {
	conn, err := stdnet.Dial("udp", "[2402:4e00::]:53")
	if err != nil {
		return "", err
	}
	defer func(conn stdnet.Conn) { _ = conn.Close() }(conn)

	local := conn.LocalAddr().(*stdnet.UDPAddr)
	return local.IP.String(), nil
}

// CollectLocalNames 收集本机所有 IP 地址
func CollectLocalNames() []string {
	var names []string
	if lv4, err := GetLocalIPv4(); err == nil && !slices.Contains(names, lv4) {
		names = append(names, lv4)
	}
	if lv6, err := GetLocalIPv6(); err == nil && !slices.Contains(names, lv6) {
		names = append(names, lv6)
	}
	if rv4, err := GetPublicIPv4(); err == nil && !slices.Contains(names, rv4) {
		names = append(names, rv4)
	}
	if rv6, err := GetPublicIPv6(); err == nil && !slices.Contains(names, rv6) {
		names = append(names, rv6)
	}
	return names
}

// FormatBytes 格式化bytes
func FormatBytes(size float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}

	i := 0
	for ; size >= 1024 && i < len(units); i++ {
		size /= 1024
	}

	return fmt.Sprintf("%.2f %s", size, units[i])
}

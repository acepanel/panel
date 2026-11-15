// Package tools 存放辅助方法
package tools

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	stdnet "net"
	"net/http"
	"net/netip"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
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
	if len(res.Percents) > 0 {
		res.Percent = lo.Sum(res.Percents) / float64(len(res.Percents))
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
	excludes := []string{"/dev", "/boot", "/sys", "/run", "/proc", "/usr", "/var", "/snap"}
	excludes = append(excludes, "/mnt/cdrom") // CDROM
	excludes = append(excludes, "/mnt/wsl")   // Windows WSL
	res.Disk, _ = disk.Partitions(false)
	res.Disk = slices.DeleteFunc(res.Disk, func(d disk.PartitionStat) bool {
		for _, exclude := range excludes {
			if strings.HasPrefix(d.Mountpoint, exclude) {
				return true
			}
		}
		return false
	})
	// 分区使用
	for _, partition := range res.Disk {
		// 挂载点在两次调用之间消失或不可读时返回 nil，直接解引用会 panic
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil || usage == nil {
			continue
		}
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
		proc    *process.Process
		pid     int32
		name    string
		cpu     float64
		rss     uint64
		ioRead  uint64
		ioWrite uint64
	}

	metrics := make([]procMetrics, 0, len(procs))
	for _, p := range procs {
		name, _ := p.Name()
		if name == "" {
			continue
		}

		m := procMetrics{proc: p, pid: p.Pid, name: name}
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

	type procDetail struct{ username, cmdline string }
	details := make(map[int32]procDetail, topN*3)
	statOf := func(m procMetrics, value float64) types.ProcessStat {
		d, ok := details[m.pid]
		if !ok {
			d.username, _ = m.proc.Username()
			d.cmdline, _ = m.proc.Cmdline()
			if len(d.cmdline) > 80 {
				d.cmdline = d.cmdline[:80]
			}
			details[m.pid] = d
		}
		return types.ProcessStat{
			PID: m.pid, Name: m.name, Username: d.username, Command: d.cmdline,
			Value: value,
		}
	}

	// CPU Top 5
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].cpu > metrics[j].cpu })
	for i := range min(topN, len(metrics)) {
		m := metrics[i]
		if m.cpu <= 0 {
			break
		}
		result.CPU = append(result.CPU, statOf(m, m.cpu))
	}

	// 内存 Top 5
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].rss > metrics[j].rss })
	for i := range min(topN, len(metrics)) {
		m := metrics[i]
		if m.rss == 0 {
			break
		}
		result.Memory = append(result.Memory, statOf(m, float64(m.rss)))
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
		stat := statOf(m, float64(m.ioRead+m.ioWrite))
		stat.Read = float64(m.ioRead)
		stat.Write = float64(m.ioWrite)
		result.DiskIO = append(result.DiskIO, stat)
	}

	return result
}

// 以下两个自杀式操作交给 systemd 的瞬态定时器延迟 1 秒执行，既熬过 HTTP 响应，
// 又脱离面板自身的 cgroup，不会在停服务时被连带杀掉而误报命令失败

// RestartPanel 重启面板
func RestartPanel(ctx context.Context) {
	_ = shell.ExecfAsync(ctx, "systemd-run --quiet --collect --on-active=1 systemctl restart acepanel")
}

// RestartServer 重启服务器
func RestartServer(ctx context.Context) {
	_ = shell.ExecfAsync(ctx, "systemd-run --quiet --collect --on-active=1 systemctl reboot")
}

// GetPublicIPv4 获取公网IPv4
func GetPublicIPv4() (string, error) {
	return publicIP("tcp4")
}

// GetPublicIPv6 获取公网IPv6
func GetPublicIPv6() (string, error) {
	return publicIP("tcp6")
}

// GetLocalIPv4 获取本地IPv4
func GetLocalIPv4() (string, error) {
	return localIPOf(context.Background(), "119.29.29.29:53")
}

// GetLocalIPv6 获取本地IPv6
func GetLocalIPv6() (string, error) {
	return localIPOf(context.Background(), "[2402:4e00::]:53")
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

// WrapIPv6 为裸 IPv6 地址套上方括号（如 ::1 → [::1]），用于 nginx server_name、URL 等语境
// 非 IPv6 或已带方括号则原样返回
func WrapIPv6(host string) string {
	if isIPv6(host) {
		return "[" + host + "]"
	}
	return host
}

// UnwrapIPv6 去除 IPv6 地址的方括号（如 [::1] → ::1），非此形式则原样返回
func UnwrapIPv6(host string) string {
	if inner, ok := strings.CutPrefix(host, "["); ok {
		if inner, ok = strings.CutSuffix(inner, "]"); ok && isIPv6(inner) {
			return inner
		}
	}
	return host
}

func publicIP(network string) (string, error) {
	client := resty.New()
	defer func(client *resty.Client) { _ = client.Close() }(client)
	client.SetLogger(noopLogger{})
	client.SetLoggerWarnLevel(true)
	client.SetTimeout(3 * time.Second)
	client.SetRetryCount(3)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	client.SetTransport(&http.Transport{
		DialContext: func(ctx context.Context, _ string, addr string) (stdnet.Conn, error) {
			return (&stdnet.Dialer{}).DialContext(ctx, network, addr)
		},
	})

	resp, err := client.R().Get("https://perfops.cloudflareperf.com/cdn-cgi/trace")
	if err != nil || !resp.IsStatusSuccess() {
		return "", fmt.Errorf("failed to get public address over %s", network)
	}

	return traceIP(resp.String())
}

// traceIP 从 cdn-cgi/trace 响应里取 ip 行
func traceIP(body string) (string, error) {
	for line := range strings.SplitSeq(body, "\n") {
		if ip, ok := strings.CutPrefix(strings.TrimSpace(line), "ip="); ok {
			if _, err := netip.ParseAddr(ip); err != nil {
				return "", err
			}
			return ip, nil
		}
	}

	return "", errors.New("no ip in trace response")
}

func localIPOf(ctx context.Context, addr string) (string, error) {
	conn, err := (&stdnet.Dialer{}).DialContext(ctx, "udp", addr)
	if err != nil {
		return "", err
	}
	defer func(conn stdnet.Conn) { _ = conn.Close() }(conn)

	local, ok := conn.LocalAddr().(*stdnet.UDPAddr)
	if !ok {
		return "", errors.New("failed to get local address")
	}
	return local.IP.String(), nil
}

// isIPv6 判断 host 是否为 IPv6 地址（裸地址，不含方括号）
func isIPv6(host string) bool {
	addr, err := netip.ParseAddr(host)
	return err == nil && !addr.Is4()
}

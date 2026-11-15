package os

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"slices"
	"strings"
	"syscall"

	psnet "github.com/shirou/gopsutil/v4/net"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// PortSet 是升序去重的端口列表
type PortSet []uint

// Overlaps 判断是否有端口落在 [start, end] 内
func (s PortSet) Overlaps(start, end uint) bool {
	i, _ := slices.BinarySearch(s, start)
	return i < len(s) && s[i] <= end
}

// ListeningPorts 一次性列出本机被占用的端口
// TCP 只算 LISTEN 状态；UDP 没有监听状态，已绑定即算
func ListeningPorts(ctx context.Context) (tcp, udp PortSet, err error) {
	conns, err := psnet.ConnectionsWithoutUidsWithContext(ctx, "inet")
	if err != nil {
		return nil, nil, err
	}

	for _, conn := range conns {
		switch {
		case conn.Type == syscall.SOCK_STREAM && conn.Status == "LISTEN":
			tcp = append(tcp, uint(conn.Laddr.Port))
		case conn.Type == syscall.SOCK_DGRAM:
			udp = append(udp, uint(conn.Laddr.Port))
		}
	}
	slices.Sort(tcp)
	slices.Sort(udp)

	return slices.Compact(tcp), slices.Compact(udp), nil
}

// PortProcess 端口占用进程信息
type PortProcess struct {
	PID     string `json:"pid"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

// GetPortProcess 获取占用指定端口的进程信息
func GetPortProcess(ctx context.Context, port uint, protocol string) []PortProcess {
	var flag string
	switch protocol {
	case "udp":
		flag = "-ulnp"
	default:
		flag = "-tlnp"
	}

	output, err := shell.Execf(ctx, "ss %s sport = :%d", flag, port)
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`\("([^"]*)",pid=(\d+),`)
	seen := make(map[string]struct{})
	var processes []PortProcess

	for line := range strings.SplitSeq(output, "\n") {
		matches := re.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			pid := match[2]
			if _, ok := seen[pid]; ok {
				continue
			}
			seen[pid] = struct{}{}

			name := match[1]
			command := name
			if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%s/cmdline", pid)); err == nil {
				// cmdline 以 \0 分隔参数
				cmd := strings.ReplaceAll(strings.TrimRight(string(cmdline), "\x00"), "\x00", " ")
				if cmd != "" {
					command = cmd
				}
			}

			processes = append(processes, PortProcess{
				PID:     pid,
				Name:    name,
				Command: command,
			})
		}
	}

	return processes
}

func TCPPortInUse(port uint) bool {
	var lc net.ListenConfig
	conn, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true
	}
	defer func(conn net.Listener) { _ = conn.Close() }(conn)
	return false
}

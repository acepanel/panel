package systemctl

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// ServiceInfo 服务详细信息
type ServiceInfo struct {
	Status string  // 运行状态 (active, inactive, failed, etc.)
	PID    int     // 主进程 PID
	Memory int64   // 内存使用（字节）
	CPU    float64 // CPU 使用率
	Uptime string  // 运行时间
}

// Unit 系统单元
type Unit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListUnits 列出可被依赖的 service、socket、target、mount 单元
func ListUnits(ctx context.Context) ([]Unit, error) {
	var files []struct {
		UnitFile string `json:"unit_file"`
		State    string `json:"state"`
	}
	var loaded []struct {
		Unit        string `json:"unit"`
		Load        string `json:"load"`
		Description string `json:"description"`
	}
	if err := listJSON(ctx, "list-unit-files", &files); err != nil {
		return nil, err
	}
	if err := listJSON(ctx, "list-units --all", &loaded); err != nil {
		return nil, err
	}

	// 未启用的服务不会被加载，只能从 unit 文件列表里拿到，描述则只有已加载的单元才有
	descriptions := make(map[string]string)
	for _, f := range files {
		// 模板单元要实例化才能引用，屏蔽的单元无法启动
		if !strings.Contains(f.UnitFile, "@.") && !strings.HasPrefix(f.State, "masked") {
			descriptions[f.UnitFile] = ""
		}
	}
	for _, u := range loaded {
		if u.Load == "loaded" {
			descriptions[u.Unit] = u.Description
		}
	}

	units := make([]Unit, 0, len(descriptions))
	for name, description := range descriptions {
		units = append(units, Unit{Name: name, Description: description})
	}
	// 忽略大小写，否则 NetworkManager 这类大写开头的会排到最前面
	slices.SortFunc(units, func(a, b Unit) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return units, nil
}

// GetServiceInfo 获取服务详细信息
func GetServiceInfo(ctx context.Context, name string) (*ServiceInfo, error) {
	output, err := shell.Execf(ctx, "systemctl show '%s' --property=ActiveState,MainPID,ExecMainStartTimestamp --no-pager", name)
	if err != nil {
		return nil, err
	}

	info := &ServiceInfo{}
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch key {
		case "ActiveState":
			info.Status = value
		case "MainPID":
			info.PID = cast.ToInt(value)
		case "ExecMainStartTimestamp":
			// 格式: Mon 2024-01-01 12:00:00 UTC
			if value != "" && value != "n/a" {
				info.Uptime = calcUptime(value)
			}
		}
	}

	if info.PID > 0 {
		if proc, err := process.NewProcess(int32(info.PID)); err == nil { //nolint:gosec
			// 获取内存信息
			if memInfo, err := proc.MemoryInfo(); err == nil && memInfo != nil {
				info.Memory = int64(memInfo.RSS) //nolint:gosec
			}
			// 获取 CPU 使用率
			if cpu, err := proc.CPUPercent(); err == nil {
				info.CPU = cpu
			}
		}
	}

	return info, nil
}

// Status 获取服务状态
func Status(ctx context.Context, name string) (bool, error) {
	// is-active 在服务未运行时返回退出码 3，只有 ctx 出错才是真失败，否则取消会被误判成「服务未运行」
	output, err := shell.Execf(ctx, "systemctl is-active '%s'", name)
	if err != nil && ctx.Err() != nil {
		return false, ctx.Err()
	}
	return output == "active", nil
}

// IsEnabled 服务是否启用
func IsEnabled(ctx context.Context, name string) (bool, error) {
	// is-enabled 在服务禁用时返回退出码 1，同 Status 只认 ctx 错误
	out, err := shell.Execf(ctx, "systemctl is-enabled '%s'", name)
	if err != nil && ctx.Err() != nil {
		return false, ctx.Err()
	}
	return out == "enabled" || out == "static" || out == "indirect", nil
}

// Start 启动服务
func Start(ctx context.Context, name string) error {
	return change(ctx, "systemctl start '%s'", name)
}

// Stop 停止服务
func Stop(ctx context.Context, name string) error {
	return change(ctx, "systemctl stop '%s'", name)
}

// Restart 重启服务
func Restart(ctx context.Context, name string) error {
	return change(ctx, "systemctl restart '%s'", name)
}

// Reload 重载服务
func Reload(ctx context.Context, name string) error {
	return change(ctx, "systemctl reload '%s'", name)
}

// Enable 启用服务
func Enable(ctx context.Context, name string) error {
	return change(ctx, "systemctl enable '%s'", name)
}

// Disable 禁用服务
func Disable(ctx context.Context, name string) error {
	return change(ctx, "systemctl disable '%s'", name)
}

// DaemonReload 重载 systemd 服务配置
func DaemonReload(ctx context.Context) error {
	return change(ctx, "systemctl daemon-reload")
}

func listJSON(ctx context.Context, command string, v any) error {
	output, err := shell.Execf(ctx, "systemctl %s --type=service,socket,target,mount --output=json --no-pager", command)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(output), v)
}

// calcUptime 计算运行时间
func calcUptime(startTime string) string {
	// 解析时间格式: Mon 2024-01-01 12:00:00 UTC
	// 或者: Mon 2024-01-01 12:00:00 CST
	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -0700",
	}

	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.Parse(layout, startTime)
		if err == nil {
			break
		}
	}
	if err != nil {
		return ""
	}

	duration := time.Since(t)
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}
	if hours > 0 {
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}
	return strconv.Itoa(minutes) + "m"
}

// change 执行 systemd 变更命令
// 变更一旦下发就没有回头路，断开请求 ctx 的取消链，避免连接断开留下「配置改了服务没生效」的半截状态
func change(ctx context.Context, shellCmd string, args ...any) error {
	_, err := shell.ExecfWithTimeout(context.WithoutCancel(ctx), 2*time.Minute, shellCmd, args...)
	return err
}

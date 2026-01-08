package systemctl

import (
	"time"

	"github.com/acepanel/panel/pkg/shell"
)

// Status 获取服务状态
func Status(name string) (bool, error) {
	output, _ := shell.Execf("systemctl is-active '%s'", name) // 不判断错误，因为 is-active 在服务未启用时会返回 3
	return output == "active", nil
}

// IsEnabled 服务是否启用
func IsEnabled(name string) (bool, error) {
	out, _ := shell.Execf("systemctl is-enabled '%s'", name) // 不判断错误，因为 is-enabled 在服务禁用时会返回 1
	return out == "enabled" || out == "static" || out == "indirect", nil
}

// Start 启动服务
func Start(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl start '%s'", name)
	return err
}

// Stop 停止服务
func Stop(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl stop '%s'", name)
	return err
}

// Restart 重启服务
func Restart(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl restart '%s'", name)
	return err
}

// Reload 重载服务
func Reload(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl reload '%s'", name)
	return err
}

// Enable 启用服务
func Enable(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl enable '%s'", name)
	return err
}

// Disable 禁用服务
func Disable(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl disable '%s'", name)
	return err
}

// Mask 屏蔽服务
func Mask(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl mask '%s'", name)
	return err
}

// Unmask 解除屏蔽服务
func Unmask(name string) error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl unmask '%s'", name)
	return err
}

// Log 获取服务日志
func Log(name string) (string, error) {
	return shell.ExecfWithTimeout(2*time.Minute, "journalctl -u '%s'", name)
}

// LogTail 获取服务日志
func LogTail(name string, lines int) (string, error) {
	return shell.ExecfWithTimeout(2*time.Minute, "journalctl -u '%s' --lines '%d'", name, lines)
}

// LogClear 清空服务日志
func LogClear(name string) error {
	if _, err := shell.Execf("journalctl --rotate -u '%s'", name); err != nil {
		return err
	}
	_, err := shell.Execf("journalctl --vacuum-time=1s -u '%s'", name)
	return err
}

// DaemonReload 重载 systemd 服务配置
func DaemonReload() error {
	_, err := shell.ExecfWithTimeout(2*time.Minute, "systemctl daemon-reload")
	return err
}

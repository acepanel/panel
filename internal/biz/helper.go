package biz

import (
	"context"
	"strings"

	"github.com/spf13/cast"
)

// OperatorID 从 context 获取操作员 ID，无法获取时返回 0（系统操作），供各层记操作日志用
func OperatorID(ctx context.Context) uint64 {
	if ctx == nil {
		return 0
	}
	userID := ctx.Value("user_id")
	if userID == nil {
		return 0
	}
	return cast.ToUint64(userID)
}

// containerSock 从设置读取容器 socket 路径，未配置或失败时返回默认值
func containerSock(setting SettingRepo) string {
	sock, _ := setting.Get(SettingKeyContainerSock)
	if sock == "" {
		sock = "/var/run/docker.sock"
	}
	// 自动补全 scheme
	if !strings.Contains(sock, "://") {
		sock = "unix://" + sock
	}
	return sock
}

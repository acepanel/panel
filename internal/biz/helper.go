package biz

import (
	"context"
	"strings"

	"github.com/libtnb/utils/crypt"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
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

// ContainerSock 从设置读取容器 socket 路径，未配置或失败时返回默认值
func ContainerSock(setting SettingRepo) string {
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

// encryptFields 用面板密钥原地加密待落库的字段
func encryptFields[T ~string | ~[]byte](fields ...*T) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}
	for _, field := range fields {
		encrypted, err := crypter.Encrypt([]byte(*field))
		if err != nil {
			return err
		}
		*field = T(encrypted)
	}
	return nil
}

// decryptFields 原地解密读出的字段，解不开的保持原值
func decryptFields[T ~string | ~[]byte](fields ...*T) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}
	for _, field := range fields {
		if plain, err := crypter.Decrypt(string(*field)); err == nil {
			*field = T(plain)
		}
	}
	return nil
}

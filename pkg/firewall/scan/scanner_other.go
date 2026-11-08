//go:build !linux

package scan

import (
	"errors"
	"log/slog"

	"github.com/leonelquinteros/gotext"
)

// Scanner eBPF 扫描检测器（非 Linux 平台占位）
type Scanner struct{}

// Supported 非 Linux 平台不支持 eBPF
func Supported() bool {
	return false
}

// New 非 Linux 平台不支持
func New(_ []string, _ *gotext.Locale, _ *slog.Logger) (*Scanner, error) {
	return nil, errors.New("not support eBPF")
}

// Events 返回空通道
func (s *Scanner) Events() <-chan Event {
	return nil
}

// Close 无操作
func (s *Scanner) Close() error {
	return nil
}

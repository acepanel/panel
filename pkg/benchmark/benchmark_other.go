//go:build !linux

package benchmark

import (
	"context"
	"errors"
)

// Disk 非 Linux 平台不支持
func Disk(_ context.Context, _ string) (DiskResult, error) {
	return DiskResult{}, errors.New("disk benchmark is only supported on Linux")
}

// smallPages 非 Linux 平台没有透明大页
func smallPages(_ []byte) {}

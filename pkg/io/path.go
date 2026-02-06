package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/acepanel/panel/pkg/shell"
)

// Remove 删除文件/目录
func Remove(path string) error {
	_, _ = shell.Execf("chattr -R -ia '%s'", path)
	return os.RemoveAll(path)
}

// Chmod 修改文件/目录权限
func Chmod(path string, permission os.FileMode) error {
	_, err := shell.Execf("chmod -R '%o' '%s'", permission, path)
	return err
}

// Chown 修改文件或目录所有者
func Chown(path, user, group string) error {
	_, err := shell.Execf("chown -R '%s:%s' '%s'", user, group, path)
	return err
}

// Exists 判断路径是否存在
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Empty 判断路径是否为空
func Empty(path string) bool {
	files, err := os.ReadDir(path)
	if err != nil {
		return true
	}

	return len(files) == 0
}

func Mv(src, dst string) error {
	_, err := shell.Execf(`mv -f '%s' '%s'`, src, dst)
	return err
}

// Cp 复制文件或目录（保留所有权和权限）
func Cp(src, dst string) error {
	_, err := shell.Execf(`cp -arf '%s' '%s'`, src, dst)
	return err
}

// Size 获取路径大小
func Size(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		size += info.Size()
		return nil
	})

	return size, err
}

// IsDir 判断是否为目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// SizeX 获取路径大小（du命令）
func SizeX(path string) (int64, error) {
	out, err := shell.Execf("du -sb '%s'", path)
	if err != nil {
		return 0, err
	}

	parts := strings.Fields(out)
	if len(parts) == 0 {
		return 0, fmt.Errorf("无法解析 du 输出")
	}

	return strconv.ParseInt(parts[0], 10, 64)
}

// CountX 统计目录下文件数
func CountX(path string) (int64, error) {
	out, err := shell.Execf("find '%s' -printf '.'", path)
	if err != nil {
		return 0, err
	}

	count := len(out)
	return int64(count), nil
}

package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"resty.dev/v3"

	"github.com/acepanel/panel/v3/pkg/chattr"
	"github.com/acepanel/panel/v3/pkg/shell"
)

// Write 写入文件
func Write(path string, data string, permission os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), permission); err != nil {
		return err
	}

	iFlag, aFlag := false, false
	file, err := os.OpenFile(path, os.O_RDONLY, permission)
	if err == nil {
		iFlag, _ = chattr.IsAttr(file, chattr.FS_IMMUTABLE_FL)
		aFlag, _ = chattr.IsAttr(file, chattr.FS_APPEND_FL)
		if iFlag {
			_ = chattr.UnsetAttr(file, chattr.FS_IMMUTABLE_FL)
		}
		if aFlag {
			_ = chattr.UnsetAttr(file, chattr.FS_APPEND_FL)
		}

		// 关闭文件重新以写入方式打开
		if err = file.Close(); err != nil {
			return err
		}
	}
	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, permission)
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	_, err = file.WriteString(data)
	if err != nil {
		return err
	}

	if iFlag {
		_ = chattr.SetAttr(file, chattr.FS_IMMUTABLE_FL)
	}
	if aFlag {
		_ = chattr.SetAttr(file, chattr.FS_APPEND_FL)
	}

	return nil
}

// WriteAppend 追加写入文件
func WriteAppend(path string, data string, permission os.FileMode) error {
	iFlag := false
	file, err := os.OpenFile(path, os.O_RDONLY, permission)
	if err == nil {
		iFlag, _ = chattr.IsAttr(file, chattr.FS_IMMUTABLE_FL)
		if iFlag {
			_ = chattr.UnsetAttr(file, chattr.FS_IMMUTABLE_FL)
		}

		// 关闭文件重新以写入方式打开
		if err = file.Close(); err != nil {
			return err
		}
	}
	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, permission)
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	_, err = file.WriteString(data)
	if err != nil {
		return err
	}

	if iFlag {
		_ = chattr.SetAttr(file, chattr.FS_IMMUTABLE_FL)
	}

	return nil
}

// Read 读取文件
func Read(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

// IsSymlink 判读是否为软链接
func IsSymlink(mode os.FileMode) bool {
	return mode&os.ModeSymlink != 0
}

// IsHidden 判断是否为隐藏文件
func IsHidden(path string) bool {
	_, file := filepath.Split(path)
	return strings.HasPrefix(file, ".")
}

// GetSymlink 获取软链接目标
func GetSymlink(path string) string {
	linkPath, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return linkPath
}

// DownloadFile 下载文件到指定路径，使用 .tmp 原子替换
func DownloadFile(url, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmpPath := destPath + ".tmp"
	client := resty.New()
	defer func() { _ = client.Close() }()
	defer func() { _ = os.Remove(tmpPath) }()

	resp, err := client.R().
		SetSaveResponse(true).
		SetOutputFileName(tmpPath).
		Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	if err = os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// LinkCLIBinaries 将指定二进制文件软链接到 /usr/local/bin
func LinkCLIBinaries(binPath string, binaries []string) error {
	for _, bin := range binaries {
		if _, err := shell.Execf("ln -sf '%s/%s' '/usr/local/bin/%s'", binPath, bin, bin); err != nil {
			return err
		}
	}

	return nil
}

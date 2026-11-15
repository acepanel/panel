package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v4/disk"
)

type localStorage struct {
	basePath string
}

func NewLocal(basePath string) (Storage, error) {
	if basePath == "" {
		return nil, errors.New("base path is empty")
	}
	return &localStorage{
		basePath: basePath,
	}, nil
}

// Delete 删除文件
func (l *localStorage) Delete(_ context.Context, files ...string) error {
	for _, file := range files {
		if err := os.Remove(joinPath(l.basePath, file)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// List 列出目录下的所有文件
func (l *localStorage) List(_ context.Context, path string) ([]File, error) {
	dir, err := os.Open(joinPath(l.basePath, path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()

	entries, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}
	return toFiles(entries), nil
}

// Put 写入文件内容
func (l *localStorage) Put(ctx context.Context, file string, content io.Reader) error {
	fullPath := joinPath(l.basePath, file)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	// 预检查空间
	if err := l.preCheck(filepath.Dir(fullPath), content); err != nil {
		return fmt.Errorf("pre check path failed: %w", err)
	}

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	if _, err = io.Copy(f, &ctxReader{ctx: ctx, r: content}); err != nil {
		_ = f.Close()
		return err
	}

	// 写入错误可能要到 Close 才暴露（如磁盘写满），这里不能吞
	return f.Close()
}

// Rename 改名
func (l *localStorage) Rename(_ context.Context, src, dst string) error {
	return os.Rename(joinPath(l.basePath, src), joinPath(l.basePath, dst))
}

// preCheck 内容大小已知时先看目标盘装不装得下，否则要写到磁盘满才失败，同盘的数据库等服务会先被拖垮
func (l *localStorage) preCheck(dir string, content io.Reader) error {
	file, ok := content.(interface{ Stat() (fs.FileInfo, error) })
	if !ok {
		return nil
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	usage, err := disk.Usage(dir)
	if err != nil {
		return err
	}

	if uint64(max(info.Size(), 0)) > usage.Free {
		return errors.New("insufficient backup directory space")
	}
	// btrfs 这类文件系统不报 inode 数
	if usage.InodesTotal > 0 && usage.InodesFree == 0 {
		return errors.New("insufficient backup directory inode")
	}

	return nil
}

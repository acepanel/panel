package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samber/lo"
)

type File struct {
	Name    string
	Size    int64
	ModTime time.Time
}

type Storage interface {
	// Delete deletes the given file(s).
	Delete(ctx context.Context, file ...string) error
	// List lists all files (not directories) in the given path.
	List(ctx context.Context, path string) ([]File, error)
	// Put writes the contents of a file.
	Put(ctx context.Context, file string, content io.Reader) error
}

// Renamer 由支持服务端改名的存储器实现，用于「先传临时名、成功后改名」避免中断留下能以假乱真的半截文件
// S3 不实现：分片上传在取消时会 abort 清理，对象在 complete 之前根本不存在
type Renamer interface {
	Rename(ctx context.Context, src, dst string) error
}

// ctxReader 让裸 io.Copy 响应取消：本地与 sftp 的写入都没有接收 context 的 API
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// 传入的都是存储器内的路径，base 为空时前导斜杠不能让它变成绝对路径
func joinPath(base, path string) string {
	return filepath.Join(base, strings.TrimPrefix(path, "/"))
}

func toFiles(entries []os.FileInfo) []File {
	return lo.FilterMap(entries, func(entry os.FileInfo, _ int) (File, bool) {
		return File{Name: entry.Name(), Size: entry.Size(), ModTime: entry.ModTime()}, !entry.IsDir()
	})
}

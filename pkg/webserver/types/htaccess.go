package types

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Htaccess 网站根目录下的 .htaccess，值为根目录。Apache 与 OpenLiteSpeed 的伪静态规则写在这里，
// 它是网站文件而非站点配置，程序自己也会改写
type Htaccess string

func (h Htaccess) Read() string {
	content, err := os.ReadFile(h.path())
	if err != nil {
		return ""
	}
	return string(content)
}

// Write 内容为空时删除文件；内容没变就不碰，免得弄脏纳入版本管理的文件
func (h Htaccess) Write(content string) error {
	path := h.path()
	if content == h.Read() {
		return nil
	}
	if content == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove .htaccess: %w", err)
		}
		return nil
	}

	_, statErr := os.Stat(path)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .htaccess: %w", err)
	}
	// 新建的文件跟随根目录的属主，WordPress 这类程序要能自己改写它
	if os.IsNotExist(statErr) {
		if info, err := os.Stat(string(h)); err == nil {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				_ = os.Chown(path, int(stat.Uid), int(stat.Gid))
			}
		}
	}
	return nil
}

func (h Htaccess) path() string {
	return filepath.Join(string(h), ".htaccess")
}

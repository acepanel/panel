package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
)

type WebDavConfig struct {
	URL      string        // WebDAV 服务器 URL
	Username string        // 用户名
	Password string        // 密码
	BasePath string        // 基础路径
	Timeout  time.Duration // 连接超时时间
}

type webDavStorage struct {
	client *gowebdav.Client
	config WebDavConfig
}

func NewWebDav(config WebDavConfig) (Storage, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	config.BasePath = strings.TrimSuffix(config.BasePath, "/")

	client := gowebdav.NewClient(config.URL, config.Username, config.Password)
	client.SetTimeout(config.Timeout)
	// 上传要关掉整体超时（大文件耗时不可预估），改由传输层兜底，避免对端黑洞时永久阻塞
	client.SetTransport(newTransport())

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to WebDAV server: %w", err)
	}

	w := &webDavStorage{
		client: client,
		config: config,
	}

	if w.config.BasePath != "" {
		if err := w.client.MkdirAll(w.config.BasePath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create base path: %w", err)
		}
	}

	return w, nil
}

// Delete 删除文件
func (w *webDavStorage) Delete(_ context.Context, files ...string) error {
	for _, file := range files {
		remotePath := joinPath(w.config.BasePath, file)
		if err := w.client.Remove(remotePath); err != nil {
			return err
		}
	}
	return nil
}

// List 列出目录下的所有文件
func (w *webDavStorage) List(_ context.Context, path string) ([]File, error) {
	entries, err := w.client.ReadDir(joinPath(w.config.BasePath, path))
	if err != nil {
		return nil, err
	}
	return toFiles(entries), nil
}

// Put 写入文件内容
// gowebdav 没有接收 context 的 API，上传无法被取消；且 content 必须保持 io.Seeker，
// 否则 WriteStream 会把整个文件读进内存来算 Content-Length
func (w *webDavStorage) Put(_ context.Context, file string, content io.Reader) error {
	remotePath := joinPath(w.config.BasePath, file)

	// 确保目录存在
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." {
		if err := w.client.MkdirAll(remoteDir, 0755); err != nil {
			return err
		}
	}

	// 调整超时
	w.client.SetTimeout(0)
	defer w.client.SetTimeout(w.config.Timeout)

	return w.client.WriteStream(remotePath, content, 0644)
}

// Rename 改名
func (w *webDavStorage) Rename(_ context.Context, src, dst string) error {
	return w.client.Rename(joinPath(w.config.BasePath, src), joinPath(w.config.BasePath, dst), true)
}

// newTransport 构造带各阶段超时的传输层
// 不设 http.Client.Timeout：上传耗时随文件大小变化，只能靠分阶段超时兜住对端黑洞
func newTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: time.Minute,
	}
}

package websitestat

import (
	"fmt"
	"log/slog"
	"net"
	"os"
)

// Listener 通过 Unix Datagram Socket 监听 nginx syslog 消息
type Listener struct {
	path string
	conn *net.UnixConn
	log  *slog.Logger
	buf  []byte // 读缓冲区，在 readLoop 单 goroutine 中复用
}

// NewListener 创建并绑定 Unix Datagram Socket 监听器
func NewListener(path string, log *slog.Logger) (*Listener, error) {
	// 清理可能残留的旧 socket 文件
	_ = os.Remove(path)

	addr := &net.UnixAddr{Name: path, Net: "unixgram"}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on unix socket %s: %w", path, err)
	}

	// 确保 nginx 用户可写
	if err = os.Chmod(path, 0666); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to chmod socket: %w", err)
	}

	return &Listener{
		path: path,
		conn: conn,
		log:  log,
		buf:  make([]byte, 256*1024),
	}, nil
}

// Read 读取一条 syslog 消息，返回 tag 和 JSON 数据
func (l *Listener) Read() (string, []byte, error) {
	n, err := l.conn.Read(l.buf)
	if err != nil {
		return "", nil, err
	}

	tag, data := parseSyslog(l.buf[:n])
	return tag, data, nil
}

// Close 关闭监听器并清理 socket 文件
func (l *Listener) Close() error {
	err := l.conn.Close()
	_ = os.Remove(l.path)
	return err
}

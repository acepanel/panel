package storage

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

// 对端接受连接后一直不发 SSH 握手数据，取消必须能把握手踹醒
func TestSFTPCancelDuringHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	must.NoError(t, err)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		<-stop
		_ = conn.Close()
	}()
	defer func() { _ = listener.Close() }()

	_, portText, err := net.SplitHostPort(listener.Addr().String())
	must.NoError(t, err)
	port, err := strconv.Atoi(portText)
	must.NoError(t, err)
	client, err := NewSFTP(SFTPConfig{Host: "127.0.0.1", Port: port, Username: "user", Password: "pass"})
	must.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, listErr := client.List(ctx, "")
		done <- listErr
	}()

	select {
	case err = <-done:
		check.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("握手阶段没有响应取消")
	}
}

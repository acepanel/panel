package docker

import (
	"context"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/moby/moby/client"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// Turn 容器终端转发器
type Turn struct {
	ctx    context.Context
	ws     *websocket.Conn
	client *client.Client
	execID string
	hijack client.ExecAttachResult
}

// NewTurn 创建容器终端转发器
func NewTurn(
	ctx context.Context,
	ws *websocket.Conn,
	containerID string,
	command []string,
	sock string,
) (*Turn, error) {
	apiClient, err := client.New(client.WithHost(sock))
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// 创建 exec 实例
	execCreateResp, err := apiClient.ExecCreate(ctx, containerID, client.ExecCreateOptions{
		Cmd:          command,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		TTY:          true,
	})
	if err != nil {
		_ = apiClient.Close()
		return nil, fmt.Errorf("failed to create exec instance: %w", err)
	}

	// 附加到 exec 实例
	hijack, err := apiClient.ExecAttach(ctx, execCreateResp.ID, client.ExecAttachOptions{
		TTY: true,
	})
	if err != nil {
		_ = apiClient.Close()
		return nil, fmt.Errorf("failed to attach to exec instance: %w", err)
	}

	turn := &Turn{
		ctx:    ctx,
		ws:     ws,
		client: apiClient,
		execID: execCreateResp.ID,
		hijack: hijack,
	}

	return turn, nil
}

// Write 实现 io.Writer 接口，将容器输出写入 WebSocket
func (t *Turn) Write(p []byte) (n int, err error) {
	if err = t.ws.Write(t.ctx, websocket.MessageBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close 关闭连接
func (t *Turn) Close() {
	t.hijack.Close()
	_ = t.hijack.CloseWrite()

	ctx := context.Background()
	resp, err := t.client.ExecInspect(ctx, t.execID, client.ExecInspectOptions{})
	if err != nil || !resp.Running || resp.PID <= 0 {
		_ = t.client.Close()
		return
	}

	_ = syscall.Kill(resp.PID, syscall.SIGTERM)
	// 3 秒后若仍未退出再强制杀死
	pid := resp.PID
	time.AfterFunc(3*time.Second, func() {
		if r, e := t.client.ExecInspect(ctx, t.execID, client.ExecInspectOptions{}); e == nil && r.Running {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
		_ = t.client.Close()
	})
}

// Handle 处理 WebSocket 消息
func (t *Turn) Handle(ctx context.Context) error {
	return shell.RelayInput(ctx, t.ws, t.hijack.Conn, func(rows, cols uint16) error {
		_, err := t.client.ExecResize(ctx, t.execID, client.ExecResizeOptions{
			Height: uint(rows),
			Width:  uint(cols),
		})
		return err
	})
}

// Wait 等待容器输出并转发到 WebSocket
func (t *Turn) Wait() {
	_, _ = io.Copy(t, t.hijack.Reader)
}

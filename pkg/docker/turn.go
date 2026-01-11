package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/moby/moby/client"
)

// MessageResize 终端大小调整消息
type MessageResize struct {
	Resize  bool `json:"resize"`
	Columns uint `json:"columns"`
	Rows    uint `json:"rows"`
}

// Turn 容器终端转发器
type Turn struct {
	ctx    context.Context
	ws     *websocket.Conn
	client *client.Client
	execID string
	hijack client.ExecAttachResult
}

// NewTurn 创建容器终端转发器
func NewTurn(ctx context.Context, ws *websocket.Conn, containerID string, command []string) (*Turn, error) {
	apiClient, err := client.New(client.WithHost("unix:///var/run/docker.sock"))
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
	if err = t.ws.Write(t.ctx, websocket.MessageText, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close 关闭连接
func (t *Turn) Close() {
	// 检查进程是否仍在运行
	inspectResp, err := t.client.ExecInspect(t.ctx, t.execID, client.ExecInspectOptions{})
	if err == nil && inspectResp.Running {
		_ = syscall.Kill(inspectResp.PID, syscall.SIGTERM)
		// 等待最多 10 秒
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			inspectResp, err = t.client.ExecInspect(t.ctx, t.execID, client.ExecInspectOptions{})
			if err != nil || !inspectResp.Running {
				break
			}
		}
		// 如果仍在运行，KILL
		if err == nil && inspectResp.Running {
			_ = syscall.Kill(inspectResp.PID, syscall.SIGKILL)
		}
	}

	t.hijack.Close()
	_ = t.hijack.CloseWrite()
	_ = t.client.Close()
}

// Handle 处理 WebSocket 消息
func (t *Turn) Handle(ctx context.Context) error {
	var resize MessageResize

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			_, data, err := t.ws.Read(ctx)
			if err != nil {
				// 通常是客户端关闭连接
				return fmt.Errorf("failed to read ws message: %w", err)
			}

			// 判断是否是 resize 消息
			if err = json.Unmarshal(data, &resize); err == nil {
				if resize.Resize && resize.Columns > 0 && resize.Rows > 0 {
					if _, err = t.client.ExecResize(ctx, t.execID, client.ExecResizeOptions{
						Height: resize.Rows,
						Width:  resize.Columns,
					}); err != nil {
						return fmt.Errorf("failed to resize terminal: %w", err)
					}
				}
				continue
			}

			if _, err = t.hijack.Conn.Write(data); err != nil {
				return fmt.Errorf("failed to write to container stdin: %w", err)
			}
		}
	}
}

// Wait 等待容器输出并转发到 WebSocket
func (t *Turn) Wait() {
	_, _ = io.Copy(t, t.hijack.Reader)
}

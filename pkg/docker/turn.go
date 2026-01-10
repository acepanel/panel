package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

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
	ctx       context.Context
	ws        *websocket.Conn
	client    *client.Client
	execID    string
	hijack    client.ExecAttachResult
	closeOnce bool
}

// NewTurn 创建容器终端转发器
func NewTurn(ctx context.Context, ws *websocket.Conn, containerID string, command []string) (*Turn, error) {
	apiClient, err := client.NewClientWithOpts(
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 Docker 客户端失败: %w", err)
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
		return nil, fmt.Errorf("创建 exec 实例失败: %w", err)
	}

	// 附加到 exec 实例
	hijack, err := apiClient.ExecAttach(ctx, execCreateResp.ID, client.ExecAttachOptions{
		TTY: true,
	})
	if err != nil {
		_ = apiClient.Close()
		return nil, fmt.Errorf("附加到 exec 实例失败: %w", err)
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
	if t.closeOnce {
		return
	}
	t.closeOnce = true
	t.hijack.Close()
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
				return fmt.Errorf("读取 ws 消息错误: %w", err)
			}

			// 判断是否是 resize 消息
			if err = json.Unmarshal(data, &resize); err == nil {
				if resize.Resize && resize.Columns > 0 && resize.Rows > 0 {
					if _, err = t.client.ExecResize(ctx, t.execID, client.ExecResizeOptions{
						Height: resize.Rows,
						Width:  resize.Columns,
					}); err != nil {
						return fmt.Errorf("调整终端大小错误: %w", err)
					}
				}
				continue
			}

			if _, err = t.hijack.Conn.Write(data); err != nil {
				return fmt.Errorf("写入容器 stdin 错误: %w", err)
			}
		}
	}
}

// Wait 等待容器输出并转发到 WebSocket
func (t *Turn) Wait() {
	_, _ = io.Copy(t, t.hijack.Reader)
}

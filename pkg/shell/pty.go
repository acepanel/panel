package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

// MessageResize 终端大小调整消息
type MessageResize struct {
	Resize  bool `json:"resize"`
	Columns uint `json:"columns"`
	Rows    uint `json:"rows"`
}

// Turn PTY 终端
type Turn struct {
	ctx  context.Context
	ws   *websocket.Conn
	ptmx *os.File
	cmd  *exec.Cmd
}

// NewPTYTurn 使用 PTY 执行命令，返回 Turn 用于流式读取输出
// 调用方需要负责调用 Close() 和 Wait()
func NewPTYTurn(ctx context.Context, ws *websocket.Conn, shell string, args ...any) (*Turn, error) {
	if !preCheckArg(args) {
		return nil, errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.CommandContext(ctx, "bash", "-c", shell)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	return &Turn{
		ctx:  ctx,
		ws:   ws,
		ptmx: ptmx,
		cmd:  cmd,
	}, nil
}

// Write 写入 PTY 输入
func (t *Turn) Write(data []byte) (int, error) {
	return t.ptmx.Write(data)
}

// Wait 等待命令完成
func (t *Turn) Wait() {
	_ = t.cmd.Wait()
}

// Close 关闭 PTY 并终止子进程
func (t *Turn) Close() {
	// 先向子进程发送 SIGTERM 信号以终止它
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Signal(syscall.SIGTERM)
	}
	_ = t.ptmx.Close()
}

// Handle 从 WebSocket 读取输入写入 PTY
func (t *Turn) Handle(ctx context.Context) error {
	var resize MessageResize

	go func() { _ = t.Pipe(ctx) }()

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
					if err = t.Resize(uint16(resize.Rows), uint16(resize.Columns)); err != nil {
						return fmt.Errorf("failed to resize terminal: %w", err)
					}
				}
				continue
			}

			if _, err = t.Write(data); err != nil {
				return fmt.Errorf("failed to write to pty stdin: %w", err)
			}
		}
	}
}

// Pipe 从 PTY 读取输出写入 WebSocket
func (t *Turn) Pipe(ctx context.Context) error {
	buf := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := t.ptmx.Read(buf)
			if err != nil {
				if err = IsPTYError(err); err != nil {
					return fmt.Errorf("failed to read from pty: %w", err)
				}
				return nil
			}
			if n > 0 {
				if err = t.ws.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
					return fmt.Errorf("failed to write to ws: %w", err)
				}
			}
		}
	}
}

// Resize 调整 PTY 窗口大小
func (t *Turn) Resize(rows, cols uint16) error {
	return pty.Setsize(t.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// IsPTYError Linux kernel return EIO when attempting to read from a master pseudo
// terminal which no longer has an open slave. So ignore error here.
// See https://github.com/creack/pty/issues/21
func IsPTYError(err error) error {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || !errors.Is(pathErr.Err, syscall.EIO) || !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

// Turn PTY 终端
type Turn struct {
	ws   *websocket.Conn
	ptmx *os.File
	cmd  *exec.Cmd
	kill atomic.Pointer[time.Timer] // Close 设的强杀定时器，进程退出后由 Wait 停掉
}

// NewPTYTurn 使用 PTY 执行命令，返回 Turn 用于流式读取输出
// 调用方需要负责调用 Close() 和 Wait()
func NewPTYTurn(ctx context.Context, ws *websocket.Conn, shell string) (*Turn, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", shell)
	applyEnv(cmd)
	// 服务环境没有 TERM 会导致无法清屏，LC_ALL=C 会把中文显示成转义，交互终端都要覆盖
	cmd.Env = append(cmd.Env, "TERM=xterm-256color", "LC_ALL=C.UTF-8")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	return &Turn{
		ws:   ws,
		ptmx: ptmx,
		cmd:  cmd,
	}, nil
}

// Write 写入 PTY 输入
func (t *Turn) Write(data []byte) (int, error) {
	return t.ptmx.Write(data)
}

// Close 关闭 PTY 并终止子进程
func (t *Turn) Close() {
	// 先发 SIGTERM 优雅终止，3 秒后仍未退出再强制杀死
	// pty.Start 建了新会话，信号发给整个进程组，命令自己派生的子进程才不会残留
	pgid := -t.cmd.Process.Pid
	_ = syscall.Kill(pgid, syscall.SIGTERM)
	t.kill.Store(time.AfterFunc(3*time.Second, func() {
		_ = syscall.Kill(pgid, syscall.SIGKILL)
	}))
	_ = t.ptmx.Close()
}

// Handle 从 WebSocket 读取输入写入 PTY
func (t *Turn) Handle(ctx context.Context) error {
	go func() { _ = t.Pipe(ctx) }()

	return RelayInput(ctx, t.ws, t, t.Resize)
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
				if err = isPTYError(err); err != nil {
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

// Wait 等待命令完成
func (t *Turn) Wait() {
	_ = t.cmd.Wait()
	// 进程已回收，不必再等定时器到点去杀一个已经不存在的进程
	if timer := t.kill.Swap(nil); timer != nil {
		timer.Stop()
	}
}

// isPTYError Linux kernel return EIO when attempting to read from a master pseudo
// terminal which no longer has an open slave. So ignore error here.
// See https://github.com/creack/pty/issues/21
func isPTYError(err error) error {
	if errors.Is(err, syscall.EIO) || errors.Is(err, io.EOF) {
		return nil
	}

	return err
}

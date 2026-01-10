package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// Exec 执行 shell 命令
func Exec(shell string) (string, error) {
	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.Command("bash", "-c", shell)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// Execf 安全执行 shell 命令
func Execf(shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.Command("bash", "-c", shell)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecfAsync 异步执行 shell 命令
func ExecfAsync(shell string, args ...any) error {
	if !preCheckArg(args) {
		return errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.Command("bash", "-c", shell)

	err := cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		if err = cmd.Wait(); err != nil {
			fmt.Println(fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(err.Error())))
		}
	}()

	return nil
}

// ExecfWithTimeout 执行 shell 命令并设置超时时间
func ExecfWithTimeout(timeout time.Duration, shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.Command("bash", "-c", shell)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(stderr.String()))
	}

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: %s", shell, "timeout")
	case err = <-done:
		if err != nil {
			return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(stderr.String()))
		}
	}

	return strings.TrimSpace(stdout.String()), err
}

// ExecfWithOutput 执行 shell 命令并输出到终端
func ExecfWithOutput(shell string, args ...any) error {
	if !preCheckArg(args) {
		return errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.Command("bash", "-c", shell)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ExecfWithPipe 执行 shell 命令并返回管道
func ExecfWithPipe(ctx context.Context, shell string, args ...any) (io.ReadCloser, error) {
	if !preCheckArg(args) {
		return nil, errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.CommandContext(ctx, "bash", "-c", shell)

	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	cmd.Stderr = cmd.Stdout
	err = cmd.Start()

	go func() { _ = cmd.Wait() }()

	return out, err
}

// ExecfWithDir 在指定目录下执行 shell 命令
func ExecfWithDir(dir, shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.Command("bash", "-c", shell)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// ExecfWithTTY 在伪终端下执行 shell 命令
func ExecfWithTTY(shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.Command("bash", "-i", "-c", shell)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr // https://github.com/creack/pty/issues/147 取 stderr

	f, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("run %s failed", shell)
	}
	defer func(f *os.File) { _ = f.Close() }(f)

	if _, err = io.Copy(&out, f); ptyError(err) != nil {
		return "", fmt.Errorf("run %s failed, out: %s, err: %w", shell, strings.TrimSpace(out.String()), err)
	}
	if stderr.Len() > 0 {
		return "", fmt.Errorf("run %s failed, out: %s", shell, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(out.String()), nil
}

func preCheckArg(args []any) bool {
	illegals := []any{`&`, `|`, `;`, `$`, `'`, `"`, "`", `(`, `)`, "\n", "\r", `>`, `<`}
	for arg := range slices.Values(args) {
		if slices.Contains(illegals, arg) {
			return false
		}
	}

	return true
}

// PTYResult 封装 PTY 执行结果
type PTYResult struct {
	ptmx *os.File
	cmd  *exec.Cmd
}

// Read 读取 PTY 输出
func (p *PTYResult) Read(buf []byte) (int, error) {
	return p.ptmx.Read(buf)
}

// Wait 等待命令完成
func (p *PTYResult) Wait() error {
	return p.cmd.Wait()
}

// Close 关闭 PTY
func (p *PTYResult) Close() error {
	return p.ptmx.Close()
}

// ExecWithPTY 使用 PTY 执行命令，返回 PTYResult 用于流式读取输出
// 调用方需要负责调用 Close() 和 Wait()
func ExecWithPTY(ctx context.Context, name string, args ...string) (*PTYResult, error) {
	_ = os.Setenv("LC_ALL", "C")
	cmd := exec.CommandContext(ctx, name, args...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	return &PTYResult{
		ptmx: ptmx,
		cmd:  cmd,
	}, nil
}

// IsPTYError 检查 PTY 错误是否可忽略
// Linux kernel return EIO when attempting to read from a master pseudo
// terminal which no longer has an open slave. So ignore error here.
// See https://github.com/creack/pty/issues/21
func IsPTYError(err error) bool {
	if err == nil {
		return false
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, syscall.EIO) {
		return false // 可忽略的错误
	}
	if errors.Is(err, io.EOF) {
		return false // 可忽略的错误
	}
	return true // 真正的错误
}

// Linux kernel return EIO when attempting to read from a master pseudo
// terminal which no longer has an open slave. So ignore error here.
// See https://github.com/creack/pty/issues/21
func ptyError(err error) error {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || !errors.Is(pathErr.Err, syscall.EIO) {
		return err
	}

	return nil
}

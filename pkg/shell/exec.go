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
	return ExecWithOptions(shell)
}

// Execf 安全执行 shell 命令，支持格式化参数
func Execf(shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	return ExecWithOptions(shell)
}

// ExecfAsync 异步执行 shell 命令
func ExecfAsync(shell string, args ...any) error {
	if !preCheckArg(args) {
		return errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_, err := ExecWithOptions(shell, WithAsync())
	return err
}

// ExecfWithTimeout 执行 shell 命令并设置超时时间
func ExecfWithTimeout(timeout time.Duration, shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	return ExecWithOptions(shell, WithTimeout(timeout))
}

// ExecfWithOutput 执行 shell 命令并输出到终端
func ExecfWithOutput(shell string, args ...any) error {
	if !preCheckArg(args) {
		return errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	_, err := ExecWithOptions(shell, WithInheritOutput())
	return err
}

// ExecfWithPipe 执行 shell 命令并返回管道
func ExecfWithPipe(ctx context.Context, shell string, args ...any) (io.ReadCloser, error) {
	if !preCheckArg(args) {
		return nil, errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	return ExecPipe(ctx, shell)
}

// ExecfWithDir 在指定目录下执行 shell 命令
func ExecfWithDir(dir, shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	return ExecWithOptions(shell, WithDir(dir))
}

// ExecfWithTTY 在伪终端下执行 shell 命令
func ExecfWithTTY(shell string, args ...any) (string, error) {
	if !preCheckArg(args) {
		return "", errors.New("command contains illegal characters")
	}
	if len(args) > 0 {
		shell = fmt.Sprintf(shell, args...)
	}

	return ExecWithOptions(shell, WithTTY())
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

// ============================================================================
// 新架构：基于选项模式的统一命令执行框架
// ============================================================================

// outputMode 定义命令输出模式
type outputMode int

const (
	OutputCapture outputMode = iota // 捕获输出（默认）
	OutputInherit                    // 继承父进程输出
	OutputPipe                       // 管道模式
)

// execConfig 命令执行配置
type execConfig struct {
	timeout    time.Duration
	dir        string
	async      bool
	useTTY     bool
	outputMode outputMode
	stdout     io.Writer
	stderr     io.Writer
	bashFlags  []string
	env        map[string]string
	ctx        context.Context
}

// ExecOption 命令执行选项函数类型
type ExecOption func(*execConfig)

// WithTimeout 设置超时时间
func WithTimeout(d time.Duration) ExecOption {
	return func(c *execConfig) {
		c.timeout = d
	}
}

// WithDir 设置工作目录
func WithDir(dir string) ExecOption {
	return func(c *execConfig) {
		c.dir = dir
	}
}

// WithContext 设置上下文（用于取消控制）
func WithContext(ctx context.Context) ExecOption {
	return func(c *execConfig) {
		c.ctx = ctx
	}
}

// WithTTY 启用伪终端模式
func WithTTY() ExecOption {
	return func(c *execConfig) {
		c.useTTY = true
		c.bashFlags = []string{"-i", "-c"}
	}
}

// WithInheritOutput 继承父进程输出（实时打印到终端）
func WithInheritOutput() ExecOption {
	return func(c *execConfig) {
		c.outputMode = OutputInherit
		c.stdout = os.Stdout
		c.stderr = os.Stderr
	}
}

// WithPipeOutput 启用管道模式（内部使用）
func WithPipeOutput() ExecOption {
	return func(c *execConfig) {
		c.outputMode = OutputPipe
	}
}

// WithEnv 设置环境变量
func WithEnv(key, value string) ExecOption {
	return func(c *execConfig) {
		if c.env == nil {
			c.env = make(map[string]string)
		}
		c.env[key] = value
	}
}

// WithAsync 启用异步执行
func WithAsync() ExecOption {
	return func(c *execConfig) {
		c.async = true
	}
}

// ============================================================================
// 内部辅助函数
// ============================================================================

// createCommand 创建并配置 exec.Cmd 对象
func createCommand(ctx context.Context, shell string, cfg *execConfig) *exec.Cmd {
	args := append(cfg.bashFlags, shell)
	cmd := exec.CommandContext(ctx, "bash", args...)

	if cfg.dir != "" {
		cmd.Dir = cfg.dir
	}

	for k, v := range cfg.env {
		_ = os.Setenv(k, v)
	}

	return cmd
}

// execSync 同步执行命令
func execSync(shell string, cfg *execConfig) (string, error) {
	cmd := createCommand(cfg.ctx, shell, cfg)

	var stdout, stderr bytes.Buffer
	if cfg.outputMode == OutputInherit {
		cmd.Stdout = cfg.stdout
		cmd.Stderr = cfg.stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	if cfg.timeout > 0 {
		return execWithTimeout(cmd, &stdout, &stderr, shell, cfg.timeout)
	}

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()),
			fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// execAsync 异步执行命令
func execAsync(shell string, cfg *execConfig) error {
	cmd := createCommand(cfg.ctx, shell, cfg)

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Println(fmt.Errorf("run %s failed, err: %s", shell, err.Error()))
		}
	}()

	return nil
}

// execWithTimeout 带超时的命令执行
func execWithTimeout(cmd *exec.Cmd, stdout, stderr *bytes.Buffer, shell string, timeout time.Duration) (string, error) {
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("run %s failed, err: %s", shell, err.Error())
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: timeout", shell)
	case err := <-done:
		if err != nil {
			return strings.TrimSpace(stdout.String()),
				fmt.Errorf("run %s failed, err: %s", shell, strings.TrimSpace(stderr.String()))
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// execWithTTY 在伪终端模式下执行命令
func execWithTTY(shell string, cfg *execConfig) (string, error) {
	cmd := createCommand(cfg.ctx, shell, cfg)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr // https://github.com/creack/pty/issues/147

	f, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("run %s failed", shell)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	if _, err = io.Copy(&out, f); ptyError(err) != nil {
		return "", fmt.Errorf("run %s failed, out: %s, err: %w", shell, strings.TrimSpace(out.String()), err)
	}
	if stderr.Len() > 0 {
		return "", fmt.Errorf("run %s failed, out: %s", shell, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(out.String()), nil
}

// ============================================================================
// 公开 API：新架构的统一入口函数
// ============================================================================

// ExecWithOptions 使用选项模式执行 shell 命令
//
// 支持的选项：WithTimeout, WithDir, WithContext, WithTTY, WithInheritOutput, WithEnv, WithAsync
//
// 示例：
//
//	// 基本用法
//	output, err := ExecWithOptions("ls -la")
//
//	// 组合多个选项
//	output, err := ExecWithOptions(
//	    "make build",
//	    WithTimeout(30*time.Second),
//	    WithDir("/path/to/project"),
//	    WithEnv("DEBUG", "1"),
//	)
func ExecWithOptions(shell string, opts ...ExecOption) (string, error) {
	// 应用默认配置
	cfg := &execConfig{
		bashFlags:  []string{"-c"},
		outputMode: OutputCapture,
		env:        map[string]string{"LC_ALL": "C"},
		ctx:        context.Background(),
	}

	// 应用用户选项
	for _, opt := range opts {
		opt(cfg)
	}

	// 管道模式不支持
	if cfg.outputMode == OutputPipe {
		return "", errors.New("pipe output mode is not supported, use ExecPipe instead")
	}

	// TTY 模式
	if cfg.useTTY {
		return execWithTTY(shell, cfg)
	}

	// 异步执行
	if cfg.async {
		return "", execAsync(shell, cfg)
	}

	// 同步执行
	return execSync(shell, cfg)
}

// ExecPipe 以管道模式执行 shell 命令，返回输出流
//
// 支持的选项：WithDir, WithEnv（超时请使用 context.WithTimeout）
//
// 示例：
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	pipe, err := ExecPipe(ctx, "find / -name '*.log'")
//	if err != nil {
//	    return err
//	}
//	defer pipe.Close()
//
//	scanner := bufio.NewScanner(pipe)
//	for scanner.Scan() {
//	    fmt.Println(scanner.Text())
//	}
func ExecPipe(ctx context.Context, shell string, opts ...ExecOption) (io.ReadCloser, error) {
	// 应用默认配置
	cfg := &execConfig{
		bashFlags:  []string{"-c"},
		env:        map[string]string{"LC_ALL": "C"},
		ctx:        ctx,
		outputMode: OutputPipe,
	}

	// 应用用户选项
	for _, opt := range opts {
		opt(cfg)
	}

	// 不支持的选项检查
	if cfg.useTTY {
		return nil, errors.New("pipe mode does not support TTY")
	}
	if cfg.async {
		return nil, errors.New("pipe mode does not support async execution")
	}
	if cfg.outputMode == OutputInherit {
		return nil, errors.New("pipe mode does not support inherit output")
	}

	// 创建命令
	cmd := createCommand(cfg.ctx, shell, cfg)

	// 创建输出管道
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// stderr 重定向到 stdout
	cmd.Stderr = cmd.Stdout

	// 启动命令
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return out, nil
}

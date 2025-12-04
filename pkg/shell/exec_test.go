package shell

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestExec 测试基本执行功能
func TestExec(t *testing.T) {
	output, err := Exec("echo 'hello world'")
	if err != nil {
		t.Fatalf("Exec 失败: %v", err)
	}
	if output != "hello world" {
		t.Errorf("期望输出 'hello world'，实际输出 '%s'", output)
	}
}

// TestExecf 测试格式化参数执行
func TestExecf(t *testing.T) {
	output, err := Execf("echo '%s'", "test")
	if err != nil {
		t.Fatalf("Execf 失败: %v", err)
	}
	if output != "test" {
		t.Errorf("期望输出 'test'，实际输出 '%s'", output)
	}
}

// TestExecfWithTimeout 测试超时功能
func TestExecfWithTimeout(t *testing.T) {
	// 测试正常执行
	output, err := ExecfWithTimeout(2*time.Second, "echo '%s'", "timeout test")
	if err != nil {
		t.Fatalf("ExecfWithTimeout 失败: %v", err)
	}
	if output != "timeout test" {
		t.Errorf("期望输出 'timeout test'，实际输出 '%s'", output)
	}

	// 测试超时
	_, err = ExecfWithTimeout(1*time.Second, "sleep 3")
	if err == nil {
		t.Error("期望超时错误，但没有返回错误")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("期望包含 'timeout' 的错误消息，实际: %v", err)
	}
}

// TestExecfWithDir 测试工作目录设置
func TestExecfWithDir(t *testing.T) {
	output, err := ExecfWithDir("/tmp", "pwd")
	if err != nil {
		t.Fatalf("ExecfWithDir 失败: %v", err)
	}
	if output != "/tmp" && output != "/private/tmp" { // macOS 上 /tmp 是 /private/tmp 的软链接
		t.Errorf("期望输出 '/tmp' 或 '/private/tmp'，实际输出 '%s'", output)
	}
}

// TestExecWithOptions 测试新的选项模式
func TestExecWithOptions(t *testing.T) {
	// 基本使用
	output, err := ExecWithOptions("echo 'options test'")
	if err != nil {
		t.Fatalf("ExecWithOptions 失败: %v", err)
	}
	if output != "options test" {
		t.Errorf("期望输出 'options test'，实际输出 '%s'", output)
	}

	// 带超时
	output, err = ExecWithOptions("echo 'with timeout'", WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("ExecWithOptions WithTimeout 失败: %v", err)
	}
	if output != "with timeout" {
		t.Errorf("期望输出 'with timeout'，实际输出 '%s'", output)
	}

	// 带工作目录
	output, err = ExecWithOptions("pwd", WithDir("/tmp"))
	if err != nil {
		t.Fatalf("ExecWithOptions WithDir 失败: %v", err)
	}
	if output != "/tmp" && output != "/private/tmp" {
		t.Errorf("期望输出 '/tmp' 或 '/private/tmp'，实际输出 '%s'", output)
	}

	// 组合多个选项
	output, err = ExecWithOptions(
		"pwd",
		WithTimeout(5*time.Second),
		WithDir("/tmp"),
	)
	if err != nil {
		t.Fatalf("ExecWithOptions 组合选项失败: %v", err)
	}
	if output != "/tmp" && output != "/private/tmp" {
		t.Errorf("期望输出 '/tmp' 或 '/private/tmp'，实际输出 '%s'", output)
	}
}

// TestExecPipe 测试管道模式
func TestExecPipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipe, err := ExecPipe(ctx, "echo 'line1\nline2\nline3'")
	if err != nil {
		t.Fatalf("ExecPipe 失败: %v", err)
	}
	defer pipe.Close()

	// 读取输出
	buf := make([]byte, 1024)
	n, err := pipe.Read(buf)
	if err != nil {
		t.Fatalf("读取管道失败: %v", err)
	}

	output := strings.TrimSpace(string(buf[:n]))
	if !strings.Contains(output, "line1") {
		t.Errorf("输出应包含 'line1'，实际输出: '%s'", output)
	}
}

// TestPreCheckArg 测试参数安全检查
func TestPreCheckArg(t *testing.T) {
	// 测试合法参数
	if !preCheckArg([]any{"test", "123", "abc"}) {
		t.Error("合法参数应通过检查")
	}

	// 测试非法参数
	illegals := []any{`&`, `|`, `;`, `$`, `'`, `"`, "`", `(`, `)`, "\n", "\r", `>`, `<`}
	for _, illegal := range illegals {
		if preCheckArg([]any{illegal}) {
			t.Errorf("非法字符 %v 应该被拒绝", illegal)
		}
	}
}

// TestExecfAsync 测试异步执行
func TestExecfAsync(t *testing.T) {
	err := ExecfAsync("sleep 1")
	if err != nil {
		t.Fatalf("ExecfAsync 失败: %v", err)
	}

	// 异步执行应该立即返回
	// 这里只测试函数是否能正常启动，不等待结果
}
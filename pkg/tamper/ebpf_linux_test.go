//go:build linux

package tamper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCString(t *testing.T) {
	buf := make([]byte, 32)
	copy(buf, "long-old-name.php\x00")
	copy(buf, "a.php\x00")
	if got := cString(buf); got != "a.php" {
		t.Fatalf("cString() = %q, want %q", got, "a.php")
	}
	if got := cString([]byte("nozero")); got != "nozero" {
		t.Fatalf("无 NUL 应原样返回,实得 %q", got)
	}
	if got := cString(nil); got != "" {
		t.Fatalf("空输入应返回空串,实得 %q", got)
	}
}

// waitEvent 在超时前等待符合条件的事件
func waitEvent(ch <-chan Event, want func(Event) bool) *Event {
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-ch:
			if want(ev) {
				return &ev
			}
		case <-deadline:
			return nil
		}
	}
}

func requireEBPF(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("需要 root")
	}
	if st := DetectEBPF(); !st.Available {
		t.Skipf("eBPF 不可用: %s (lsm=%s)", st.Reason, st.ActiveLSM)
	}
}

// TestEBPFVerifierLoad 在当前内核上加载全部 LSM 程序,验证手写汇编能过 verifier
// 内核未激活 bpf LSM 时 attach 失败但 load 已完成,同样视为通过
func TestEBPFVerifierLoad(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("需要 root")
	}
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err != nil {
		t.Skip("内核无 BTF")
	}

	for _, tc := range []struct {
		name     string
		blockNew bool
		exts     []string
	}{
		{"block_wholetree", true, nil},
		{"block_exts", true, []string{"php", "SH", ".Html", "jsp", "phtml", "so.1"}},
		{"observe_exts", false, []string{"php"}},
		{"observe_wholetree", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := newEBPFEngine(newLocale(), newLogger(), tc.blockNew, tc.exts)
			if err == nil {
				_ = e.Close()
				return
			}
			msg := err.Error()
			switch {
			case strings.Contains(msg, "btf func bpf_lsm_"):
				t.Skipf("内核未编译 BPF LSM: %v", err)
			case strings.Contains(msg, "failed to attach"):
				t.Logf("verifier 通过,attach 不可用(内核未激活 bpf LSM): %v", err)
			default:
				t.Fatalf("加载失败: %v", err)
			}
		})
	}
}

// TestEBPFCreateBlock 拦截模式:新建在内核被拒并携带进程信息
func TestEBPFCreateBlock(t *testing.T) {
	requireEBPF(t)

	t.Run("exts", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "index.php"), "x")

		m, err := NewManager(Config{
			Mode: ModeEBPF, BlockNewFiles: true,
			Rules: []Rule{{Name: "t", Paths: []string{dir}, Exts: []string{"php"}}},
		}, newLocale(), newLogger())
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = m.Stop() }()

		evil := filepath.Join(dir, "evil.php")
		if err := os.WriteFile(evil, []byte("x"), 0644); err == nil {
			t.Fatal("新建 .php 应被拒")
		}
		if err := os.WriteFile(filepath.Join(dir, "EVIL2.PHP"), []byte("x"), 0644); err == nil {
			t.Fatal("大写后缀新建应被拒")
		}
		if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("x"), 0644); err != nil {
			t.Fatalf("非匹配后缀应放行: %v", err)
		}
		ev := waitEvent(m.Events(), func(e Event) bool { return e.Op == OpCreate && e.Path == evil })
		if ev == nil {
			t.Fatal("应收到新建拦截事件")
		}
		if ev.PID == 0 || ev.Comm == "" || !ev.Denied {
			t.Fatalf("事件应携带进程信息且标记已拒绝: %+v", ev)
		}
		t.Logf("拦截事件: path=%s comm=%s pid=%d", ev.Path, ev.Comm, ev.PID)

		// 外部文件移入同样被拒
		outside := filepath.Join(t.TempDir(), "outside.php")
		writeFile(t, outside, "x")
		if err := os.Rename(outside, filepath.Join(dir, "in.php")); err == nil {
			t.Fatal("移入 .php 应被拒")
		}

		// 严格模式下整目录移入直接拒绝(消除"放行后异步补扫"竞态窗口)
		outDir := filepath.Join(t.TempDir(), "pack")
		if err := os.Mkdir(outDir, 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(outDir, "inner.php"), "x")
		if err := os.Rename(outDir, filepath.Join(dir, "pack")); err == nil {
			t.Fatal("严格模式下目录移入应被拒")
		}

		// 严格模式下 mkdir 直接拒绝(消除"放行后异步纳管"竞态窗口)
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err == nil {
			t.Fatal("严格模式下 mkdir 应被拒")
		}
		t.Log("扩展名拦截: 匹配拒/大小写拒/非匹配放行/移入拒/mkdir 拒 ✓")
	})

	t.Run("wholetree", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "index.html"), "x")

		m, err := NewManager(Config{
			Mode: ModeEBPF, BlockNewFiles: true,
			Rules: []Rule{{Name: "t", Paths: []string{dir}}},
		}, newLocale(), newLogger())
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = m.Stop() }()

		if err := os.WriteFile(filepath.Join(dir, "any.txt"), []byte("x"), 0644); err == nil {
			t.Fatal("整树任意新建应被拒")
		}
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err == nil {
			t.Fatal("整树 mkdir 应被拒")
		}
		if err := os.Symlink("/etc/passwd", filepath.Join(dir, "ln")); err == nil {
			t.Fatal("整树 symlink 应被拒")
		}
		t.Log("整树拦截: 新建/建目录/符号链接全拒 ✓")
	})
}

// TestEBPFExtFailClosed 混入无法进内核的扩展名时,该目录必须升级整树而非漏保护
func TestEBPFExtFailClosed(t *testing.T) {
	requireEBPF(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.php"), "x")

	m, err := NewManager(Config{
		Mode: ModeEBPF, BlockNewFiles: true,
		// abcdefghijklmnop 超 14 字节内核限制,dirValue 应升级整树
		Rules: []Rule{{Name: "t", Paths: []string{dir}, Exts: []string{"php", "abcdefghijklmnop"}}},
	}, newLocale(), newLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop() }()

	// 未匹配 php 的任意新建也应被拒(升级整树后 flags=0 匹配所有名字)
	if err := os.WriteFile(filepath.Join(dir, "any.txt"), []byte("x"), 0644); err == nil {
		t.Fatal("部分扩展名被拒时应升级整树,任意新建应被拒")
	}
	t.Log("扩展名 fail-closed: 无法进内核的扩展名混入 → 升级整树 ✓")
}

// TestEBPFCreateObserve 观察模式:新建放行、上报事件并自动纳保
func TestEBPFCreateObserve(t *testing.T) {
	requireEBPF(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.php"), "x")

	m, err := NewManager(Config{
		Mode: ModeEBPF, BlockNewFiles: false,
		Rules: []Rule{{Name: "t", Paths: []string{dir}, Exts: []string{"php"}}},
	}, newLocale(), newLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop() }()

	np := filepath.Join(dir, "new.php")
	if err := os.WriteFile(np, []byte("x"), 0644); err != nil {
		t.Fatalf("观察模式新建应放行: %v", err)
	}
	ev := waitEvent(m.Events(), func(e Event) bool { return e.Op == OpCreate && e.Path == np })
	if ev == nil {
		t.Fatal("应收到新建事件")
	}
	if ev.PID == 0 || ev.Comm == "" || ev.Denied {
		t.Fatalf("事件应携带进程信息且未标记拒绝: %+v", ev)
	}

	// 长文件名(64+ 字节,如中文)不得截断,事件仍能定位并纳保
	long := filepath.Join(dir, strings.Repeat("防篡改测试超长文件名", 3)+".php")
	if err := os.WriteFile(long, []byte("x"), 0644); err != nil {
		t.Fatalf("长文件名新建应放行: %v", err)
	}
	if waitEvent(m.Events(), func(e Event) bool { return e.Op == OpCreate && e.Path == long }) == nil {
		t.Fatal("应收到长文件名新建事件")
	}

	// 纳保生效后写入被拒
	deadline := time.Now().Add(3 * time.Second)
	for !tryWrite(np) {
		if time.Now().After(deadline) {
			t.Fatal("新文件未在期限内纳保")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 观察模式软命中:目录移入放行并整树补扫,内部匹配文件被纳保
	outDir := filepath.Join(t.TempDir(), "pack")
	if err := os.Mkdir(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outDir, "inner.php"), "x")
	moved := filepath.Join(dir, "pack")
	if err := os.Rename(outDir, moved); err != nil {
		t.Fatalf("观察模式目录移入应放行: %v", err)
	}
	inner := filepath.Join(moved, "inner.php")
	deadline = time.Now().Add(3 * time.Second)
	for !tryWrite(inner) {
		if time.Now().After(deadline) {
			t.Fatal("移入目录内文件未在期限内补扫纳保")
		}
		time.Sleep(50 * time.Millisecond)
	}
	// 非匹配后缀不上报不纳保
	tp := filepath.Join(dir, "free.txt")
	if err := os.WriteFile(tp, []byte("x"), 0644); err != nil {
		t.Fatalf("非匹配新建应放行: %v", err)
	}
	if tryWrite(tp) {
		t.Fatal("非匹配文件不应被纳保")
	}
	t.Log("观察模式: 放行+事件+自动纳保、非匹配无感 ✓")
}

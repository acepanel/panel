//go:build linux

package tamper

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"golang.org/x/sys/unix"

	"github.com/acepanel/panel/v3/pkg/chattr"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// tryWrite 尝试以写方式打开文件,返回是否被拒绝
func tryWrite(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return true // 被拒
	}
	_ = f.Close()
	return false
}

func chattrSupported(dir string) bool {
	p := filepath.Join(dir, ".chattr_probe")
	if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
		return false
	}
	defer func() { _ = os.Remove(p) }()
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	if err := chattr.SetAttr(f, chattr.FS_IMMUTABLE_FL); err != nil {
		return false
	}
	_ = chattr.UnsetAttr(f, chattr.FS_IMMUTABLE_FL)
	return true
}

func newLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newLocale() *gotext.Locale {
	return gotext.NewLocale("", "en")
}

func TestChattrMode(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("需要 root")
	}
	dir := t.TempDir()
	if !chattrSupported(dir) {
		t.Skip("文件系统不支持 immutable 属性")
	}

	protected := filepath.Join(dir, "index.html")
	free := filepath.Join(dir, "cache.txt")
	writeFile(t, protected, "original")
	writeFile(t, free, "cache")

	m, err := NewManager(Config{
		Mode:  ModeChattr,
		Rules: []Rule{{Name: "test", Paths: []string{dir}, Exts: []string{"html"}}},
	}, newLocale(), newLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}

	if !tryWrite(protected) {
		_ = m.Stop()
		t.Fatal("受保护文件应写入被拒")
	}
	if tryWrite(free) {
		_ = m.Stop()
		t.Fatal("非保护文件应可写")
	}

	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if tryWrite(protected) {
		t.Fatal("停止后受保护文件应恢复可写")
	}
	t.Log("chattr 模式: 保护生效、非保护放行、停止后恢复 ✓")
}

func TestEBPFMode(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("需要 root")
	}
	st := DetectEBPF()
	if !st.Available {
		t.Skipf("eBPF 不可用: %s (lsm=%s)", st.Reason, st.ActiveLSM)
	}
	t.Logf("内核 %s, LSM=%s", st.KernelVersion, st.ActiveLSM)

	dir := t.TempDir()
	protected := filepath.Join(dir, "index.php")
	free := filepath.Join(dir, "upload.txt")
	victim := filepath.Join(dir, "del.php")
	writeFile(t, protected, "<?php original")
	writeFile(t, free, "upload")
	writeFile(t, victim, "<?php delete-me")

	m, err := NewManager(Config{
		Mode:  ModeEBPF,
		Rules: []Rule{{Name: "test", Paths: []string{dir}, Exts: []string{"php"}}},
	}, newLocale(), newLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop() }()

	// 写拦截
	if !tryWrite(protected) {
		t.Fatal("受保护 .php 应写入被拒")
	}
	// 非保护放行
	if tryWrite(free) {
		t.Fatal("非保护 .txt 应可写")
	}
	// 删除拦截
	if err := os.Remove(victim); err == nil {
		t.Fatal("受保护 .php 应删除被拒")
	}
	// 重命名拦截
	if err := os.Rename(protected, filepath.Join(dir, "renamed.php")); err == nil {
		t.Fatal("受保护 .php 应重命名被拒")
	}
	// 改名覆盖拦截(rename 到受保护目标,经 new_dentry 检查)
	evil := filepath.Join(dir, "evil.txt")
	writeFile(t, evil, "evil")
	if err := os.Rename(evil, victim); err == nil {
		t.Fatal("改名覆盖受保护 .php 应被拒")
	}
	// 截断拦截(truncate 不经写打开,经 inode_setattr)
	if err := os.Truncate(protected, 0); err == nil {
		t.Fatal("受保护 .php 应截断被拒")
	}
	// 属性修改拦截
	if err := os.Chmod(protected, 0777); err == nil {
		t.Fatal("受保护 .php 应 chmod 被拒")
	}
	// xattr 修改拦截(P1-3:setxattr/removexattr 独立 hook)
	if err := unix.Setxattr(protected, "user.evil", []byte("x"), 0); err == nil {
		t.Fatal("受保护 .php 应 setxattr 被拒")
	}
	// 硬链接源拦截(P1-2:防止旁路别名)
	if err := os.Link(protected, filepath.Join(dir, "alias.php")); err == nil {
		t.Fatal("对受保护 .php 建硬链接应被拒")
	}
	// 目录对象保护:rmdir/rename/chmod 目录本身(P1-1)
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	m.Relock([]string{subDir}) // 显式纳保空目录
	if err := os.Remove(subDir); err == nil {
		t.Fatal("受保护空目录应 rmdir 被拒")
	}
	if err := os.Rename(subDir, filepath.Join(dir, "renamed")); err == nil {
		t.Fatal("受保护目录应 rename 被拒")
	}
	if err := os.Chmod(subDir, 0700); err == nil {
		t.Fatal("受保护目录应 chmod 被拒")
	}

	// 事件回填路径
	select {
	case ev := <-m.Events():
		if ev.Path == "" {
			t.Error("事件应回填路径")
		}
		t.Logf("拦截事件: op=%s path=%s comm=%s pid=%d", ev.OpStr, ev.Path, ev.Comm, ev.PID)
	case <-time.After(2 * time.Second):
		t.Error("应收到拦截事件")
	}

	// 解锁后可写,恢复后再拒
	m.Unlock([]string{protected})
	if tryWrite(protected) {
		t.Error("解锁后应可写")
	}
	m.Relock([]string{protected})
	if !tryWrite(protected) {
		t.Error("恢复后应再次被拒")
	}

	t.Log("eBPF 模式: 写/删/改名/覆盖/截断/属性拦截、事件回填、解锁恢复 ✓")
}

func TestScanExcludeAndExt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.php"), "1")
	writeFile(t, filepath.Join(dir, "b.txt"), "2")
	_ = os.MkdirAll(filepath.Join(dir, "cache"), 0755)
	writeFile(t, filepath.Join(dir, "cache", "c.php"), "3")

	m := &Manager{cfg: Config{
		Rules: []Rule{{Paths: []string{dir}, Exts: []string{"php"}, Excludes: []string{"cache"}}},
	}}
	entries := m.scan()

	var files, dirs []string
	for _, e := range entries {
		if e.isDir {
			dirs = append(dirs, e.path)
			if len(e.exts) != 1 || e.exts[0] != "php" {
				t.Fatalf("目录条目应携带规则扩展名,实得 %v", e.exts)
			}
			continue
		}
		files = append(files, filepath.Base(e.path))
	}
	if len(files) != 1 || files[0] != "a.php" {
		t.Fatalf("期望仅 a.php,实得 %v", files)
	}
	if len(dirs) != 1 || dirs[0] != dir {
		t.Fatalf("期望仅根目录条目(排除目录不产出),实得 %v", dirs)
	}
	// inode 应有效
	var st syscall.Stat_t
	_ = syscall.Lstat(filepath.Join(dir, "a.php"), &st)
	for _, e := range entries {
		if !e.isDir && e.inode != st.Ino {
			t.Fatalf("inode 不匹配")
		}
	}

	// 多规则覆盖同一目录:扩展名并集,整树优先
	m2 := &Manager{cfg: Config{Rules: []Rule{
		{Paths: []string{dir}, Exts: []string{"php"}},
		{Paths: []string{dir}, Exts: []string{"html"}},
	}}}
	for _, e := range m2.scan() {
		if e.isDir && e.path == dir && (len(e.exts) != 2 || !slices.Contains(e.exts, "html")) {
			t.Fatalf("重叠规则目录应合并扩展名,实得 %v", e.exts)
		}
	}
	m3 := &Manager{cfg: Config{Rules: []Rule{
		{Paths: []string{dir}, Exts: []string{"php"}},
		{Paths: []string{dir}},
	}}}
	for _, e := range m3.scan() {
		if e.isDir && e.path == dir && len(e.exts) != 0 {
			t.Fatalf("整树规则应覆盖扩展名规则,实得 %v", e.exts)
		}
	}
	t.Log("扫描: 后缀过滤 + 排除目录 + inode 采集 + 重叠规则合并 ✓")
}

// fakeEngine 记录下发给内核的增删，用于断言集合的同步
type fakeEngine struct {
	applied []fileEntry
	removed []fileEntry
}

func (e *fakeEngine) apply(entries []fileEntry) error {
	e.applied = append(e.applied, entries...)
	return nil
}

func (e *fakeEngine) remove(entries []fileEntry) error {
	e.removed = append(e.removed, entries...)
	return nil
}

func (e *fakeEngine) start() error         { return nil }
func (e *fakeEngine) events() <-chan Event { return nil }
func (e *fakeEngine) close() error         { return nil }

// 同一路径换掉 inode 后旧的必须离开集合，否则它被回收复用给别的文件就会误拦
func TestRelockDropsReplacedInode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.php")
	writeFile(t, path, "one")

	eng := &fakeEngine{}
	m := &Manager{
		cfg:       Config{Rules: []Rule{{Paths: []string{dir}}}},
		t:         newLocale(),
		log:       newLogger(),
		eng:       eng,
		entries:   make(map[string]fileEntry),
		inodePath: make(map[fileKey]string),
	}

	m.Relock([]string{path})
	must.Len(t, m.entries, 1)
	old := m.entries[path]
	check.Equal(t, m.inodePath[fileKey{old.dev, old.inode}], path)

	// 写临时文件再改名，面板保存文件就是这么做的，路径不变但换了 inode
	tmp := path + ".tmp"
	writeFile(t, tmp, "two")
	must.NoError(t, os.Rename(tmp, path))

	m.Relock([]string{path})

	cur := m.entries[path]
	must.False(t, cur.sameObject(old))
	must.Len(t, eng.removed, 1)
	check.True(t, eng.removed[0].sameObject(old))
	check.Equal(t, m.inodePath[fileKey{cur.dev, cur.inode}], path)
	// 旧 inode 不能再留在回填表里，否则内核里的陈旧条目也无从摘除
	_, stale := m.inodePath[fileKey{old.dev, old.inode}]
	check.False(t, stale)
	check.Len(t, m.entries, 1)
}

// inode 没变时不该产生多余的摘除
func TestRelockKeepsSameInode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.php")
	writeFile(t, path, "one")

	eng := &fakeEngine{}
	m := &Manager{
		cfg:       Config{Rules: []Rule{{Paths: []string{dir}}}},
		t:         newLocale(),
		log:       newLogger(),
		eng:       eng,
		entries:   make(map[string]fileEntry),
		inodePath: make(map[fileKey]string),
	}

	m.Relock([]string{path})
	m.Relock([]string{path})

	check.Empty(t, eng.removed)
	check.Len(t, m.entries, 1)
}

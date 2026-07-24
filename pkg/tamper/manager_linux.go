//go:build linux

package tamper

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

type fileEntry struct {
	path  string
	dev   uint64
	inode uint64
	isDir bool
	exts  []string // 目录条目的规则扩展名,空=整树
}

type fileKey struct {
	dev   uint64
	inode uint64
}

type engine interface {
	apply(entries []fileEntry) error
	remove(entries []fileEntry) error
	start() error
	events() <-chan Event
	close() error
}

type Manager struct {
	cfg    Config
	log    *slog.Logger
	eng    engine
	isEBPF bool

	watcher *fsnotify.Watcher

	mu         sync.RWMutex
	entries    []fileEntry
	inodePath  map[fileKey]string // eBPF 事件回填路径
	nFiles     int
	nWatchDirs int
	running    bool

	out    chan Event
	closed chan struct{}
	once   sync.Once
}

func NewManager(cfg Config, log *slog.Logger) (*Manager, error) {
	m := &Manager{
		cfg:       cfg,
		log:       log,
		inodePath: make(map[fileKey]string),
		out:       make(chan Event, 256),
		closed:    make(chan struct{}),
	}

	switch cfg.Mode {
	case ModeEBPF:
		st := DetectEBPF()
		if !st.Available {
			return nil, fmt.Errorf("eBPF mode unavailable: %s", st.Reason)
		}
		var exts []string
		for _, r := range cfg.Rules {
			exts = append(exts, r.Exts...)
		}
		eng, err := newEBPFEngine(log, cfg.BlockNewFiles, exts)
		if err != nil {
			return nil, err
		}
		m.eng = eng
		m.isEBPF = true
	case ModeChattr:
		m.eng = newChattrEngine()
	default:
		return nil, fmt.Errorf("unknown tamper mode: %s", cfg.Mode)
	}

	return m, nil
}

type mountDev struct {
	point string
	dev   uint64
}

var mountUnescape = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

// 短 TTL 缓存，handleCreate 突发时避免每 event 都读整个 mountinfo
var (
	mountsMu sync.Mutex
	mounts   []mountDev
	mountsAt time.Time
)

const mountsTTL = time.Second

func cachedMountDevs() []mountDev {
	mountsMu.Lock()
	if time.Since(mountsAt) > mountsTTL {
		mounts = loadMountDevs()
		mountsAt = time.Now()
	}
	m := mounts
	mountsMu.Unlock()
	return m
}

// refreshMountDevs 强制刷新缓存
func refreshMountDevs() {
	mountsMu.Lock()
	mounts = loadMountDevs()
	mountsAt = time.Now()
	mountsMu.Unlock()
}

// loadMountDevs 从 mountinfo 第三列取内核 s_dev
// btrfs 等文件系统的 st_dev 是每子卷匿名设备,与 eBPF 侧 i_sb->s_dev 不一致会静默漏保护
func loadMountDevs() []mountDev {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	var mounts []mountDev
	for line := range strings.Lines(string(data)) {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		majS, minS, ok := strings.Cut(f[2], ":")
		if !ok {
			continue
		}
		maj, err1 := strconv.ParseUint(majS, 10, 32)
		min, err2 := strconv.ParseUint(minS, 10, 32)
		if err1 != nil || err2 != nil {
			continue
		}
		mounts = append(mounts, mountDev{point: mountUnescape.Replace(f[4]), dev: maj<<20 | min&0xfffff})
	}
	return mounts
}

// 同点位后挂载遮蔽先挂载,故最长前缀并列取靠后者
func devOfPath(mounts []mountDev, path string, st *syscall.Stat_t) uint64 {
	best := -1
	for i, m := range mounts {
		if UnderRoot(path, m.point) && (best < 0 || len(m.point) >= len(mounts[best].point)) {
			best = i
		}
	}
	if best >= 0 {
		return mounts[best].dev
	}
	return uint64(unix.Major(uint64(st.Dev)))<<20 | uint64(unix.Minor(uint64(st.Dev)))&0xfffff
}

func statOf(mounts []mountDev, path string) (dev, ino uint64) {
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return 0, 0
	}
	return devOfPath(mounts, path, &st), st.Ino
}

// 目录恒产出携带规则扩展名,文件按 rule.Exts 过滤(空=全部)
func walkRule(mounts []mountDev, root string, rule *Rule, emit func(fileEntry)) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if isExcluded(path, rule.Excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			dev, ino := statOf(mounts, path)
			emit(fileEntry{path: path, dev: dev, inode: ino, isDir: true, exts: rule.Exts})
			return nil
		}
		if MatchExt(path, rule.Exts) {
			dev, ino := statOf(mounts, path)
			emit(fileEntry{path: path, dev: dev, inode: ino, isDir: false})
		}
		return nil
	})
}

// 多规则覆盖同一目录时合并扩展名:整树优先,否则并集
func (m *Manager) scan() []fileEntry {
	mounts := cachedMountDevs()
	var entries []fileEntry
	seen := make(map[string]bool)
	dirIdx := make(map[string]int)

	for _, rule := range m.cfg.Rules {
		for _, root := range rule.Paths {
			walkRule(mounts, root, &rule, func(e fileEntry) {
				if !e.isDir {
					if seen[e.path] {
						return
					}
					seen[e.path] = true
					entries = append(entries, e)
					return
				}
				i, ok := dirIdx[e.path]
				if !ok {
					dirIdx[e.path] = len(entries)
					entries = append(entries, e)
					return
				}
				if len(entries[i].exts) == 0 || len(e.exts) == 0 {
					entries[i].exts = nil
					return
				}
				merged := slices.Clone(entries[i].exts) // 避免 append 污染 rule.Exts

				for _, x := range e.exts {
					if !slices.Contains(merged, x) {
						merged = append(merged, x)
					}
				}
				entries[i].exts = merged
			})
		}
	}

	return entries
}

func (m *Manager) Start() error {
	refreshMountDevs() // 显式登记入口强制取最新 mountinfo
	entries := m.scan()

	m.mu.Lock()
	m.entries = entries
	for _, e := range entries {
		m.inodePath[fileKey{e.dev, e.inode}] = e.path
	}
	m.recount()
	m.running = true
	m.mu.Unlock()

	if err := m.eng.apply(entries); err != nil {
		return fmt.Errorf("failed to apply protection: %w", err)
	}
	if err := m.eng.start(); err != nil {
		return fmt.Errorf("failed to start protection: %w", err)
	}

	if ch := m.eng.events(); ch != nil {
		go m.forwardEvents(ch)
	}

	// chattr 靠 fsnotify 补新建监控;eBPF 由创建类 LSM hook 全内核处理
	if !m.isEBPF {
		if err := m.startWatcher(); err != nil {
			m.log.Warn("failed to start tamper file watcher, new file interception unavailable", slog.Any("err", err))
		}
	}

	st := m.Stats()
	m.log.Info("tamper protection enabled",
		slog.String("mode", string(m.cfg.Mode)),
		slog.Int("files", st.ProtectedFiles),
		slog.Int("dirs", st.ProtectedDirs))
	return nil
}

func (m *Manager) startWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	m.watcher = w

	dirs := make(map[string]bool)
	for _, rule := range m.cfg.Rules {
		for _, root := range rule.Paths {
			_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil || !d.IsDir() {
					return nil
				}
				if isExcluded(path, rule.Excludes) {
					return filepath.SkipDir
				}
				dirs[path] = true
				return nil
			})
		}
	}
	failed := 0
	for dir := range dirs {
		if err := w.Add(dir); err != nil {
			failed++
		}
	}
	if failed > 0 {
		// 常见原因 fs.inotify.max_user_watches 耗尽
		m.log.Warn("failed to watch some tamper directories, new files may be missed",
			slog.Int("failed", failed), slog.Int("total", len(dirs)))
	}

	m.mu.Lock()
	m.nWatchDirs = len(dirs) - failed
	m.mu.Unlock()

	go m.watchLoop()
	return nil
}

func (m *Manager) watchLoop() {
	for {
		select {
		case <-m.closed:
			return
		case ev, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&fsnotify.Create != 0 {
				m.onCreate(ev.Name)
			}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.log.Warn("tamper watcher error", slog.Any("err", err))
		}
	}
}

// 新目录补扫竞态窗口内已落入的条目
func (m *Manager) onCreate(path string) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		if m.watcher != nil {
			if err := m.watcher.Add(path); err == nil {
				m.mu.Lock()
				m.nWatchDirs++
				m.mu.Unlock()
			}
		}
		if items, err := os.ReadDir(path); err == nil {
			for _, it := range items {
				m.onCreate(filepath.Join(path, it.Name()))
			}
		}
		return
	}

	rule := m.ruleOf(path)
	if rule == nil || isExcluded(path, rule.Excludes) || !MatchExt(path, rule.Exts) {
		return
	}

	dev, ino := statOf(cachedMountDevs(), path)
	ev := Event{
		Path:  path,
		Dev:   dev,
		Inode: ino,
		Op:    OpCreate,
		OpStr: OpCreate.String(),
		Time:  time.Now(),
	}

	if m.cfg.BlockNewFiles {
		// chattr 整树模式下父目录带 +a,删除新文件需临时解除
		if err := os.Remove(path); err != nil && len(rule.Exts) == 0 {
			if _, ok := m.eng.(*chattrEngine); ok {
				parent := []fileEntry{{path: filepath.Dir(path), isDir: true}}
				_ = m.eng.remove(parent)
				_ = os.Remove(path)
				_ = m.eng.apply(parent)
			}
		}
	} else {
		m.remember([]fileEntry{{path: path, dev: dev, inode: ino}})
		if e, ok := m.eng.(*chattrEngine); ok {
			e.lockOne(path, false)
		}
	}

	m.emit(ev)
}

func (m *Manager) ruleOf(path string) *Rule {
	for i := range m.cfg.Rules {
		for _, root := range m.cfg.Rules[i].Paths {
			if UnderRoot(path, root) {
				return &m.cfg.Rules[i]
			}
		}
	}
	return nil
}

// 内核已拒绝的直接上报,放行的(观察/软命中)交 handleCreate 异步纳保
func (m *Manager) forwardEvents(ch <-chan Event) {
	for {
		select {
		case <-m.closed:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			m.mu.RLock()
			ev.Path = m.inodePath[fileKey{ev.Dev, ev.Inode}]
			m.mu.RUnlock()
			ev.Time = time.Now()
			if ev.Op == OpCreate {
				if ev.Path == "" || ev.Name == "" {
					continue
				}
				ev.Path = filepath.Join(ev.Path, ev.Name)
				if ev.Denied {
					m.emit(ev)
				} else {
					go m.handleCreate(ev)
				}
				continue
			}
			m.emit(ev)
		}
	}
}

// hook 早于实际落盘,stat 需重试;新目录整树补扫
func (m *Manager) handleCreate(ev Event) {
	var entries []fileEntry
	for range 5 {
		if entries = statEntries([]string{ev.Path}); len(entries) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(entries) == 0 {
		return
	}

	// 内核不感知排除规则,被排除路径的新建不纳保不上报
	rule := m.ruleOf(ev.Path)
	if rule == nil || isExcluded(ev.Path, rule.Excludes) {
		return
	}
	if entries[0].isDir {
		entries = entries[:0]
		walkRule(cachedMountDevs(), ev.Path, rule, func(e fileEntry) { entries = append(entries, e) })
	} else if !MatchExt(ev.Path, rule.Exts) {
		return
	}
	m.remember(entries)
	_ = m.eng.apply(entries)
	m.emit(ev)
}

func (m *Manager) emit(ev Event) {
	select {
	case m.out <- ev:
	case <-m.closed:
	default:
	}
}

func (m *Manager) Events() <-chan Event { return m.out }

func (m *Manager) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Stats{
		Mode:           m.cfg.Mode,
		Running:        m.running,
		ProtectedFiles: m.nFiles,
		ProtectedDirs:  m.nWatchDirs,
	}
}

// 现场 stat,移动/替换后 inode 可能变化
func statEntries(paths []string) []fileEntry {
	mounts := cachedMountDevs()
	entries := make([]fileEntry, 0, len(paths))
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		dev, ino := statOf(mounts, p)
		entries = append(entries, fileEntry{path: p, dev: dev, inode: ino, isDir: info.IsDir()})
	}
	return entries
}

func (m *Manager) remember(entries []fileEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		m.inodePath[fileKey{e.dev, e.inode}] = e.path
	}
	m.entries = slices.DeleteFunc(m.entries, func(x fileEntry) bool {
		return slices.ContainsFunc(entries, func(e fileEntry) bool { return e.path == x.path })
	})
	m.entries = append(m.entries, entries...)
	m.recount()
}

func (m *Manager) forget(entries []fileEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		delete(m.inodePath, fileKey{e.dev, e.inode})
	}
	m.entries = slices.DeleteFunc(m.entries, func(x fileEntry) bool {
		return slices.ContainsFunc(entries, func(e fileEntry) bool { return e.path == x.path })
	})
	m.recount()
}

// 调用方持锁;chattr 由 fsnotify 按监控口径另计
func (m *Manager) recount() {
	m.nFiles = 0
	dirs := 0
	for _, e := range m.entries {
		if e.isDir {
			dirs++
		} else {
			m.nFiles++
		}
	}
	if m.isEBPF {
		m.nWatchDirs = dirs
	}
}

func (m *Manager) Unlock(paths []string) {
	entries := statEntries(paths)
	m.forget(entries)
	_ = m.eng.remove(entries)
}

// Relock 移动/替换后需重新登记新路径与新 inode
func (m *Manager) Relock(paths []string) {
	refreshMountDevs()
	entries := statEntries(paths)
	for i := range entries {
		if entries[i].isDir {
			if r := m.ruleOf(entries[i].path); r != nil {
				entries[i].exts = r.Exts
			}
		}
	}
	m.remember(entries)
	_ = m.eng.apply(entries)
}

func (m *Manager) Stop() error {
	m.once.Do(func() { close(m.closed) })

	if m.watcher != nil {
		_ = m.watcher.Close()
	}

	m.mu.RLock()
	entries := m.entries
	m.mu.RUnlock()
	// eBPF 保护随引擎关闭销毁
	if !m.isEBPF {
		_ = m.eng.remove(entries)
	}

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	return m.eng.close()
}

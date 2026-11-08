package biz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/pkg/tamper"
)

// TamperRule 防篡改保护规则(通常对应一个网站目录)
type TamperRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;default:'';unique" json:"name"` // 标识
	Path      string    `gorm:"not null;default:''" json:"path"`        // 受保护目录
	Exts      []string  `gorm:"serializer:json" json:"exts"`            // 受保护后缀，空=全部
	Excludes  []string  `gorm:"serializer:json" json:"excludes"`        // 排除子路径
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TamperLog 篡改拦截/告警日志
type TamperLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"not null;default:'';serializer:zstd" json:"path"`
	Op        string    `gorm:"not null;default:''" json:"op"` // write/unlink/rename/setattr/create
	PID       uint      `gorm:"not null;default:0" json:"pid"`
	Comm      string    `gorm:"not null;default:''" json:"comm"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// TamperSetting 防篡改全局设置
type TamperSetting struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`            // chattr / ebpf
	BlockNewFiles bool   `json:"block_new_files"` // 新建受保护类型文件时删除拦截
	LogDays       uint   `json:"log_days"`        // 日志保留天数
}

// TamperRepo 防篡改数据访问接口
type TamperRepo interface {
	ListRules() ([]*TamperRule, error)
	GetRule(id uint) (*TamperRule, error)
	GetRuleByName(name string) (*TamperRule, error)
	CreateRule(rule *TamperRule) error
	UpdateRule(rule *TamperRule) error
	DeleteRule(id uint) error
	AddLogs(logs []*TamperLog) error
	ListLogs(page, limit uint) ([]*TamperLog, int64, error)
	ClearLogs() error
	ClearLogsBefore(t time.Time) error
	VacuumDB() error
}

// TamperUsecase 防篡改业务逻辑,持有运行时 Manager
type TamperUsecase struct {
	repo    TamperRepo
	setting *SettingUsecase
	notify  *NotifyUsecase
	log     *slog.Logger
	t       *gotext.Locale

	mu        sync.Mutex
	mgr       *tamper.Manager
	cleanedAt time.Time
	buf       []*TamperLog
	bufMu     sync.Mutex
	notifyAt  time.Time
	drainC    chan struct{}
}

func NewTamperUsecase(notifyUsecase *NotifyUsecase, settingUsecase *SettingUsecase, t *gotext.Locale, log *slog.Logger, tamperRepo TamperRepo) *TamperUsecase {
	return &TamperUsecase{
		repo:    tamperRepo,
		setting: settingUsecase,
		notify:  notifyUsecase,
		log:     log,
		t:       t,
	}
}

// Supported 当前平台是否支持防篡改
func (uc *TamperUsecase) Supported() bool {
	return tamper.Supported()
}

// DetectEBPF 检测 eBPF 模式可用性
func (uc *TamperUsecase) DetectEBPF() tamper.EBPFStatus {
	return tamper.DetectEBPF()
}

// EnableBPFLSMGrub 修改 grub 激活 bpf LSM(需重启系统生效)
func (uc *TamperUsecase) EnableBPFLSMGrub(ctx context.Context) error {
	if err := tamper.EnableBPFLSMGrub(ctx); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("tamper ebpf activated"), slog.String("type", OperationTypeSafe), slog.Uint64("operator_id", OperatorID(ctx)))

	return nil
}

// GetSetting 读取全局设置
func (uc *TamperUsecase) GetSetting() (*TamperSetting, error) {
	enabled, _ := uc.setting.GetBool(SettingKeyTamperEnabled)
	mode, _ := uc.setting.Get(SettingKeyTamperMode)
	if mode == "" {
		mode = string(tamper.ModeChattr)
	}
	blockNew, _ := uc.setting.GetBool(SettingKeyTamperBlockNew)
	logDays, _ := uc.setting.GetInt(SettingKeyTamperLogDays, 30)
	return &TamperSetting{
		Enabled:       enabled,
		Mode:          mode,
		BlockNewFiles: blockNew,
		LogDays:       uint(logDays),
	}, nil
}

// SaveSetting 保存全局设置并立即生效
func (uc *TamperUsecase) SaveSetting(ctx context.Context, s *TamperSetting) error {
	if err := uc.setting.Set(SettingKeyTamperMode, s.Mode); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyTamperBlockNew, cast.ToString(s.BlockNewFiles)); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyTamperLogDays, cast.ToString(s.LogDays)); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyTamperEnabled, cast.ToString(s.Enabled)); err != nil {
		return err
	}
	if err := uc.Reconcile(); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("tamper settings updated"), slog.String("type", OperationTypeSafe), slog.Uint64("operator_id", OperatorID(ctx)), slog.Bool("enabled", s.Enabled), slog.String("mode", s.Mode), slog.Bool("block_new_files", s.BlockNewFiles), slog.Uint64("log_days", uint64(s.LogDays)))

	return nil
}

// Rules 规则管理
func (uc *TamperUsecase) ListRules() ([]*TamperRule, error) { return uc.repo.ListRules() }

func (uc *TamperUsecase) GetRule(id uint) (*TamperRule, error) { return uc.repo.GetRule(id) }

func (uc *TamperUsecase) CreateRule(ctx context.Context, rule *TamperRule) error {
	if err := uc.createRule(rule); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("tamper rule created"), slog.String("type", OperationTypeSafe), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(rule.ID)), slog.String("name", rule.Name), slog.String("path", rule.Path), slog.Bool("enabled", rule.Enabled), slog.Any("exts", rule.Exts), slog.Any("excludes", rule.Excludes))

	return nil
}

func (uc *TamperUsecase) UpdateRule(ctx context.Context, rule *TamperRule) error {
	if err := uc.updateRule(rule); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("tamper rule updated"), slog.String("type", OperationTypeSafe), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(rule.ID)), slog.String("name", rule.Name), slog.String("path", rule.Path), slog.Bool("enabled", rule.Enabled), slog.Any("exts", rule.Exts), slog.Any("excludes", rule.Excludes))

	return nil
}

func (uc *TamperUsecase) DeleteRule(ctx context.Context, id uint) error {
	rule, err := uc.repo.GetRule(id)
	if err != nil {
		return err
	}
	if err = uc.deleteRule(id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("tamper rule deleted"), slog.String("type", OperationTypeSafe), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", rule.Name), slog.String("path", rule.Path))

	return nil
}

func (uc *TamperUsecase) createRule(rule *TamperRule) error {
	if err := uc.repo.CreateRule(rule); err != nil {
		return err
	}
	return uc.Reconcile()
}

func (uc *TamperUsecase) updateRule(rule *TamperRule) error {
	if err := uc.repo.UpdateRule(rule); err != nil {
		return err
	}
	return uc.Reconcile()
}

func (uc *TamperUsecase) deleteRule(id uint) error {
	if err := uc.repo.DeleteRule(id); err != nil {
		return err
	}
	return uc.Reconcile()
}

// Logs 日志
func (uc *TamperUsecase) ListLogs(page, limit uint) ([]*TamperLog, int64, error) {
	// 先落缓冲,拦截日志即时可见
	uc.FlushLogs()
	return uc.repo.ListLogs(page, limit)
}

func (uc *TamperUsecase) ClearLogs(ctx context.Context) error {
	if err := uc.repo.ClearLogs(); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("tamper logs cleared"), slog.String("type", OperationTypeSafe), slog.Uint64("operator_id", OperatorID(ctx)))

	return nil
}

// buildConfig 从设置与规则构造运行配置
func (uc *TamperUsecase) buildConfig(s *TamperSetting) (tamper.Config, error) {
	rules, err := uc.repo.ListRules()
	if err != nil {
		return tamper.Config{}, err
	}
	cfg := tamper.Config{
		Mode:          tamper.Mode(s.Mode),
		BlockNewFiles: s.BlockNewFiles,
	}
	for _, r := range rules {
		if !r.Enabled || r.Path == "" {
			continue
		}
		cfg.Rules = append(cfg.Rules, tamper.Rule{
			Name:     r.Name,
			Paths:    []string{r.Path},
			Exts:     r.Exts,
			Excludes: r.Excludes,
		})
	}
	return cfg, nil
}

// Reconcile 依据设置启停并同步 Manager,应由设置/规则变更后调用
func (uc *TamperUsecase) Reconcile() error {
	s, err := uc.GetSetting()
	if err != nil {
		return err
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()

	// 未启用或无可用规则:停止
	if !s.Enabled {
		uc.stopLocked()
		return nil
	}

	cfg, err := uc.buildConfig(s)
	if err != nil {
		return err
	}
	if len(cfg.Rules) == 0 {
		uc.stopLocked()
		return nil
	}

	// 重启 Manager 以套用最新配置(规则/模式可能变化)
	uc.stopLocked()
	mgr, err := tamper.NewManager(cfg, uc.t, uc.log)
	if err != nil {
		return err
	}
	if err = mgr.Start(); err != nil {
		_ = mgr.Stop()
		return err
	}
	uc.mgr = mgr
	uc.drainC = make(chan struct{})
	go uc.drain(mgr, uc.drainC)
	return nil
}

// stopLocked 停止当前 Manager(调用方持锁)
func (uc *TamperUsecase) stopLocked() {
	if uc.mgr != nil {
		if uc.drainC != nil {
			close(uc.drainC)
			uc.drainC = nil
		}
		_ = uc.mgr.Stop()
		uc.mgr = nil
	}
}

// drain 持续读取拦截事件缓冲到内存
func (uc *TamperUsecase) drain(mgr *tamper.Manager, done chan struct{}) {
	ch := mgr.Events()
	for {
		select {
		case <-done:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			uc.bufMu.Lock()
			uc.buf = append(uc.buf, &TamperLog{
				Path:      ev.Path,
				Op:        ev.OpStr,
				PID:       uint(ev.PID),
				Comm:      ev.Comm,
				CreatedAt: ev.Time,
			})
			uc.bufMu.Unlock()
		}
	}
}

// FlushLogs 将缓冲的拦截日志落库,由定时任务调用
func (uc *TamperUsecase) FlushLogs() {
	uc.bufMu.Lock()
	if len(uc.buf) == 0 {
		uc.bufMu.Unlock()
		return
	}
	logs := uc.buf
	uc.buf = nil
	uc.bufMu.Unlock()

	if err := uc.repo.AddLogs(logs); err != nil {
		uc.log.Warn(uc.t.Get("failed to persist tamper logs"), slog.Any("err", err))
		return
	}

	uc.notifyBlocked(logs)
}

// notifyBlocked 汇总上报拦截事件，5 分钟内不重复通知
func (uc *TamperUsecase) notifyBlocked(logs []*TamperLog) {
	uc.bufMu.Lock()
	if time.Since(uc.notifyAt) < 5*time.Minute {
		uc.bufMu.Unlock()
		return
	}
	uc.notifyAt = time.Now()
	uc.bufMu.Unlock()

	latest := logs[len(logs)-1]
	uc.notify.SendEvent(NotifyEventTamper, uc.t.Get("[AcePanel] Tamper Protection Alert"), NotifyBody(uc.t.Get("tamper protection blocked file operations"), [][2]string{
		{uc.t.Get("Count"), cast.ToString(len(logs))},
		{uc.t.Get("Path"), latest.Path},
		{uc.t.Get("Operation"), latest.Op},
		{uc.t.Get("Process"), fmt.Sprintf("%s (%d)", latest.Comm, latest.PID)},
		{uc.t.Get("Time"), latest.CreatedAt.Format(time.DateTime)},
	}))
}

// CleanupLogs 清理过期日志
// 调用方每分钟触发，但日志按天过期，故限流到 6 小时一次
func (uc *TamperUsecase) CleanupLogs() {
	uc.mu.Lock()
	if time.Since(uc.cleanedAt) < 6*time.Hour {
		uc.mu.Unlock()
		return
	}
	uc.cleanedAt = time.Now()
	uc.mu.Unlock()

	s, err := uc.GetSetting()
	if err != nil || s.LogDays == 0 {
		return
	}
	_ = uc.repo.ClearLogsBefore(time.Now().AddDate(0, 0, -int(s.LogDays)))
}

// Running 当前是否有 Manager 在运行
func (uc *TamperUsecase) Running() bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	return uc.mgr != nil
}

// Stats 运行统计
func (uc *TamperUsecase) Stats() tamper.Stats {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.mgr == nil {
		return tamper.Stats{}
	}
	return uc.mgr.Stats()
}

// enabledRules 取启用规则并转为 tamper.Rule
func (uc *TamperUsecase) enabledRules() ([]*TamperRule, []tamper.Rule, error) {
	rules, err := uc.repo.ListRules()
	if err != nil {
		return nil, nil, err
	}
	var converted []tamper.Rule
	for _, r := range rules {
		if !r.Enabled || r.Path == "" {
			continue
		}
		converted = append(converted, tamper.Rule{
			Name:     r.Name,
			Paths:    []string{r.Path},
			Exts:     r.Exts,
			Excludes: r.Excludes,
		})
	}
	return rules, converted, nil
}

// CheckPaths 批量查询路径是否处于保护范围(防篡改未运行时全为 false)
func (uc *TamperUsecase) CheckPaths(paths []string) (map[string]bool, error) {
	res := make(map[string]bool, len(paths))
	for _, p := range paths {
		res[p] = false
	}
	if !uc.Running() {
		return res, nil
	}
	_, rules, err := uc.enabledRules()
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		res[p] = tamper.Covered(rules, p, info.IsDir())
	}
	return res, nil
}

// SetProtect 切换单个路径的保护状态,经由规则/排除联动实现:
// 保护:命中排除则移除排除项,无覆盖规则则对目录新建整树规则
// 解除:路径即规则根则删除规则,否则加入排除项
func (uc *TamperUsecase) SetProtect(ctx context.Context, path string, protect bool) error {
	path = filepath.Clean(path)
	rule, err := uc.setProtect(path, protect)
	if err != nil || rule == nil {
		return err
	}

	uc.log.Info(uc.t.Get("tamper path protection updated"), slog.String("type", OperationTypeSafe), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("path", path), slog.Bool("protect", protect), slog.Uint64("rule_id", uint64(rule.ID)), slog.String("rule_name", rule.Name))

	return nil
}

// setProtect 返回被改动的规则,无改动时返回 nil
func (uc *TamperUsecase) setProtect(path string, protect bool) (*TamperRule, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	isDir := info.IsDir()

	rules, _, err := uc.enabledRules()
	if err != nil {
		return nil, err
	}
	var covering *TamperRule
	for _, r := range rules {
		if r.Enabled && r.Path != "" && tamper.UnderRoot(path, filepath.Clean(r.Path)) {
			covering = r
			break
		}
	}

	if protect {
		if covering == nil {
			if !isDir {
				return nil, errors.New("file is not inside any protection rule, protect its directory instead")
			}
			rule := &TamperRule{Name: uc.ruleNameFor(rules, path), Path: path, Enabled: true}
			return rule, uc.createRule(rule)
		}
		// 移除导致该路径被排除的排除项
		kept := make([]string, 0, len(covering.Excludes))
		for _, ex := range covering.Excludes {
			if !tamper.ExcludeMatches(ex, path) {
				kept = append(kept, ex)
			}
		}
		if len(kept) != len(covering.Excludes) {
			covering.Excludes = kept
			return covering, uc.updateRule(covering)
		}
		if !isDir && !tamper.MatchExt(path, covering.Exts) {
			return nil, errors.New("file extension is not protected by the rule, edit the rule extensions instead")
		}
		return nil, nil // 已处于保护中
	}

	if covering == nil {
		return nil, nil // 本就未受保护
	}
	if path == filepath.Clean(covering.Path) {
		return covering, uc.deleteRule(covering.ID)
	}
	if !tamper.Covered([]tamper.Rule{{Paths: []string{covering.Path}, Exts: covering.Exts, Excludes: covering.Excludes}}, path, isDir) {
		return nil, nil // 本就未受保护(已排除或后缀不匹配)
	}
	covering.Excludes = append(covering.Excludes, path)
	return covering, uc.updateRule(covering)
}

// ruleNameFor 生成不冲突的规则名
func (uc *TamperUsecase) ruleNameFor(rules []*TamperRule, path string) string {
	name := filepath.Base(path)
	for _, r := range rules {
		if r.Name == name {
			return strings.Trim(strings.ReplaceAll(path, "/", "-"), "-")
		}
	}
	return name
}

// Unlock 临时解除指定路径保护(供面板合法写入前调用),返回是否处于保护中
func (uc *TamperUsecase) Unlock(paths ...string) bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.mgr == nil {
		return false
	}
	uc.mgr.Unlock(paths)
	return true
}

// Relock 恢复指定路径保护
func (uc *TamperUsecase) Relock(paths ...string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.mgr != nil {
		uc.mgr.Relock(paths)
	}
}

func (uc *TamperUsecase) VacuumDB() error {
	return uc.repo.VacuumDB()
}

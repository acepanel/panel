package biz

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/apploader"
)

// 告警指标类型
const (
	AlertTypeCPU           = "cpu"            // CPU 使用率 %
	AlertTypeMemory        = "memory"         // 内存使用率 %
	AlertTypeSwap          = "swap"           // Swap 使用率 %
	AlertTypeLoad1         = "load1"          // 1 分钟平均负载
	AlertTypeLoad5         = "load5"          // 5 分钟平均负载
	AlertTypeLoad15        = "load15"         // 15 分钟平均负载
	AlertTypeDisk          = "disk"           // 磁盘使用率 %，目标为挂载点
	AlertTypeDiskInode     = "disk_inode"     // 磁盘 inode 使用率 %，目标为挂载点
	AlertTypeDiskRead      = "disk_read"      // 磁盘读取速率 MB/s，目标为设备名
	AlertTypeDiskWrite     = "disk_write"     // 磁盘写入速率 MB/s，目标为设备名
	AlertTypeNetIn         = "net_in"         // 网卡下行速率 MB/s，目标为网卡名
	AlertTypeNetOut        = "net_out"        // 网卡上行速率 MB/s，目标为网卡名
	AlertTypeWebsite5xx    = "website_5xx"    // 网站本小时 5xx 次数，目标为网站名
	AlertTypeWebsiteError  = "website_error"  // 网站本小时错误率 %，目标为网站名
	AlertTypeService       = "service"        // 服务未运行，目标为服务名
	AlertTypeProject       = "project"        // 项目未运行，目标为项目名
	AlertTypeContainer     = "container"      // 容器未运行，目标为容器名
	AlertTypeApp           = "app"            // 应用未运行，目标为应用标识
	AlertTypeDatabase      = "database"       // 数据库服务器不可达，目标为服务器名
	AlertTypeCertExpire    = "cert_expire"    // 证书剩余天数，目标为域名
	AlertTypeWebsiteExpire = "website_expire" // 网站剩余天数，目标为网站名
)

// 告警比较运算符
const (
	AlertOperatorGT  = "gt"
	AlertOperatorGTE = "gte"
	AlertOperatorLT  = "lt"
	AlertOperatorLTE = "lte"
)

// AlertRule 告警规则
type AlertRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;default:''" json:"name"`
	Type      string    `gorm:"not null;default:''" json:"type"`
	Target    string    `gorm:"not null;default:''" json:"target"`     // 挂载点/网卡/磁盘/服务名/域名，空表示全部
	Operator  string    `gorm:"not null;default:'gt'" json:"operator"` // gt/gte/lt/lte
	Threshold float64   `gorm:"not null;default:0" json:"threshold"`
	Duration  uint      `gorm:"not null;default:1" json:"duration"` // 连续满足次数
	Silence   uint      `gorm:"not null;default:30" json:"silence"` // 静默期（分钟）
	Channels  []uint    `gorm:"serializer:json" json:"channels"`    // 通知渠道 ID
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Alert 告警记录
type Alert struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RuleID    uint      `gorm:"not null;default:0;index" json:"rule_id"`
	RuleName  string    `gorm:"not null;default:''" json:"rule_name"`
	Type      string    `gorm:"not null;default:''" json:"type"`
	Target    string    `gorm:"not null;default:''" json:"target"`
	Value     float64   `gorm:"not null;default:0" json:"value"`
	Message   string    `gorm:"not null;default:''" json:"message"`
	Notified  bool      `gorm:"not null;default:false" json:"notified"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// AlertMetric 单个目标的取值
type AlertMetric struct {
	Target string
	Value  float64
}

// WebsiteHourStat 网站当前小时的请求统计
type WebsiteHourStat struct {
	Site      string
	Requests  uint64
	Errors    uint64
	Status5xx uint64
}

type AlertRepo interface {
	ListRules(page, limit uint) ([]*AlertRule, int64, error)
	AllRules() ([]*AlertRule, error)
	GetRule(id uint) (*AlertRule, error)
	CreateRule(rule *AlertRule) error
	UpdateRule(rule *AlertRule) error
	DeleteRule(id uint) error
	AddAlert(alert *Alert) error
	ListAlerts(page, limit uint) ([]*Alert, int64, error)
	ClearAlerts() error
	ClearAlertsBefore(t time.Time) error
	CertExpiry() ([]*AlertMetric, error)
	WebsiteExpiry() ([]*AlertMetric, error)
	ProjectNames() ([]string, error)
	DatabaseServers() ([]*DatabaseServer, error)
	WebsiteHourStats() ([]*WebsiteHourStat, error)
}

// ioSnapshot 网卡/磁盘累计计数快照及据此算出的速率（字节/秒）
type ioSnapshot struct {
	in      uint64
	out     uint64
	inRate  uint64
	outRate uint64
	at      time.Time
}

type AlertUsecase struct {
	repo      AlertRepo
	notify    *NotifyUsecase
	setting   SettingRepo
	container ContainerRepo
	app       AppRepo
	database  DatabaseServerRepo
	loader    *apploader.Loader
	log       *slog.Logger
	t         *gotext.Locale

	mu         sync.Mutex
	hits       map[string]uint       // 连续命中次数
	silenced   map[string]time.Time  // 静默截止时间，此前不重复通知
	netSnaps   map[string]ioSnapshot // 网卡累计流量快照
	diskSnaps  map[string]ioSnapshot // 磁盘累计 IO 快照
	healthKeys map[string]struct{}   // 已通知的健康问题
	sshFired   map[string]time.Time  // SSH 爆破上次通知时间
	sshAt      time.Time             // SSH 日志上次检查时间（journald 回退路径）
	sshCursor  string                // SSH 日志上次读取到的 journal 游标
	sshLog     string                // 正在跟踪的 sshd 文本日志路径
	sshOffset  int64                 // 文本日志上次读到的字节偏移
	cleanedAt  time.Time             // 上次清理历史告警的时间
}

func NewAlertUsecase(
	notifyUsecase *NotifyUsecase,
	loader *apploader.Loader,
	t *gotext.Locale,
	log *slog.Logger,
	alertRepo AlertRepo,
	appRepo AppRepo,
	containerRepo ContainerRepo,
	databaseServerRepo DatabaseServerRepo,
	settingRepo SettingRepo,
) *AlertUsecase {
	return &AlertUsecase{
		repo:       alertRepo,
		notify:     notifyUsecase,
		setting:    settingRepo,
		container:  containerRepo,
		app:        appRepo,
		database:   databaseServerRepo,
		loader:     loader,
		log:        log,
		t:          t,
		hits:       make(map[string]uint),
		silenced:   make(map[string]time.Time),
		netSnaps:   make(map[string]ioSnapshot),
		diskSnaps:  make(map[string]ioSnapshot),
		healthKeys: make(map[string]struct{}),
		sshFired:   make(map[string]time.Time),
	}
}

func (uc *AlertUsecase) ListRules(ctx context.Context, page, limit uint) ([]*AlertRule, int64, error) {
	rules, total, err := uc.repo.ListRules(page, limit)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("alert rule list viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("page", uint64(page)), slog.Uint64("limit", uint64(limit)))

	return rules, total, nil
}

func (uc *AlertUsecase) GetRule(ctx context.Context, id uint) (*AlertRule, error) {
	rule, err := uc.repo.GetRule(id)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("alert rule viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(rule.ID)), slog.String("name", rule.Name))

	return rule, nil
}

func (uc *AlertUsecase) CreateRule(ctx context.Context, req *request.AlertRuleCreate) (*AlertRule, error) {
	rule := &AlertRule{
		Name:      req.Name,
		Type:      req.Type,
		Target:    req.Target,
		Operator:  req.Operator,
		Threshold: req.Threshold,
		Duration:  req.Duration,
		Silence:   req.Silence,
		Channels:  req.Channels,
		Enabled:   req.Enabled,
	}
	uc.normalizeRule(rule)

	if err := uc.repo.CreateRule(rule); err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("alert rule created"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(rule.ID)), slog.String("name", rule.Name), slog.String("alert_type", rule.Type), slog.String("target", rule.Target), slog.Bool("enabled", rule.Enabled))

	return rule, nil
}

func (uc *AlertUsecase) UpdateRule(ctx context.Context, req *request.AlertRuleUpdate) error {
	rule, err := uc.repo.GetRule(req.ID)
	if err != nil {
		return err
	}

	rule.Name = req.Name
	rule.Type = req.Type
	rule.Target = req.Target
	rule.Operator = req.Operator
	rule.Threshold = req.Threshold
	rule.Duration = req.Duration
	rule.Silence = req.Silence
	rule.Channels = req.Channels
	rule.Enabled = req.Enabled
	uc.normalizeRule(rule)

	if err = uc.repo.UpdateRule(rule); err != nil {
		return err
	}

	// 规则变更后重置命中状态，避免沿用旧阈值的计数
	uc.mu.Lock()
	uc.clearState(rule.ID)
	uc.mu.Unlock()

	uc.log.Info(uc.t.Get("alert rule updated"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(rule.ID)), slog.String("name", rule.Name), slog.String("alert_type", rule.Type), slog.String("target", rule.Target), slog.Bool("enabled", rule.Enabled))

	return nil
}

func (uc *AlertUsecase) DeleteRule(ctx context.Context, id uint) error {
	rule, err := uc.repo.GetRule(id)
	if err != nil {
		return err
	}
	if err = uc.repo.DeleteRule(id); err != nil {
		return err
	}

	uc.mu.Lock()
	uc.clearState(id)
	uc.mu.Unlock()

	uc.log.Warn(uc.t.Get("alert rule deleted"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", rule.Name))

	return nil
}

func (uc *AlertUsecase) ListAlerts(ctx context.Context, page, limit uint) ([]*Alert, int64, error) {
	alerts, total, err := uc.repo.ListAlerts(page, limit)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("alert record list viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("page", uint64(page)), slog.Uint64("limit", uint64(limit)))

	return alerts, total, nil
}

func (uc *AlertUsecase) ClearAlerts(ctx context.Context) error {
	if err := uc.repo.ClearAlerts(); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("alert records cleared"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)))

	return nil
}

// clearState 清除某条规则的运行时状态，调用前需持有锁
func (uc *AlertUsecase) clearState(ruleID uint) {
	prefix := fmt.Sprintf("%d:", ruleID)
	for key := range uc.hits {
		if strings.HasPrefix(key, prefix) {
			delete(uc.hits, key)
		}
	}
	for key := range uc.silenced {
		if strings.HasPrefix(key, prefix) {
			delete(uc.silenced, key)
		}
	}
}

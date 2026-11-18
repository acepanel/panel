package biz

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
)

// ScanEvent 扫描事件模型
type ScanEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SourceID  uint      `gorm:"not null;uniqueIndex:idx_scan_unique,priority:2" json:"-"`
	Port      uint      `gorm:"not null;uniqueIndex:idx_scan_unique,priority:3" json:"port"`
	Protocol  string    `gorm:"not null;default:'tcp';uniqueIndex:idx_scan_unique,priority:4" json:"protocol"`
	Date      string    `gorm:"not null;uniqueIndex:idx_scan_unique,priority:1" json:"date"` // YYYY-MM-DD
	Count     uint      `gorm:"not null;default:1" json:"count"`
	FirstSeen time.Time `gorm:"not null" json:"first_seen"`
	LastSeen  time.Time `gorm:"not null" json:"last_seen"`
	SourceIP  string    `gorm:"->;-:migration" json:"source_ip"`
	Country   string    `gorm:"->;-:migration" json:"country"`
	Region    string    `gorm:"->;-:migration" json:"region"`
	City      string    `gorm:"->;-:migration" json:"city"`
	ISP       string    `gorm:"->;-:migration" json:"isp"`
}

// ScanSource 扫描来源模型
type ScanSource struct {
	ID       uint   `gorm:"primaryKey"`
	SourceIP string `gorm:"not null;uniqueIndex:idx_scan_sources_ip"`
	Country  string `gorm:"not null;default:''"`
	Region   string `gorm:"not null;default:''"`
	City     string `gorm:"not null;default:''"`
	ISP      string `gorm:"not null;default:''"`
}

// ScanBlock 扫描感知自动屏蔽的 IP
type ScanBlock struct {
	IP        string    `gorm:"primaryKey" json:"ip"`
	ExpiresAt int64     `gorm:"not null;default:0;index" json:"expires_at"` // Unix 秒，0 为永久
	Outbound  bool      `gorm:"not null;default:false" json:"outbound"`     // 出站是否一并屏蔽，解封时据此删规则
	CreatedAt time.Time `json:"created_at"`
}

// ScanSummary 扫描汇总
type ScanSummary struct {
	TotalCount  uint `json:"total_count"`
	UniqueIPs   uint `json:"unique_ips"`
	UniquePorts uint `json:"unique_ports"`
}

// ScanDayTrend 每日趋势
type ScanDayTrend struct {
	Date       string `json:"date"`
	TotalCount uint   `json:"total_count"`
	UniqueIPs  uint   `json:"unique_ips"`
}

// ScanSourceRank 扫描源 IP 排行
type ScanSourceRank struct {
	SourceIP   string `json:"source_ip"`
	TotalCount uint   `json:"total_count"`
	PortCount  uint   `json:"port_count"`
	LastSeen   string `json:"last_seen"`
	Country    string `json:"country"`
	Region     string `json:"region"`
	City       string `json:"city"`
	ISP        string `json:"isp"`
}

// ScanPortRank 被扫描端口排行
type ScanPortRank struct {
	Port       uint   `json:"port"`
	Protocol   string `json:"protocol"`
	TotalCount uint   `json:"total_count"`
	IPCount    uint   `json:"ip_count"`
}

// ScanSetting 扫描感知设置
type ScanSetting struct {
	Enabled        bool     `json:"enabled"`
	Days           uint     `json:"days"`
	Interfaces     []string `json:"interfaces"`
	AutoBlock      bool     `json:"auto_block"`
	BlockThreshold uint     `json:"block_threshold"` // 扫描次数阈值
	BlockWindow    uint     `json:"block_window"`    // 检测窗口（分钟）
	BlockDuration  uint     `json:"block_duration"`  // 屏蔽时长（小时），0=永久
	BlockOutbound  bool     `json:"block_outbound"`  // 屏蔽时连出站一并屏蔽
	Whitelist      []string `json:"whitelist"`       // IP/CIDR 白名单
}

// ScanEventRepo 扫描事件数据访问接口
type ScanEventRepo interface {
	Upsert(events []*ScanEvent) error
	List(start, end, sourceIP string, port uint, location string, page, limit uint) ([]*ScanEvent, uint, error)
	Summary(start, end string) (*ScanSummary, error)
	Trend(start, end string) ([]*ScanDayTrend, error)
	TopSourceIPs(start, end string, limit uint) ([]*ScanSourceRank, error)
	TopPorts(start, end string, limit uint) ([]*ScanPortRank, error)
	ClearBefore(date string) error
	Clear() error
	VacuumDB() error
	// AddBlocks 记录屏蔽的 IP，已有记录的保持原样
	AddBlocks(blocks []*ScanBlock) error
	// BlockedIPs 返回给定 IP 中已有屏蔽记录的
	BlockedIPs(ips []string) ([]string, error)
	// ExpiredBlocks 返回到期的限时屏蔽
	ExpiredBlocks(now time.Time) ([]*ScanBlock, error)
	DeleteBlocks(ips []string) error
}

// ScanEventUsecase 扫描事件业务逻辑
type ScanEventUsecase struct {
	t       *gotext.Locale
	log     *slog.Logger
	repo    ScanEventRepo
	setting SettingRepo
}

func NewScanEventUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	repo ScanEventRepo,
	setting SettingRepo,
) *ScanEventUsecase {
	return &ScanEventUsecase{t: t, log: log, repo: repo, setting: setting}
}

func (uc *ScanEventUsecase) Upsert(events []*ScanEvent) error {
	return uc.repo.Upsert(events)
}

func (uc *ScanEventUsecase) List(
	start, end, sourceIP string,
	port uint,
	location string,
	page, limit uint,
) ([]*ScanEvent, uint, error) {
	return uc.repo.List(start, end, sourceIP, port, location, page, limit)
}

func (uc *ScanEventUsecase) Summary(start, end string) (*ScanSummary, error) {
	return uc.repo.Summary(start, end)
}

func (uc *ScanEventUsecase) Trend(start, end string) ([]*ScanDayTrend, error) {
	return uc.repo.Trend(start, end)
}

func (uc *ScanEventUsecase) TopSourceIPs(start, end string, limit uint) ([]*ScanSourceRank, error) {
	return uc.repo.TopSourceIPs(start, end, limit)
}

func (uc *ScanEventUsecase) TopPorts(start, end string, limit uint) ([]*ScanPortRank, error) {
	return uc.repo.TopPorts(start, end, limit)
}

func (uc *ScanEventUsecase) ClearBefore(date string) error {
	return uc.repo.ClearBefore(date)
}

func (uc *ScanEventUsecase) Clear(ctx context.Context) error {
	if err := uc.repo.Clear(); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("scan events cleared"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)))
	return nil
}

func (uc *ScanEventUsecase) VacuumDB() error {
	return uc.repo.VacuumDB()
}

func (uc *ScanEventUsecase) AddBlocks(blocks []*ScanBlock) error {
	return uc.repo.AddBlocks(blocks)
}

func (uc *ScanEventUsecase) BlockedIPs(ips []string) ([]string, error) {
	return uc.repo.BlockedIPs(ips)
}

func (uc *ScanEventUsecase) ExpiredBlocks(now time.Time) ([]*ScanBlock, error) {
	return uc.repo.ExpiredBlocks(now)
}

func (uc *ScanEventUsecase) DeleteBlocks(ips []string) error {
	return uc.repo.DeleteBlocks(ips)
}

func (uc *ScanEventUsecase) GetSetting() (*ScanSetting, error) {
	enabled, err := uc.setting.GetBool(SettingKeyScanAware)
	if err != nil {
		return nil, err
	}
	days, err := uc.setting.GetInt(SettingKeyScanAwareDays, 30)
	if err != nil {
		return nil, err
	}

	interfacesStr, err := uc.setting.Get(SettingKeyScanAwareInterfaces)
	if err != nil {
		return nil, err
	}

	var interfaces []string
	if interfacesStr != "" {
		_ = json.Unmarshal([]byte(interfacesStr), &interfaces)
	}

	autoBlock, err := uc.setting.GetBool(SettingKeyScanAwareAutoBlock)
	if err != nil {
		return nil, err
	}
	blockThreshold, err := uc.setting.GetInt(SettingKeyScanAwareBlockThreshold, 100)
	if err != nil {
		return nil, err
	}
	blockWindow, err := uc.setting.GetInt(SettingKeyScanAwareBlockWindow, 5)
	if err != nil {
		return nil, err
	}
	blockDuration, err := uc.setting.GetInt(SettingKeyScanAwareBlockDuration, 0)
	if err != nil {
		return nil, err
	}
	blockOutbound, err := uc.setting.GetBool(SettingKeyScanAwareBlockOutbound)
	if err != nil {
		return nil, err
	}

	whitelist, err := uc.setting.GetSlice(SettingKeyScanAwareWhitelist)
	if err != nil {
		return nil, err
	}

	return &ScanSetting{
		Enabled:        enabled,
		Days:           uint(days),
		Interfaces:     interfaces,
		AutoBlock:      autoBlock,
		BlockThreshold: uint(blockThreshold),
		BlockWindow:    uint(blockWindow),
		BlockDuration:  uint(blockDuration),
		BlockOutbound:  blockOutbound,
		Whitelist:      whitelist,
	}, nil
}

func (uc *ScanEventUsecase) UpdateSetting(ctx context.Context, setting *ScanSetting) error {
	if err := uc.setting.Set(SettingKeyScanAware, cast.ToString(setting.Enabled)); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyScanAwareDays, cast.ToString(setting.Days)); err != nil {
		return err
	}

	interfacesJSON, err := json.Marshal(setting.Interfaces)
	if err != nil {
		return err
	}
	if err = uc.setting.Set(SettingKeyScanAwareInterfaces, string(interfacesJSON)); err != nil {
		return err
	}

	if err = uc.setting.Set(SettingKeyScanAwareAutoBlock, cast.ToString(setting.AutoBlock)); err != nil {
		return err
	}
	if err = uc.setting.Set(SettingKeyScanAwareBlockThreshold, cast.ToString(setting.BlockThreshold)); err != nil {
		return err
	}
	if err = uc.setting.Set(SettingKeyScanAwareBlockWindow, cast.ToString(setting.BlockWindow)); err != nil {
		return err
	}
	if err = uc.setting.Set(SettingKeyScanAwareBlockDuration, cast.ToString(setting.BlockDuration)); err != nil {
		return err
	}
	if err = uc.setting.Set(SettingKeyScanAwareBlockOutbound, cast.ToString(setting.BlockOutbound)); err != nil {
		return err
	}

	if err = uc.setting.SetSlice(SettingKeyScanAwareWhitelist, setting.Whitelist); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("scan aware setting updated"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)), slog.Bool("enabled", setting.Enabled), slog.Bool("auto_block", setting.AutoBlock), slog.Uint64("days", uint64(setting.Days)), slog.Uint64("block_threshold", uint64(setting.BlockThreshold)), slog.Uint64("block_window", uint64(setting.BlockWindow)), slog.Uint64("block_duration", uint64(setting.BlockDuration)), slog.Bool("block_outbound", setting.BlockOutbound), slog.Any("whitelist", setting.Whitelist))
	return nil
}

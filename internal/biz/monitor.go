package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type Monitor struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	Info      types.CurrentInfo `gorm:"not null;default:'{}';serializer:zstd" json:"info"`
	CreatedAt time.Time         `gorm:"index:idx_monitors_created_at" json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type MonitorRepo interface {
	Create(monitor *Monitor) error
	ClearBefore(t time.Time) error
	Clear() error
	List(start, end time.Time) ([]*Monitor, error)
	VacuumDB() error
}

type MonitorUsecase struct {
	t       *gotext.Locale
	log     *slog.Logger
	repo    MonitorRepo
	setting SettingRepo
}

func NewMonitorUsecase(t *gotext.Locale, log *slog.Logger, repo MonitorRepo, setting SettingRepo) *MonitorUsecase {
	return &MonitorUsecase{t: t, log: log, repo: repo, setting: setting}
}

func (uc *MonitorUsecase) GetSetting(ctx context.Context) (*request.MonitorSetting, error) {
	monitor, err := uc.setting.Get(SettingKeyMonitor)
	if err != nil {
		return nil, err
	}
	monitorDays, err := uc.setting.Get(SettingKeyMonitorDays)
	if err != nil {
		return nil, err
	}
	monitorInterval, err := uc.setting.GetInt(SettingKeyMonitorInterval, 1)
	if err != nil {
		return nil, err
	}
	alertDays, err := uc.setting.GetInt(SettingKeyAlertLogDays, 30)
	if err != nil {
		return nil, err
	}

	setting := new(request.MonitorSetting)
	setting.Enabled = cast.ToBool(monitor)
	setting.Days = cast.ToUint(monitorDays)
	setting.Interval = uint(monitorInterval)
	setting.AlertDays = uint(alertDays)

	uc.log.Info(uc.t.Get("monitor setting viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)))

	return setting, nil
}

func (uc *MonitorUsecase) UpdateSetting(ctx context.Context, setting *request.MonitorSetting) error {
	if err := uc.setting.Set(SettingKeyMonitor, cast.ToString(setting.Enabled)); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyMonitorDays, cast.ToString(setting.Days)); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyMonitorInterval, cast.ToString(setting.Interval)); err != nil {
		return err
	}

	if err := uc.setting.Set(SettingKeyAlertLogDays, cast.ToString(setting.AlertDays)); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("monitor setting updated"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Bool("enabled", setting.Enabled), slog.Uint64("days", uint64(setting.Days)), slog.Uint64("interval", uint64(setting.Interval)), slog.Uint64("alert_days", uint64(setting.AlertDays)))

	return nil
}

func (uc *MonitorUsecase) Clear(ctx context.Context) error {
	if err := uc.repo.Clear(); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("monitor data cleared"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)))

	return nil
}

func (uc *MonitorUsecase) Create(monitor *Monitor) error {
	return uc.repo.Create(monitor)
}

func (uc *MonitorUsecase) ClearBefore(t time.Time) error {
	return uc.repo.ClearBefore(t)
}

func (uc *MonitorUsecase) List(start, end time.Time) ([]*Monitor, error) {
	return uc.repo.List(start, end)
}

func (uc *MonitorUsecase) VacuumDB() error {
	return uc.repo.VacuumDB()
}

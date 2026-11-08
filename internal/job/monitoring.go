package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/tools"
)

// Monitoring 系统监控
type Monitoring struct {
	log         *slog.Logger
	monitorRepo *biz.MonitorUsecase
	settingRepo *biz.SettingUsecase
	t           *gotext.Locale
	lastRun     time.Time
	cleanedAt   time.Time
}

// NewMonitoring 构造系统监控任务
func NewMonitoring(settingUsecase *biz.SettingUsecase, monitorUsecase *biz.MonitorUsecase, t *gotext.Locale, log *slog.Logger) Job {
	return Job{
		Spec: "* * * * *",
		Task: &Monitoring{
			log:         log,
			monitorRepo: monitorUsecase,
			settingRepo: settingUsecase,
			t:           t,
		},
	}
}

func (r *Monitoring) Run(_ context.Context) error {
	if app.Status != app.StatusNormal {
		return nil
	}

	monitor, err := r.settingRepo.Get(biz.SettingKeyMonitor)
	if err != nil || !cast.ToBool(monitor) {
		return nil //nolint:nilerr
	}

	// 根据采集间隔判断是否该采集
	interval, _ := r.settingRepo.GetInt(biz.SettingKeyMonitorInterval, 1)
	if interval < 1 {
		interval = 1
	}
	if !r.lastRun.IsZero() && time.Since(r.lastRun) < time.Duration(interval)*time.Minute-30*time.Second {
		return nil
	}
	r.lastRun = time.Now()

	info := tools.CurrentInfo(nil, nil)
	info.TopProcesses = tools.CollectTopProcesses()

	// 去除部分数据以减少数据库存储
	info.Disk = nil
	info.Cpus = nil

	if app.Status != app.StatusNormal {
		return nil
	}

	if err = r.monitorRepo.Create(&biz.Monitor{Info: info}); err != nil {
		r.log.Warn(r.t.Get("failed to create monitor record"), slog.String("type", biz.OperationTypeMonitor), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return nil
	}

	// 删除过期数据，按天过期故限流到 6 小时一次，避免每分钟一次 DELETE
	if time.Since(r.cleanedAt) < 6*time.Hour {
		return nil
	}
	r.cleanedAt = time.Now()

	dayStr, err := r.settingRepo.Get(biz.SettingKeyMonitorDays)
	if err != nil {
		return nil //nolint:nilerr
	}
	day := cast.ToInt(dayStr)
	if day <= 0 || app.Status != app.StatusNormal {
		return nil
	}
	if err = r.monitorRepo.ClearBefore(time.Now().AddDate(0, 0, -day)); err != nil {
		r.log.Warn(r.t.Get("failed to delete monitor record"), slog.String("type", biz.OperationTypeMonitor), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return nil
	}
	return nil
}

package job

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/tools"
)

// PanelTask 面板每日任务
type PanelTask struct {
	db          *gorm.DB
	log         *slog.Logger
	backupRepo  *biz.BackupUsecase
	cacheRepo   *biz.CacheUsecase
	taskRepo    *biz.TaskUsecase
	settingRepo *biz.SettingUsecase
	monitorRepo *biz.MonitorUsecase
	scanRepo    *biz.ScanEventUsecase
	statRepo    *biz.WebsiteStatUsecase
	tamperRepo  *biz.TamperUsecase
	t           *gotext.Locale
}

// NewPanelTask 构造面板每日任务
func NewPanelTask(
	backupUsecase *biz.BackupUsecase,
	cacheUsecase *biz.CacheUsecase,
	monitorUsecase *biz.MonitorUsecase,
	scanEventUsecase *biz.ScanEventUsecase,
	settingUsecase *biz.SettingUsecase,
	tamperUsecase *biz.TamperUsecase,
	taskUsecase *biz.TaskUsecase,
	websiteStatUsecase *biz.WebsiteStatUsecase,
	db *gorm.DB,
	t *gotext.Locale,
	log *slog.Logger,
) Job {
	return Job{
		Spec: "0 2 * * *",
		// 维护状态由本任务自己进出
		AnyStatus: true,
		Task: &PanelTask{
			db:          db,
			log:         log,
			backupRepo:  backupUsecase,
			cacheRepo:   cacheUsecase,
			taskRepo:    taskUsecase,
			settingRepo: settingUsecase,
			monitorRepo: monitorUsecase,
			scanRepo:    scanEventUsecase,
			statRepo:    websiteStatUsecase,
			tamperRepo:  tamperUsecase,
			t:           t,
		},
	}
}

func (r *PanelTask) Run(ctx context.Context) error {
	r.maintain(ctx)

	if offline, err := r.settingRepo.GetBool(biz.SettingKeyOfflineMode); err == nil && !offline {
		r.updateIPDB(ctx)
		r.updateCaches()
		if autoUpdate, err := r.settingRepo.GetBool(biz.SettingKeyAutoUpdate); err == nil && autoUpdate {
			r.updatePanel(ctx)
		}
	}

	runtime.GC()
	debug.FreeOSMemory()

	return nil
}

// maintain 在维护状态下整理数据库并备份面板
func (r *PanelTask) maintain(ctx context.Context) {
	// healthKeyPanelDB 面板主数据库健康问题上报 key
	const healthKeyPanelDB = "database:panel"

	// 升级中不进维护，否则收尾时会把升级状态冲回正常
	if !app.Status.CompareAndSwap(app.StatusNormal, app.StatusMaintain) {
		return
	}
	// 中途 panic 也要恢复，否则面板会一直停在维护状态
	defer app.Status.Store(app.StatusNormal)

	// 优化主数据库，失败只上报，磁盘满时用户还要进面板清理
	checkpointErr := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
	if checkpointErr != nil {
		r.log.Warn(r.t.Get("failed to wal checkpoint database"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", checkpointErr))
	}
	vacuumErr := r.db.Exec("VACUUM").Error
	if vacuumErr != nil {
		r.log.Warn(r.t.Get("failed to vacuum database"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", vacuumErr))
	}
	if err := errors.Join(checkpointErr, vacuumErr); err != nil {
		app.Health.Report(healthKeyPanelDB, app.HealthLevelError, err.Error())
	} else {
		app.Health.Clear(healthKeyPanelDB)
	}
	// 优化辅助数据库
	if err := r.scanRepo.VacuumDB(); err != nil {
		r.log.Warn(r.t.Get("failed to vacuum scan event database"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}
	if err := r.statRepo.VacuumDB(); err != nil {
		r.log.Warn(r.t.Get("failed to vacuum website stat database"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}
	if err := r.tamperRepo.VacuumDB(); err != nil {
		r.log.Warn(r.t.Get("failed to vacuum tamper database"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}
	if err := r.monitorRepo.VacuumDB(); err != nil {
		r.log.Warn(r.t.Get("failed to vacuum monitor database"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}

	// 备份面板
	if err := r.backupRepo.CreatePanel(ctx); err != nil {
		r.log.Warn(r.t.Get("failed to backup panel"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}

	// 清理备份
	if err := r.backupRepo.ClearExpired(r.backupRepo.GetDefaultPath(biz.BackupTypePanel), "panel_", 10); err != nil {
		r.log.Warn(r.t.Get("failed to clear backup"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}
}

// updateIPDB 更新 IPDB 订阅文件
func (r *PanelTask) updateIPDB(ctx context.Context) {
	// 文件已存在时每周五更新，不存在则立即下载
	destPath := filepath.Join(app.Root, "panel/storage/geo.ipdb")
	if _, err := os.Stat(destPath); err == nil && time.Now().Weekday() != time.Friday {
		return
	}

	ipdbType, _ := r.settingRepo.Get(biz.SettingKeyIPDBType)
	if ipdbType != "subscribe" {
		return
	}
	ipdbURL, _ := r.settingRepo.Get(biz.SettingKeyIPDBURL)
	if ipdbURL == "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := tools.DownloadFile(ctx, ipdbURL, destPath); err != nil {
		r.log.Warn(r.t.Get("failed to download ipdb"), slog.String("url", ipdbURL), slog.Any("err", err))
		return
	}

	r.log.Info(r.t.Get("ipdb updated"), slog.String("url", ipdbURL), slog.String("path", destPath))
}

// updateCaches 错峰同步云端数据
func (r *PanelTask) updateCaches() {
	failed := map[biz.CacheKey]string{
		biz.CacheKeyCategories:  r.t.Get("failed to update categories cache"),
		biz.CacheKeyApps:        r.t.Get("failed to update apps cache"),
		biz.CacheKeyEnvironment: r.t.Get("failed to update environment cache"),
		biz.CacheKeyTemplates:   r.t.Get("failed to update template cache"),
	}

	for _, update := range r.cacheRepo.Updates() {
		time.AfterFunc(time.Duration(rand.IntN(300))*time.Second, func() { //nolint:gosec
			if err := update.Run(); err != nil {
				r.log.Warn(failed[update.Key], slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
			}
		})
	}
}

// updatePanel 更新面板
func (r *PanelTask) updatePanel(ctx context.Context) {
	// 延后触发且要熬过面板自杀重启，绑可取消 ctx 会让更新与重启中途被杀
	ctx = context.WithoutCancel(ctx)

	// 加 360 秒确保在缓存更新后才更新面板
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second+360*time.Second, func() { //nolint:gosec
		// 到真正更新时再查，重启会杀掉这几分钟里新起的后台任务
		if r.taskRepo.HasRunningTask() {
			return
		}
		release, err := r.backupRepo.LatestPanel()
		if err != nil || release.URL == "" {
			return
		}
		if newer, err := release.Newer(); err != nil || !newer {
			return
		}
		if err = r.backupRepo.UpdatePanel(ctx, release, func(msg string) {
			r.log.Debug(r.t.Get("panel updating"), slog.String("type", biz.OperationTypePanel), slog.String("progress", msg))
		}); err != nil {
			r.log.Warn(r.t.Get("failed to update panel"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}
		// 成功后由本入口负责重启
		tools.RestartPanel(ctx)
	})
}

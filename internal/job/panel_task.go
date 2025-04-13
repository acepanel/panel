package job

import (
	"log/slog"
	"math/rand/v2"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/go-rat/utils/collect"
	"github.com/hashicorp/go-version"
	"gorm.io/gorm"

	"github.com/tnb-labs/panel/internal/app"
	"github.com/tnb-labs/panel/internal/biz"
	"github.com/tnb-labs/panel/pkg/api"
)

// PanelTask 面板每日任务
type PanelTask struct {
	api         *api.API
	db          *gorm.DB
	log         *slog.Logger
	backupRepo  biz.BackupRepo
	cacheRepo   biz.CacheRepo
	taskRepo    biz.TaskRepo
	settingRepo biz.SettingRepo
}

func NewPanelTask(db *gorm.DB, log *slog.Logger, backup biz.BackupRepo, cache biz.CacheRepo, task biz.TaskRepo, setting biz.SettingRepo) *PanelTask {
	return &PanelTask{
		api:         api.NewAPI(app.Version, app.Locale),
		db:          db,
		log:         log,
		backupRepo:  backup,
		cacheRepo:   cache,
		taskRepo:    task,
		settingRepo: setting,
	}
}

func (r *PanelTask) Run() {
	app.Status = app.StatusMaintain

	// 优化数据库
	if err := r.db.Exec("VACUUM").Error; err != nil {
		app.Status = app.StatusFailed
		r.log.Warn("[Panel Task] failed to vacuum database", slog.Any("err", err))
		return
	}
	if err := r.db.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		app.Status = app.StatusFailed
		r.log.Warn("[Panel Task] failed to set database journal_mode to WAL", slog.Any("err", err))
		return
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		app.Status = app.StatusFailed
		r.log.Warn("[Panel Task] failed to wal checkpoint database", slog.Any("err", err))
		return
	}

	// 备份面板
	if err := r.backupRepo.Create(biz.BackupTypePanel, ""); err != nil {
		r.log.Warn("备份面板失败", slog.Any("err", err))
	}

	// 清理备份
	if path, err := r.backupRepo.GetPath("panel"); err == nil {
		if err = r.backupRepo.ClearExpired(path, "panel_", 10); err != nil {
			r.log.Warn("[Panel Task] failed to clear backup", slog.Any("err", err))
		}
	}

	// 非离线模式下任务
	if offline, err := r.settingRepo.GetBool(biz.SettingKeyOfflineMode); err == nil && !offline {
		r.updateApps()
		r.updateRewrites()
		if autoUpdate, err := r.settingRepo.GetBool(biz.SettingKeyAutoUpdate); err == nil && autoUpdate {
			r.updatePanel()
		}
	}

	// 回收内存
	runtime.GC()
	debug.FreeOSMemory()

	app.Status = app.StatusNormal
}

// 更新商店缓存
func (r *PanelTask) updateApps() {
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second, func() {
		if err := r.cacheRepo.UpdateApps(); err != nil {
			r.log.Warn("[Panel Task] failed to update apps cache", slog.Any("err", err))
		}
	})
}

// 更新伪静态缓存
func (r *PanelTask) updateRewrites() {
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second, func() {
		if err := r.cacheRepo.UpdateRewrites(); err != nil {
			r.log.Warn("[Panel Task] failed to update rewrites cache", slog.Any("err", err))
		}
	})
}

// 更新面板
func (r *PanelTask) updatePanel() {
	if r.taskRepo.HasRunningTask() {
		return
	}

	// 加 300 秒确保在缓存更新后才更新面板
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second+300*time.Second, func() {
		panel, err := r.api.LatestVersion()
		if err != nil {
			return
		}
		current, err := version.NewVersion(app.Version)
		if err != nil {
			return
		}
		latest, err := version.NewVersion(panel.Version)
		if err != nil {
			return
		}
		if current.GreaterThanOrEqual(latest) {
			return
		}
		if download := collect.First(panel.Downloads); download != nil {
			if err = r.backupRepo.UpdatePanel(panel.Version, download.URL, download.Checksum); err != nil {
				r.log.Warn("[Panel Task] failed to update panel", slog.Any("err", err))
				_ = r.backupRepo.FixPanel()
			}
		}
	})
}

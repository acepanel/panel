package job

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/acepanel/panel/pkg/config"
	"github.com/hashicorp/go-version"
	"github.com/libtnb/utils/collect"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/pkg/api"
)

// PanelTask 面板每日任务
type PanelTask struct {
	api         *api.API
	conf        *config.Config
	db          *gorm.DB
	log         *slog.Logger
	backupRepo  biz.BackupRepo
	cacheRepo   biz.CacheRepo
	taskRepo    biz.TaskRepo
	settingRepo biz.SettingRepo
}

func NewPanelTask(conf *config.Config, db *gorm.DB, log *slog.Logger, backup biz.BackupRepo, cache biz.CacheRepo, task biz.TaskRepo, setting biz.SettingRepo) *PanelTask {
	return &PanelTask{
		api:         api.NewAPI(app.Version, app.Locale),
		conf:        conf,
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
		r.log.Warn("failed to vacuum database", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return
	}
	if err := r.db.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		app.Status = app.StatusFailed
		r.log.Warn("failed to set database journal_mode to WAL", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		app.Status = app.StatusFailed
		r.log.Warn("failed to wal checkpoint database", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return
	}

	// 备份面板
	if err := r.backupRepo.Create(biz.BackupTypePanel, ""); err != nil {
		r.log.Warn("failed to backup panel", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}

	// 清理备份
	if path, err := r.backupRepo.GetPath("panel"); err == nil {
		if err = r.backupRepo.ClearExpired(path, "panel_", 10); err != nil {
			r.log.Warn("failed to clear backup", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
		}
	}

	// 非离线模式下任务
	if offline, err := r.settingRepo.GetBool(biz.SettingKeyOfflineMode); err == nil && !offline {
		r.updateCategories()
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

// 更新分类缓存
func (r *PanelTask) updateCategories() {
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second, func() {
		if err := r.cacheRepo.UpdateCategories(); err != nil {
			r.log.Warn("failed to update categories cache", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
		}
	})
}

// 更新商店缓存
func (r *PanelTask) updateApps() {
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second, func() {
		if err := r.cacheRepo.UpdateApps(); err != nil {
			r.log.Warn("failed to update apps cache", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
		}
	})
}

// 更新伪静态缓存
func (r *PanelTask) updateRewrites() {
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second, func() {
		if err := r.cacheRepo.UpdateRewrites(); err != nil {
			r.log.Warn("failed to update rewrites cache", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
		}
	})
}

// 更新面板
func (r *PanelTask) updatePanel() {
	if r.taskRepo.HasRunningTask() {
		return
	}

	channel, _ := r.settingRepo.Get(biz.SettingKeyChannel)

	// 加 300 秒确保在缓存更新后才更新面板
	time.AfterFunc(time.Duration(rand.IntN(300))*time.Second+300*time.Second, func() {
		panel, err := r.api.LatestVersion(channel)
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
			url := fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, download.URL)
			checksum := fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, download.Checksum)
			if err = r.backupRepo.UpdatePanel(panel.Version, url, checksum); err != nil {
				r.log.Warn("failed to update panel", slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", 0), slog.Any("err", err))
				_ = r.backupRepo.FixPanel()
			}
		}
	})
}

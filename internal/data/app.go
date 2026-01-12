package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/expr-lang/expr"
	"github.com/hashicorp/go-version"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/pkg/api"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/types"
)

type appRepo struct {
	t     *gotext.Locale
	conf  *config.Config
	db    *gorm.DB
	log   *slog.Logger
	cache biz.CacheRepo
	task  biz.TaskRepo
	api   *api.API
}

func NewAppRepo(t *gotext.Locale, conf *config.Config, db *gorm.DB, log *slog.Logger, cache biz.CacheRepo, task biz.TaskRepo) biz.AppRepo {
	return &appRepo{
		t:     t,
		conf:  conf,
		db:    db,
		log:   log,
		cache: cache,
		task:  task,
		api:   api.NewAPI(app.Version, app.Locale),
	}
}

func (r *appRepo) Categories() []types.LV {
	cached, err := r.cache.Get(biz.CacheKeyCategories)
	if err != nil {
		return nil
	}

	var categories api.Categories
	if err = json.Unmarshal([]byte(cached), &categories); err != nil {
		return nil
	}

	slices.SortFunc(categories, func(a, b *api.Category) int {
		return a.Order - b.Order
	})

	result := make([]types.LV, 0)
	for item := range slices.Values(categories) {
		result = append(result, types.LV{
			Label: item.Name,
			Value: item.Slug,
		})
	}

	return result
}

func (r *appRepo) All() api.Apps {
	cached, err := r.cache.Get(biz.CacheKeyApps)
	if err != nil {
		return nil
	}
	var apps api.Apps
	if err = json.Unmarshal([]byte(cached), &apps); err != nil {
		return nil
	}
	return apps
}

func (r *appRepo) Get(slug string) (*api.App, error) {
	for item := range slices.Values(r.All()) {
		if item.Slug == slug {
			return item, nil
		}
	}
	return nil, errors.New(r.t.Get("app %s not found", slug))
}

func (r *appRepo) UpdateExist(slug string) bool {
	item, err := r.Get(slug)
	if err != nil {
		return false
	}
	installed, err := r.GetInstalled(slug)
	if err != nil {
		return false
	}

	for channel := range slices.Values(item.Channels) {
		if channel.Slug == installed.Channel {
			if channel.Version != installed.Version {
				return true
			}
		}
	}

	return false
}

func (r *appRepo) Installed() ([]*biz.App, error) {
	var apps []*biz.App
	if err := r.db.Find(&apps).Error; err != nil {
		return nil, err
	}

	return apps, nil

}

func (r *appRepo) GetInstalled(slug string) (*biz.App, error) {
	installed := new(biz.App)
	if err := r.db.Where("slug = ?", slug).First(installed).Error; err != nil {
		return nil, err
	}

	return installed, nil
}

func (r *appRepo) GetInstalledAll(query string, cond ...string) ([]*biz.App, error) {
	var apps []*biz.App
	if err := r.db.Where(query, cond).Find(&apps).Error; err != nil {
		return nil, err
	}

	return apps, nil
}

func (r *appRepo) GetHomeShow() ([]map[string]string, error) {
	var apps []*biz.App
	if err := r.db.Where("show = ?", true).Order("show_order").Find(&apps).Error; err != nil {
		return nil, err
	}

	filtered := make([]map[string]string, 0)
	for item := range slices.Values(apps) {
		loaded, err := r.Get(item.Slug)
		if err != nil {
			continue
		}
		filtered = append(filtered, map[string]string{
			"name":        loaded.Name,
			"description": loaded.Description,
			"slug":        loaded.Slug,
			"version":     item.Version,
		})
	}

	return filtered, nil
}

func (r *appRepo) IsInstalled(query string, cond ...any) (bool, error) {
	var count int64
	if len(cond) == 0 {
		if err := r.db.Model(&biz.App{}).Where("slug = ?", query).Count(&count).Error; err != nil {
			return false, err
		}
	} else {
		if err := r.db.Model(&biz.App{}).Where(query, cond...).Count(&count).Error; err != nil {
			return false, err
		}
	}

	return count > 0, nil
}

func (r *appRepo) Install(channel, slug string) error {
	item, err := r.Get(slug)
	if err != nil {
		return err
	}
	panel, err := version.NewVersion(app.Version)
	if err != nil {
		return err
	}

	if installed, _ := r.IsInstalled(slug); installed {
		return errors.New(r.t.Get("app %s already installed", slug))
	}

	shellUrl, shellChannel, shellVersion := "", "", ""
	for ch := range slices.Values(item.Channels) {
		vs, err := version.NewVersion(ch.Panel)
		if err != nil {
			continue
		}
		if ch.Slug == channel {
			if vs.GreaterThan(panel) && !r.conf.App.Debug {
				return errors.New(r.t.Get("app %s requires panel version %s, current version %s", item.Name, ch.Panel, app.Version))
			}
			shellUrl = fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, ch.Install)
			shellChannel = ch.Slug
			shellVersion = ch.Version
			break
		}
	}
	if shellUrl == "" {
		return errors.New(r.t.Get("app %s not support current panel version", item.Name))
	}

	if err = r.preCheck(item); err != nil {
		return err
	}

	// 下载回调
	if err = r.api.AppCallback(slug); err != nil {
		r.log.Warn("download callback failed", slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", 0), slog.String("app", slug), slog.Any("err", err))
	}

	if app.IsCli {
		return shell.ExecfWithOutput(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s"`, shellUrl, shellChannel, shellVersion)
	}

	task := new(biz.Task)
	task.Name = r.t.Get("Install app %s", item.Name)
	task.Status = biz.TaskStatusWaiting
	task.Shell = fmt.Sprintf(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s" >> /tmp/%s.log 2>&1`, shellUrl, shellChannel, shellVersion, item.Slug)
	task.Log = "/tmp/" + item.Slug + ".log"

	return r.task.Push(task)
}

func (r *appRepo) UnInstall(slug string) error {
	item, err := r.Get(slug)
	if err != nil {
		return err
	}
	panel, err := version.NewVersion(app.Version)
	if err != nil {
		return err
	}

	if installed, _ := r.IsInstalled(slug); !installed {
		return errors.New(r.t.Get("app %s not installed", item.Name))
	}
	installed, err := r.GetInstalled(slug)
	if err != nil {
		return err
	}

	shellUrl, shellChannel, shellVersion := "", "", ""
	for ch := range slices.Values(item.Channels) {
		vs, err := version.NewVersion(ch.Panel)
		if err != nil {
			continue
		}
		if ch.Slug == installed.Channel {
			if vs.GreaterThan(panel) && !r.conf.App.Debug {
				return errors.New(r.t.Get("app %s requires panel version %s, current version %s", item.Name, ch.Panel, app.Version))
			}
			shellUrl = fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, ch.Uninstall)
			shellChannel = ch.Slug
			shellVersion = installed.Version
			break
		}
	}
	if shellUrl == "" {
		return errors.New(r.t.Get("failed to get uninstall script for app %s", item.Name))
	}

	if err = r.preCheck(item); err != nil {
		return err
	}

	if app.IsCli {
		return shell.ExecfWithOutput(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s"`, shellUrl, shellChannel, shellVersion)
	}

	task := new(biz.Task)
	task.Name = r.t.Get("Uninstall app %s", item.Name)
	task.Status = biz.TaskStatusWaiting
	task.Shell = fmt.Sprintf(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s" >> /tmp/%s.log 2>&1`, shellUrl, shellChannel, shellVersion, item.Slug)
	task.Log = "/tmp/" + item.Slug + ".log"

	return r.task.Push(task)
}

func (r *appRepo) Update(slug string) error {
	item, err := r.Get(slug)
	if err != nil {
		return err
	}
	panel, err := version.NewVersion(app.Version)
	if err != nil {
		return err
	}

	if installed, _ := r.IsInstalled(slug); !installed {
		return errors.New(r.t.Get("app %s not installed", item.Name))
	}
	installed, err := r.GetInstalled(slug)
	if err != nil {
		return err
	}

	shellUrl, shellChannel, shellVersion := "", "", ""
	for ch := range slices.Values(item.Channels) {
		vs, err := version.NewVersion(ch.Panel)
		if err != nil {
			continue
		}
		if ch.Slug == installed.Channel {
			if vs.GreaterThan(panel) && !r.conf.App.Debug {
				return errors.New(r.t.Get("app %s requires panel version %s, current version %s", item.Name, ch.Panel, app.Version))
			}
			shellUrl = fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, ch.Update)
			shellChannel = ch.Slug
			shellVersion = ch.Version
			break
		}
	}
	if shellUrl == "" {
		return errors.New(r.t.Get("app %s not support current panel version", item.Name))
	}

	if err = r.preCheck(item); err != nil {
		return err
	}

	// 下载回调
	if err = r.api.AppCallback(slug); err != nil {
		r.log.Warn("download callback failed", slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", 0), slog.String("app", slug), slog.Any("err", err))
	}

	if app.IsCli {
		return shell.ExecfWithOutput(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s"`, shellUrl, shellChannel, shellVersion)
	}

	task := new(biz.Task)
	task.Name = r.t.Get("Update app %s", item.Name)
	task.Status = biz.TaskStatusWaiting
	task.Shell = fmt.Sprintf(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s" >> /tmp/%s.log 2>&1`, shellUrl, shellChannel, shellVersion, item.Slug)
	task.Log = "/tmp/" + item.Slug + ".log"

	return r.task.Push(task)
}

func (r *appRepo) UpdateShow(slug string, show bool) error {
	item, err := r.GetInstalled(slug)
	if err != nil {
		return err
	}

	item.Show = show

	return r.db.Save(item).Error
}

func (r *appRepo) preCheck(app *api.App) error {
	var apps []string
	var installed []string

	all := r.All()
	for _, item := range all {
		apps = append(apps, item.Slug)
	}
	installedApps, err := r.Installed()
	if err != nil {
		return err
	}
	for _, item := range installedApps {
		installed = append(installed, item.Slug)
	}

	env := map[string]any{
		"apps":      apps,
		"installed": installed,
	}
	output, err := expr.Eval(app.Depends, env)
	if err != nil {
		return err
	}

	result := cast.ToString(output)
	if result != "ok" {
		return errors.New(r.t.Get("App %s %s", app.Name, result))
	}

	return nil
}

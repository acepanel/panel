package biz

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Slug      string    `gorm:"not null;default:'';unique" json:"slug"`
	Channel   string    `gorm:"not null;default:''" json:"channel"`
	Version   string    `gorm:"not null;default:''" json:"version"`
	Show      bool      `gorm:"not null;default:false" json:"show"`
	ShowOrder int       `gorm:"not null;default:0" json:"show_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AppCustom 自定义编译参数(前置脚本在 configure 前执行,参数追加到 configure 末尾)
type AppCustom struct {
	PreScript string `json:"pre_script"`
	Args      string `json:"args"`
}

// 支持自定义编译参数的应用与环境类型(源码编译类)
var (
	customCompileApps     = []string{"apache", "memcached", "nginx", "openresty", "pureftpd", "s3fs"}
	customCompileEnvTypes = []string{"php", "python"}
)

// CustomCompileApp 判断应用是否支持自定义编译参数
func CustomCompileApp(slug string) bool {
	return slices.Contains(customCompileApps, slug)
}

// CustomCompileEnv 判断环境类型是否支持自定义编译参数
func CustomCompileEnv(typ string) bool {
	return slices.Contains(customCompileEnvTypes, typ)
}

type AppRepo interface {
	Installed() ([]*App, error)
	GetInstalled(slug string) (*App, error)
	GetInstalledAll(query string, cond ...string) ([]*App, error)
	ListHomeShow() ([]*App, error)
	IsInstalled(query string, cond ...any) (bool, error)
	UpdateShow(slug string, show bool) error
	UpdateOrder(slugs []string) error
	DownloadCallback(slug string)
	CheckPanelVersion() error
	ResolveScript(item *api.App, matchChannel, action, execVersion string) (string, error)
	ExecScript(ctx context.Context, script string) error
	PreCheck(item *api.App, catalog api.Apps) error
	GetCustom(slug string) (*AppCustom, error)
	SaveCustom(ctx context.Context, slug string, custom *AppCustom) error
}

type AppUsecase struct {
	repo  AppRepo
	cache CacheRepo
	task  TaskRepo
	t     *gotext.Locale
	log   *slog.Logger
}

func NewAppUsecase(t *gotext.Locale, log *slog.Logger, appRepo AppRepo, cacheRepo CacheRepo, taskRepo TaskRepo) *AppUsecase {
	return &AppUsecase{
		repo:  appRepo,
		cache: cacheRepo,
		task:  taskRepo,
		t:     t,
		log:   log,
	}
}

func (uc *AppUsecase) Categories() []types.LV {
	cached, err := uc.cache.Get(CacheKeyCategories)
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

	return lo.Map(categories, func(item *api.Category, _ int) types.LV {
		return types.LV{Label: item.Name, Value: item.Slug}
	})
}

func (uc *AppUsecase) All() api.Apps {
	cached, err := uc.cache.Get(CacheKeyApps)
	if err != nil {
		return nil
	}
	var apps api.Apps
	if err = json.Unmarshal([]byte(cached), &apps); err != nil {
		return nil
	}
	return apps
}

func (uc *AppUsecase) Get(slug string) (*api.App, error) {
	for item := range slices.Values(uc.All()) {
		if item.Slug == slug {
			return item, nil
		}
	}
	return nil, errors.New(uc.t.Get("app %s not found", slug))
}

func (uc *AppUsecase) UpdateExist(slug string) bool {
	item, err := uc.Get(slug)
	if err != nil {
		return false
	}
	installed, err := uc.repo.GetInstalled(slug)
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

func (uc *AppUsecase) Installed() ([]*App, error) {
	return uc.repo.Installed()
}

func (uc *AppUsecase) GetInstalled(slug string) (*App, error) {
	return uc.repo.GetInstalled(slug)
}

func (uc *AppUsecase) GetInstalledAll(query string, cond ...string) ([]*App, error) {
	return uc.repo.GetInstalledAll(query, cond...)
}

func (uc *AppUsecase) GetHomeShow() ([]map[string]string, error) {
	apps, err := uc.repo.ListHomeShow()
	if err != nil {
		return nil, err
	}

	// 目录只加载一次
	catalog := lo.KeyBy(uc.All(), func(item *api.App) string { return item.Slug })

	filtered := make([]map[string]string, 0)
	for item := range slices.Values(apps) {
		loaded, ok := catalog[item.Slug]
		if !ok {
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

func (uc *AppUsecase) IsInstalled(query string, cond ...any) (bool, error) {
	return uc.repo.IsInstalled(query, cond...)
}

func (uc *AppUsecase) Install(ctx context.Context, channel, slug string) error {
	item, err := uc.Get(slug)
	if err != nil {
		return err
	}

	// 恢复原编排：非法面板版本应在存在性检查前报错
	if err = uc.repo.CheckPanelVersion(); err != nil {
		return err
	}

	if installed, _ := uc.repo.IsInstalled(slug); installed {
		return errors.New(uc.t.Get("app %s already installed", slug))
	}

	script, err := uc.repo.ResolveScript(item, channel, "install", "")
	if err != nil {
		return err
	}

	if err = uc.repo.PreCheck(item, uc.All()); err != nil {
		return err
	}

	uc.repo.DownloadCallback(slug)

	if app.IsCli {
		err = uc.repo.ExecScript(ctx, script)
	} else {
		task := new(Task)
		task.Key = "app:" + slug
		task.Name = uc.t.Get("Install app %s", item.Name)
		task.Status = TaskStatusWaiting
		task.Shell = script
		err = uc.task.Push(task)
	}
	if err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("app installed"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("slug", slug), slog.String("name", item.Name), slog.String("channel", channel), slog.String("version", channelVersion(item, channel)))

	return nil
}

func (uc *AppUsecase) UnInstall(ctx context.Context, slug string) error {
	item, err := uc.Get(slug)
	if err != nil {
		return err
	}

	// 恢复原编排：非法面板版本应在存在性检查前报错
	if err = uc.repo.CheckPanelVersion(); err != nil {
		return err
	}

	if installed, _ := uc.repo.IsInstalled(slug); !installed {
		return errors.New(uc.t.Get("app %s not installed", item.Name))
	}
	installed, err := uc.repo.GetInstalled(slug)
	if err != nil {
		return err
	}

	script, err := uc.repo.ResolveScript(item, installed.Channel, "uninstall", installed.Version)
	if err != nil {
		return err
	}

	if err = uc.repo.PreCheck(item, uc.All()); err != nil {
		return err
	}

	if app.IsCli {
		err = uc.repo.ExecScript(ctx, script)
	} else {
		task := new(Task)
		task.Key = "app:" + slug
		task.Name = uc.t.Get("Uninstall app %s", item.Name)
		task.Status = TaskStatusWaiting
		task.Shell = script
		err = uc.task.Push(task)
	}
	if err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("app uninstalled"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(installed.ID)), slog.String("slug", slug), slog.String("name", item.Name), slog.String("channel", installed.Channel), slog.String("version", installed.Version))

	return nil
}

func (uc *AppUsecase) Update(ctx context.Context, slug string) error {
	item, err := uc.Get(slug)
	if err != nil {
		return err
	}

	// 恢复原编排：非法面板版本应在存在性检查前报错
	if err = uc.repo.CheckPanelVersion(); err != nil {
		return err
	}

	if installed, _ := uc.repo.IsInstalled(slug); !installed {
		return errors.New(uc.t.Get("app %s not installed", item.Name))
	}
	installed, err := uc.repo.GetInstalled(slug)
	if err != nil {
		return err
	}

	script, err := uc.repo.ResolveScript(item, installed.Channel, "update", "")
	if err != nil {
		return err
	}

	if err = uc.repo.PreCheck(item, uc.All()); err != nil {
		return err
	}

	uc.repo.DownloadCallback(slug)

	if app.IsCli {
		err = uc.repo.ExecScript(ctx, script)
	} else {
		task := new(Task)
		task.Key = "app:" + slug
		task.Name = uc.t.Get("Update app %s", item.Name)
		task.Status = TaskStatusWaiting
		task.Shell = script
		err = uc.task.Push(task)
	}
	if err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("app updated"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(installed.ID)), slog.String("slug", slug), slog.String("name", item.Name), slog.String("channel", installed.Channel), slog.String("from_version", installed.Version), slog.String("to_version", channelVersion(item, installed.Channel)))

	return nil
}

func (uc *AppUsecase) UpdateShow(ctx context.Context, slug string, show bool) error {
	if err := uc.repo.UpdateShow(slug, show); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("app home show updated"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("slug", slug), slog.Bool("show", show))

	return nil
}

func (uc *AppUsecase) GetCustom(slug string) (*AppCustom, error) {
	return uc.repo.GetCustom(slug)
}

func (uc *AppUsecase) SaveCustom(ctx context.Context, slug string, custom *AppCustom) error {
	if err := uc.repo.SaveCustom(ctx, slug, custom); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("app custom compile saved"), slog.String("type", lo.Ternary(CustomCompileApp(slug), OperationTypeApp, OperationTypeEnvironment)), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("slug", slug))

	return nil
}

func (uc *AppUsecase) UpdateOrder(slugs []string) error {
	return uc.repo.UpdateOrder(slugs)
}

func channelVersion(item *api.App, channel string) string {
	for ch := range slices.Values(item.Channels) {
		if ch.Slug == channel {
			return ch.Version
		}
	}
	return ""
}

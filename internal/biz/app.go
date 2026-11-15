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

type AppRepo interface {
	Installed() ([]*App, error)
	GetInstalled(slug string) (*App, error)
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

func NewAppUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	appRepo AppRepo,
	cacheRepo CacheRepo,
	taskRepo TaskRepo,
) *AppUsecase {
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

func (uc *AppUsecase) Installed() ([]*App, error) {
	return uc.repo.Installed()
}

func (uc *AppUsecase) GetInstalled(slug string) (*App, error) {
	return uc.repo.GetInstalled(slug)
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

func (uc *AppUsecase) UpdateShow(ctx context.Context, slug string, show bool) error {
	if err := uc.repo.UpdateShow(slug, show); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("app home show updated"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("slug", slug), slog.Bool("show", show))

	return nil
}

func (uc *AppUsecase) UpdateOrder(slugs []string) error {
	return uc.repo.UpdateOrder(slugs)
}

func (uc *AppUsecase) Install(ctx context.Context, channel, slug string) error {
	item, _, err := uc.do(ctx, slug, "install", channel)
	if err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("app installed"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("slug", slug), slog.String("name", item.Name), slog.String("channel", channel), slog.String("version", uc.channelVersion(item, channel)))

	return nil
}

func (uc *AppUsecase) UnInstall(ctx context.Context, slug string) error {
	item, installed, err := uc.do(ctx, slug, "uninstall", "")
	if err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("app uninstalled"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(installed.ID)), slog.String("slug", slug), slog.String("name", item.Name), slog.String("channel", installed.Channel), slog.String("version", installed.Version))

	return nil
}

func (uc *AppUsecase) Update(ctx context.Context, slug string) error {
	item, installed, err := uc.do(ctx, slug, "update", "")
	if err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("app updated"), slog.String("type", OperationTypeApp), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(installed.ID)), slog.String("slug", slug), slog.String("name", item.Name), slog.String("channel", installed.Channel), slog.String("from_version", installed.Version), slog.String("to_version", uc.channelVersion(item, installed.Channel)))

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

// do 安装时 channel 由调用方指定，卸载和更新沿用已安装的渠道
func (uc *AppUsecase) do(ctx context.Context, slug, action, channel string) (*api.App, *App, error) {
	item, err := uc.Get(slug)
	if err != nil {
		return nil, nil, err
	}

	// 恢复原编排：非法面板版本应在存在性检查前报错
	if err = uc.repo.CheckPanelVersion(); err != nil {
		return nil, nil, err
	}

	var installed *App
	var name, execVersion string
	exist, _ := uc.repo.IsInstalled(slug)
	if action == "install" {
		if exist {
			return nil, nil, errors.New(uc.t.Get("app %s already installed", slug))
		}
	} else {
		if !exist {
			return nil, nil, errors.New(uc.t.Get("app %s not installed", item.Name))
		}
		if installed, err = uc.repo.GetInstalled(slug); err != nil {
			return nil, nil, err
		}
		channel = installed.Channel
	}

	switch action {
	case "install":
		name = uc.t.Get("Install app %s", item.Name)
	case "uninstall":
		name = uc.t.Get("Uninstall app %s", item.Name)
		execVersion = installed.Version
	case "update":
		name = uc.t.Get("Update app %s", item.Name)
	}

	script, err := uc.repo.ResolveScript(item, channel, action, execVersion)
	if err != nil {
		return nil, nil, err
	}

	if err = uc.repo.PreCheck(item, uc.All()); err != nil {
		return nil, nil, err
	}

	if action != "uninstall" {
		uc.repo.DownloadCallback(slug)
	}

	if app.IsCli {
		err = uc.repo.ExecScript(ctx, script)
	} else {
		task := new(Task)
		task.Key = "app:" + slug
		task.Name = name
		task.Status = TaskStatusWaiting
		task.Shell = script
		err = uc.task.Push(task)
	}
	if err != nil {
		return nil, nil, err
	}

	return item, installed, nil
}

func (uc *AppUsecase) channelVersion(item *api.App, channel string) string {
	for ch := range slices.Values(item.Channels) {
		if ch.Slug == channel {
			return ch.Version
		}
	}
	return ""
}

// CustomCompileApp 判断应用是否支持自定义编译参数
func CustomCompileApp(slug string) bool {
	// 支持自定义编译参数的应用(源码编译类)
	return slices.Contains([]string{"apache", "memcached", "nginx", "openresty", "pureftpd", "s3fs"}, slug)
}

// CustomCompileEnv 判断环境类型是否支持自定义编译参数
func CustomCompileEnv(typ string) bool {
	// 支持自定义编译参数的环境类型(源码编译类)
	return slices.Contains([]string{"php", "python"}, typ)
}

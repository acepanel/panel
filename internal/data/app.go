package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/hashicorp/go-version"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type appRepo struct {
	t    *gotext.Locale
	conf *config.Config
	db   *gorm.DB
	log  *slog.Logger
	api  *api.API
}

func NewAppRepo(conf *config.Config, db *gorm.DB, t *gotext.Locale, log *slog.Logger) biz.AppRepo {
	return &appRepo{
		t:    t,
		conf: conf,
		db:   db,
		log:  log,
		api:  api.NewAPI(app.Version, app.Locale),
	}
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

func (r *appRepo) ListHomeShow() ([]*biz.App, error) {
	var apps []*biz.App
	if err := r.db.Where("show = ?", true).Order("show_order").Find(&apps).Error; err != nil {
		return nil, err
	}

	return apps, nil
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

func (r *appRepo) UpdateShow(slug string, show bool) error {
	item, err := r.GetInstalled(slug)
	if err != nil {
		return err
	}

	item.Show = show

	return r.db.Save(item).Error
}

func (r *appRepo) UpdateOrder(slugs []string) error {
	for i, slug := range slugs {
		if err := r.db.Model(&biz.App{}).Where("slug = ?", slug).Update("show_order", i).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *appRepo) DownloadCallback(slug string) {
	// 下载回调
	if err := r.api.AppCallback(slug); err != nil {
		r.log.Warn(r.t.Get("download callback failed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", 0), slog.String("app", slug), slog.Any("err", err))
	}
}

// CheckPanelVersion 校验面板版本号可解析；原编排在存在性检查前先做此校验
func (r *appRepo) CheckPanelVersion() error {
	_, err := version.NewVersion(app.Version)
	return err
}

// ResolveScript 解析渠道并按操作类型生成执行脚本
func (r *appRepo) ResolveScript(item *api.App, matchChannel, action, execVersion string) (string, error) {
	panel, err := version.NewVersion(app.Version)
	if err != nil {
		return "", err
	}

	shellUrl, shellChannel, shellVersion := "", "", ""
	for ch := range slices.Values(item.Channels) {
		vs, err := version.NewVersion(ch.Panel)
		if err != nil {
			continue
		}
		if ch.Slug == matchChannel {
			if vs.GreaterThan(panel) && !r.conf.App.Debug {
				return "", errors.New(r.t.Get("app %s requires panel version %s, current version %s", item.Name, ch.Panel, app.Version))
			}
			switch action {
			case "install":
				shellUrl = fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, ch.Install)
				shellVersion = ch.Version
			case "uninstall":
				shellUrl = fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, ch.Uninstall)
				shellVersion = execVersion
			case "update":
				shellUrl = fmt.Sprintf("https://%s%s", r.conf.App.DownloadEndpoint, ch.Update)
				shellVersion = ch.Version
			}
			shellChannel = ch.Slug
			break
		}
	}
	if shellUrl == "" {
		if action == "uninstall" {
			return "", errors.New(r.t.Get("failed to get uninstall script for app %s", item.Name))
		}
		return "", errors.New(r.t.Get("app %s not support current panel version", item.Name))
	}

	return fmt.Sprintf(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s"`, shellUrl, shellChannel, shellVersion), nil
}

// ExecScript 执行脚本
func (r *appRepo) ExecScript(ctx context.Context, script string) error {
	return shell.ExecWithOutput(ctx, script)
}

// customDir 自定义编译参数存放目录,安装脚本在编译时直接读取
func customDir(slug string) string {
	return filepath.Join(app.Root, "panel", "storage", "customize", slug)
}

func (r *appRepo) GetCustom(slug string) (*biz.AppCustom, error) {
	custom := new(biz.AppCustom)
	if pre, err := io.Read(filepath.Join(customDir(slug), "pre.sh")); err == nil {
		custom.PreScript = pre
	}
	if args, err := io.Read(filepath.Join(customDir(slug), "args")); err == nil {
		custom.Args = args
	}

	return custom, nil
}

func (r *appRepo) SaveCustom(ctx context.Context, slug string, custom *biz.AppCustom) error {
	dir := customDir(slug)
	// 全部为空时直接清理目录
	if strings.TrimSpace(custom.PreScript) == "" && strings.TrimSpace(custom.Args) == "" {
		return io.Remove(dir)
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	pre := filepath.Join(dir, "pre.sh")
	if strings.TrimSpace(custom.PreScript) == "" {
		if err := io.Remove(pre); err != nil {
			return err
		}
	} else {
		if err := io.Write(pre, custom.PreScript, 0700); err != nil {
			return err
		}
	}
	args := filepath.Join(dir, "args")
	if strings.TrimSpace(custom.Args) == "" {
		if err := io.Remove(args); err != nil {
			return err
		}
	} else {
		if err := io.Write(args, custom.Args, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (r *appRepo) PreCheck(app *api.App, catalog api.Apps) error {
	if strings.TrimSpace(app.Depends) == "" {
		return nil
	}

	var apps []string
	var installed []string

	all := catalog
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
		"self":      app.Slug, // 让同一互斥组的应用共用一条表达式
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

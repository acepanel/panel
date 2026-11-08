package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/apploader"
	"github.com/acepanel/panel/v3/pkg/types"
)

type AppService struct {
	t           *gotext.Locale
	log         *slog.Logger
	appRepo     *biz.AppUsecase
	cacheRepo   *biz.CacheUsecase
	settingRepo *biz.SettingUsecase
	loader      *apploader.Loader
}

func NewAppService(loader *apploader.Loader, appUsecase *biz.AppUsecase, cacheUsecase *biz.CacheUsecase, settingUsecase *biz.SettingUsecase, t *gotext.Locale, log *slog.Logger) *AppService {
	return &AppService{
		t:           t,
		log:         log,
		appRepo:     appUsecase,
		cacheRepo:   cacheUsecase,
		settingRepo: settingUsecase,
		loader:      loader,
	}
}

func (s *AppService) Categories(w http.ResponseWriter, r *http.Request) {
	categories := s.appRepo.Categories()

	Success(w, categories)
}

func (s *AppService) List(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	query := strings.ToLower(r.URL.Query().Get("query"))
	onlyInstalled := r.URL.Query().Get("installed") == "true"

	all := s.appRepo.All()
	installedApps, err := s.appRepo.Installed()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	installedAppMap := lo.KeyBy(installedApps, func(p *biz.App) string {
		return p.Slug
	})

	statusMap := s.collectStatuses(r.Context(), installedApps)

	var apps []types.AppDetail
	for _, item := range all {
		installed, installedChannel, installedVersion, updateExist, show, status := false, "", "", false, false, ""
		if inst, ok := installedAppMap[item.Slug]; ok {
			installed = true
			installedChannel = inst.Channel
			installedVersion = inst.Version
			show = inst.Show
			status = statusMap[item.Slug]
			// 直接用已加载的目录与安装记录比对，避免每个应用都重新查库并反序列化整个应用目录
			for _, channel := range item.Channels {
				if channel.Slug == inst.Channel && channel.Version != inst.Version {
					updateExist = true
					break
				}
			}
		}
		if onlyInstalled && !installed {
			continue
		}
		if category != "" && !strings.Contains(strings.Join(item.Categories, ","), category) {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(item.Name), query) &&
			!strings.Contains(strings.ToLower(item.Description), query) {
			continue
		}

		app := types.AppDetail{
			Name:             item.Name,
			Description:      item.Description,
			Categories:       item.Categories,
			Slug:             item.Slug,
			Installed:        installed,
			InstalledChannel: installedChannel,
			InstalledVersion: installedVersion,
			UpdateExist:      updateExist,
			Show:             show,
			Status:           status,
			CustomSupported:  biz.CustomCompileApp(item.Slug),
		}

		for _, c := range item.Channels {
			app.Channels = append(app.Channels, struct {
				Slug    string `json:"slug"`
				Name    string `json:"name"`
				Panel   string `json:"panel"`
				Version string `json:"version"`
				Log     string `json:"log"`
			}{
				Slug:    c.Slug,
				Name:    c.Name,
				Panel:   c.Panel,
				Version: c.Version,
				Log:     c.Log,
			})
		}

		apps = append(apps, app)
	}

	paged, total := Paginate(r, apps)

	s.log.Info(s.t.Get("app list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("category", category), slog.String("query", query), slog.Bool("installed", onlyInstalled), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))))

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *AppService) Install(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.App](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.Install(r.Context(), req.Channel, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) Uninstall(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.UnInstall(r.Context(), req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.Update(r.Context(), req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) UpdateShow(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppUpdateShow](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.UpdateShow(r.Context(), req.Slug, req.Show); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppUpdateOrder](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.UpdateOrder(req.Slugs); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) IsInstalled(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppSlugs](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	flag := false
	slugs := strings.SplitSeq(req.Slugs, ",")
	for item := range slugs {
		installed, err := s.appRepo.IsInstalled(item)
		if err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if installed {
			flag = true
			break
		}
	}

	Success(w, flag)
}

// GetCustom 获取自定义编译参数
func (s *AppService) GetCustom(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	custom, err := s.appRepo.GetCustom(req.Slug)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app custom compile viewed"), slog.String("type", lo.Ternary(biz.CustomCompileApp(req.Slug), biz.OperationTypeApp, biz.OperationTypeEnvironment)), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug))

	Success(w, custom)
}

// SaveCustom 保存自定义编译参数
func (s *AppService) SaveCustom(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppCustomSave](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.SaveCustom(r.Context(), req.Slug, &biz.AppCustom{
		PreScript: req.PreScript,
		Args:      req.Args,
	}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) UpdateCache(w http.ResponseWriter, r *http.Request) {
	if offline, _ := s.settingRepo.GetBool(biz.SettingKeyOfflineMode); offline {
		Error(w, http.StatusForbidden, s.t.Get("Unable to update app list cache in offline mode"))
		return
	}

	if err := s.cacheRepo.UpdateCategories(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err := s.cacheRepo.UpdateApps(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err := s.cacheRepo.UpdateEnvironments(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err := s.cacheRepo.UpdateTemplates(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app cache updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, nil)
}

// collectStatuses 并发获取已安装应用的运行状态
func (s *AppService) collectStatuses(ctx context.Context, installed []*biz.App) map[string]string {
	pairs := lop.Map(installed, func(item *biz.App, _ int) lo.Entry[string, string] {
		if a, ok := s.loader.Get(item.Slug); ok {
			return lo.Entry[string, string]{Key: item.Slug, Value: a.Status(ctx)}
		}
		return lo.Entry[string, string]{Key: item.Slug, Value: types.AppStatusNA}
	})
	return lo.FromEntries(pairs)
}

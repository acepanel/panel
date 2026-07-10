package service

import (
	"net/http"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/apploader"
	"github.com/acepanel/panel/v3/pkg/types"
)

type AppService struct {
	t           *gotext.Locale
	appRepo     *biz.AppUsecase
	cacheRepo   *biz.CacheUsecase
	settingRepo *biz.SettingUsecase
	loader      *apploader.Loader
}

func NewAppService(i do.Injector) (*AppService, error) {
	return &AppService{
		t:           do.MustInvoke[*gotext.Locale](i),
		appRepo:     do.MustInvoke[*biz.AppUsecase](i),
		cacheRepo:   do.MustInvoke[*biz.CacheUsecase](i),
		settingRepo: do.MustInvoke[*biz.SettingUsecase](i),
		loader:      do.MustInvoke[*apploader.Loader](i),
	}, nil
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

	statusMap := s.collectStatuses(installedApps)

	var apps []types.AppDetail
	for _, item := range all {
		installed, installedChannel, installedVersion, updateExist, show, status := false, "", "", false, false, ""
		if _, ok := installedAppMap[item.Slug]; ok {
			installed = true
			installedChannel = installedAppMap[item.Slug].Channel
			installedVersion = installedAppMap[item.Slug].Version
			updateExist = s.appRepo.UpdateExist(item.Slug)
			show = installedAppMap[item.Slug].Show
			status = statusMap[item.Slug]
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

	if err = s.appRepo.Install(req.Channel, req.Slug); err != nil {
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

	if err = s.appRepo.UnInstall(req.Slug); err != nil {
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

	if err = s.appRepo.Update(req.Slug); err != nil {
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

	if err = s.appRepo.UpdateShow(req.Slug, req.Show); err != nil {
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

	Success(w, nil)
}

// collectStatuses 并发获取已安装应用的运行状态
func (s *AppService) collectStatuses(installed []*biz.App) map[string]string {
	pairs := lop.Map(installed, func(item *biz.App, _ int) lo.Entry[string, string] {
		if a, ok := s.loader.Get(item.Slug); ok {
			return lo.Entry[string, string]{Key: item.Slug, Value: a.Status()}
		}
		return lo.Entry[string, string]{Key: item.Slug, Value: types.AppStatusNA}
	})
	return lo.FromEntries(pairs)
}

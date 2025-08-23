package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/types"
)

type AppService struct {
	t           *gotext.Locale
	appRepo     biz.AppRepo
	cacheRepo   biz.CacheRepo
	settingRepo biz.SettingRepo
}

func NewAppService(t *gotext.Locale, app biz.AppRepo, cache biz.CacheRepo, setting biz.SettingRepo) *AppService {
	return &AppService{
		t:           t,
		appRepo:     app,
		cacheRepo:   cache,
		settingRepo: setting,
	}
}

func (s *AppService) List(c fiber.Ctx) error {
	all := s.appRepo.All()
	installedApps, err := s.appRepo.Installed()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	installedAppMap := make(map[string]*biz.App)

	for _, p := range installedApps {
		installedAppMap[p.Slug] = p
	}

	var apps []types.AppCenter
	for _, item := range all {
		installed, installedChannel, installedVersion, updateExist, show := false, "", "", false, false
		if _, ok := installedAppMap[item.Slug]; ok {
			installed = true
			installedChannel = installedAppMap[item.Slug].Channel
			installedVersion = installedAppMap[item.Slug].Version
			updateExist = s.appRepo.UpdateExist(item.Slug)
			show = installedAppMap[item.Slug].Show
		}
		apps = append(apps, types.AppCenter{
			Icon:        item.Icon,
			Name:        item.Name,
			Description: item.Description,
			Slug:        item.Slug,
			Channels: []struct {
				Slug      string `json:"slug"`
				Name      string `json:"name"`
				Panel     string `json:"panel"`
				Install   string `json:"-"`
				Uninstall string `json:"-"`
				Update    string `json:"-"`
				Subs      []struct {
					Log     string `json:"log"`
					Version string `json:"version"`
				} `json:"subs"`
			}(item.Channels),
			Installed:        installed,
			InstalledChannel: installedChannel,
			InstalledVersion: installedVersion,
			UpdateExist:      updateExist,
			Show:             show,
		})
	}

	paged, total := Paginate(c, apps)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *AppService) Install(c fiber.Ctx) error {
	req, err := Bind[request.App](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.appRepo.Install(req.Channel, req.Slug); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *AppService) Uninstall(c fiber.Ctx) error {
	req, err := Bind[request.AppSlug](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.appRepo.UnInstall(req.Slug); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *AppService) Update(c fiber.Ctx) error {
	req, err := Bind[request.AppSlug](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.appRepo.Update(req.Slug); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *AppService) UpdateShow(c fiber.Ctx) error {
	req, err := Bind[request.AppUpdateShow](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.appRepo.UpdateShow(req.Slug, req.Show); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *AppService) IsInstalled(c fiber.Ctx) error {
	req, err := Bind[request.AppSlug](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	app, err := s.appRepo.Get(req.Slug)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	installed, err := s.appRepo.IsInstalled(req.Slug)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"name":      app.Name,
		"installed": installed,
	})
}

func (s *AppService) UpdateCache(c fiber.Ctx) error {
	if offline, _ := s.settingRepo.GetBool(biz.SettingKeyOfflineMode); offline {
		return Error(c, http.StatusForbidden, s.t.Get("Unable to update app list cache in offline mode"))
	}

	if err := s.cacheRepo.UpdateApps(); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

package service

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/gofiber/fiber/v3"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/io"
)

type WebsiteService struct {
	websiteRepo biz.WebsiteRepo
	settingRepo biz.SettingRepo
}

func NewWebsiteService(website biz.WebsiteRepo, setting biz.SettingRepo) *WebsiteService {
	return &WebsiteService{
		websiteRepo: website,
		settingRepo: setting,
	}
}

func (s *WebsiteService) GetRewrites(c fiber.Ctx) error {
	rewrites, err := s.websiteRepo.GetRewrites()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, rewrites)
}

func (s *WebsiteService) GetDefaultConfig(c fiber.Ctx) error {
	index, err := io.Read(filepath.Join(app.Root, "server/nginx/html/index.html"))
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	stop, err := io.Read(filepath.Join(app.Root, "server/nginx/html/stop.html"))
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, fiber.Map{
		"index": index,
		"stop":  stop,
	})
}

func (s *WebsiteService) UpdateDefaultConfig(c fiber.Ctx) error {
	req, err := Bind[request.WebsiteDefaultConfig](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.UpdateDefaultConfig(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

// UpdateCert 用于自动化工具更新证书
func (s *WebsiteService) UpdateCert(c fiber.Ctx) error {
	req, err := Bind[request.WebsiteUpdateCert](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.UpdateCert(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	websites, total, err := s.websiteRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, fiber.Map{
		"total": total,
		"items": websites,
	})
}

func (s *WebsiteService) Create(c fiber.Ctx) error {
	req, err := Bind[request.WebsiteCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if len(req.Path) == 0 {
		req.Path, _ = s.settingRepo.Get(biz.SettingKeyWebsitePath)
		req.Path = filepath.Join(req.Path, req.Name)
	}

	if _, err = s.websiteRepo.Create(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	config, err := s.websiteRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, config)
}

func (s *WebsiteService) Update(c fiber.Ctx) error {
	req, err := Bind[request.WebsiteUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.Update(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.WebsiteDelete](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.Delete(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) ClearLog(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.ClearLog(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) UpdateRemark(c fiber.Ctx) error {
	req, err := Bind[request.WebsiteUpdateRemark](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.UpdateRemark(req.ID, req.Remark); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) ResetConfig(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.ResetConfig(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) UpdateStatus(c fiber.Ctx) error {
	req, err := Bind[request.WebsiteUpdateStatus](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.UpdateStatus(req.ID, req.Status); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *WebsiteService) ObtainCert(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.websiteRepo.ObtainCert(context.Background(), req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

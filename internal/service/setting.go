package service

import (
	"github.com/gofiber/fiber/v2"
	"net/http"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/tools"
)

type SettingService struct {
	settingRepo biz.SettingRepo
}

func NewSettingService(setting biz.SettingRepo) *SettingService {
	return &SettingService{
		settingRepo: setting,
	}
}

func (s *SettingService) Get(c fiber.Ctx) error {
	setting, err := s.settingRepo.GetPanel()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, setting)
}

func (s *SettingService) Update(c fiber.Ctx) error {
	req, err := Bind[request.SettingPanel](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	restart := false
	if restart, err = s.settingRepo.UpdatePanel(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if restart {
		tools.RestartPanel()
	}

	return Success(c, nil)
}

// UpdateCert 用于自动化工具更新证书
func (s *SettingService) UpdateCert(c fiber.Ctx) error {
	req, err := Bind[request.SettingCert](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.settingRepo.UpdateCert(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	tools.RestartPanel()

	return Success(c, nil)
}

package service

import (
	"github.com/gofiber/fiber/v2"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/systemctl"
)

type SystemctlService struct {
	t *gotext.Locale
}

func NewSystemctlService(t *gotext.Locale) *SystemctlService {
	return &SystemctlService{
		t: t,
	}
}

func (s *SystemctlService) Status(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	status, err := systemctl.Status(req.Service)
	if err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to get %s service running status: %v", req.Service, err))
	}

	return Success(c, status)
}

func (s *SystemctlService) IsEnabled(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	enabled, err := systemctl.IsEnabled(req.Service)
	if err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to get %s service enable status: %v", req.Service, err))
	}

	return Success(c, enabled)
}

func (s *SystemctlService) Enable(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = systemctl.Enable(req.Service); err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to enable %s service: %v", req.Service, err))
	}

	return Success(c, nil)
}

func (s *SystemctlService) Disable(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = systemctl.Disable(req.Service); err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to disable %s service: %v", req.Service, err))
	}

	return Success(c, nil)
}

func (s *SystemctlService) Restart(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = systemctl.Restart(req.Service); err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to restart %s service: %v", req.Service, err))
	}

	return Success(c, nil)
}

func (s *SystemctlService) Reload(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = systemctl.Reload(req.Service); err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to reload %s service: %v", req.Service, err))
	}

	return Success(c, nil)
}

func (s *SystemctlService) Start(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = systemctl.Start(req.Service); err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to start %s service: %v", req.Service, err))
	}

	return Success(c, nil)
}

func (s *SystemctlService) Stop(c fiber.Ctx) error {
	req, err := Bind[request.SystemctlService](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = systemctl.Stop(req.Service); err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("failed to stop %s service: %v", req.Service, err))
	}

	return Success(c, nil)
}

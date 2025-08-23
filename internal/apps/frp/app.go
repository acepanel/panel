package frp

import (
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/systemctl"
)

type App struct{}

func NewApp() *App {
	return &App{}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) GetConfig(c fiber.Ctx) error {
	req, err := service.Bind[Name](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	config, err := io.Read(fmt.Sprintf("%s/server/frp/%s.toml", app.Root, req.Name))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) UpdateConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write(fmt.Sprintf("%s/server/frp/%s.toml", app.Root, req.Name), req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart(req.Name); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

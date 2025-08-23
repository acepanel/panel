package minio

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/systemctl"
)

type App struct{}

func NewApp() *App {
	return &App{}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/env", s.GetEnv)
	r.Post("/env", s.UpdateEnv)
}

func (s *App) GetEnv(c fiber.Ctx) error {
	env, _ := io.Read("/etc/default/minio")
	return service.Success(c, env)
}

func (s *App) UpdateEnv(c fiber.Ctx) error {
	req, err := service.Bind[UpdateEnv](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write("/etc/default/minio", req.Env, 0600); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("minio"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

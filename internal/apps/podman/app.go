package podman

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
	r.Get("/registry_config", s.GetRegistryConfig)
	r.Post("/registry_config", s.UpdateRegistryConfig)
	r.Get("/storage_config", s.GetStorageConfig)
	r.Post("/storage_config", s.UpdateStorageConfig)
}

func (s *App) GetRegistryConfig(c fiber.Ctx) error {
	config, err := io.Read("/etc/containers/registries.conf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) UpdateRegistryConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write("/etc/containers/registries.conf", req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("podman"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) GetStorageConfig(c fiber.Ctx) error {
	config, err := io.Read("/etc/containers/storage.conf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) UpdateStorageConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write("/etc/containers/storage.conf", req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("podman"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

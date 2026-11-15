package podman

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t   *gotext.Locale
	log *slog.Logger
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		t:   t,
		log: log,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/registry_config", s.GetRegistryConfig)
	r.Post("/registry_config", s.UpdateRegistryConfig)
	r.Get("/storage_config", s.GetStorageConfig)
	r.Post("/storage_config", s.UpdateStorageConfig)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "podman")
	return types.AggregateAppStatus(ok)
}

func (s *App) GetRegistryConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "podman"), slog.String("path", "/etc/containers/registries.conf"))
	common.ServeConfig(w, "/etc/containers/registries.conf")
}

func (s *App) UpdateRegistryConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "podman", "/etc/containers/registries.conf", 0644, common.Restart("podman"))
}

func (s *App) GetStorageConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "podman"), slog.String("path", "/etc/containers/storage.conf"))
	common.ServeConfig(w, "/etc/containers/storage.conf")
}

func (s *App) UpdateStorageConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "podman", "/etc/containers/storage.conf", 0644, common.Restart("podman"))
}

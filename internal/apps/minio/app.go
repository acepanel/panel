package minio

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

const envPath = "/etc/default/minio"

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
	r.Get("/env", s.GetEnv)
	r.Post("/env", s.UpdateEnv)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "minio")
	return types.AggregateAppStatus(ok)
}

func (s *App) GetEnv(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "minio"), slog.String("path", envPath))
	common.ServeConfig(w, envPath)
}

func (s *App) UpdateEnv(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateEnv](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	common.WriteConfig(w, r, s.log, s.t, "minio", envPath, req.Env, 0600, "minio")
}

package gitea

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
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
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "gitea")
	return types.AggregateAppStatus(ok)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	path := app.Root + "/server/gitea/app.ini"
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "gitea"), slog.String("path", path))
	common.ServeConfig(w, path)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "gitea", app.Root+"/server/gitea/app.ini", 0644, "gitea")
}

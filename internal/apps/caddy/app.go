package caddy

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	"github.com/acepanel/panel/v3/pkg/webserver/caddy"
)

type App struct {
	*common.WebServer
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		WebServer: common.NewWebServer(t, log, "caddy", webserver.TypeCaddy, caddy.MainConf, caddy.ErrorLogPath),
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.SaveConfig)
	r.Get("/error_log", s.ErrorLog)
	r.Post("/clear_error_log", s.ClearErrorLog)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "caddy")
	return types.AggregateAppStatus(ok)
}

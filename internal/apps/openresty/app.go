package openresty

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/nginx"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	nginx *nginx.App
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		nginx: nginx.New("openresty", t, log),
	}
}

func (s *App) Route(r chi.Router) {
	s.nginx.Route(r)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "nginx")
	return types.AggregateAppStatus(ok)
}

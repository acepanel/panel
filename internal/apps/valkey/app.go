package valkey

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/redis"
	"github.com/acepanel/panel/v3/internal/biz"
)

type App struct {
	redis *redis.App
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, taskRepo biz.TaskRepo) *App {
	return &App{
		redis: redis.New("valkey", "Valkey", t, log, databaseServerRepo, taskRepo),
	}
}

func (s *App) Route(r chi.Router) {
	s.redis.Route(r)
}

func (s *App) Status(ctx context.Context) string {
	return s.redis.Status(ctx)
}

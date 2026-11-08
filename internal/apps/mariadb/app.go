package mariadb

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/mysql"
	"github.com/acepanel/panel/v3/internal/biz"
)

type App struct {
	mysql *mysql.App
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, settingRepo biz.SettingRepo, taskRepo biz.TaskRepo) *App {
	return &App{
		mysql: mysql.New("mariadb", t, log, databaseServerRepo, settingRepo, taskRepo),
	}
}

func (s *App) Route(r chi.Router) {
	s.mysql.Route(r)
}

func (s *App) Status(ctx context.Context) string {
	return s.mysql.Status(ctx)
}

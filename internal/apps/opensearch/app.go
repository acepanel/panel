package opensearch

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/elasticsearch"
	"github.com/acepanel/panel/v3/internal/biz"
)

type App struct {
	elasticsearch *elasticsearch.App
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo) *App {
	return &App{
		elasticsearch: elasticsearch.New("opensearch", "OpenSearch", []string{"jvm.options"}, t, log, databaseServerRepo),
	}
}

func (s *App) Route(r chi.Router) {
	s.elasticsearch.Route(r)
}

func (s *App) Status(ctx context.Context) string {
	return s.elasticsearch.Status(ctx)
}

package caddy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	"github.com/acepanel/panel/v3/pkg/webserver/caddy"
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
	r.Post("/config", s.SaveConfig)
	r.Get("/error_log", s.ErrorLog)
	r.Post("/clear_error_log", s.ClearErrorLog)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "caddy")
	return types.AggregateAppStatus(ok)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(caddy.MainConf)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "caddy"), slog.String("path", caddy.MainConf))

	service.Success(w, config)
}

func (s *App) SaveConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(caddy.MainConf, req.Config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "caddy"), slog.String("path", caddy.MainConf))

	service.Success(w, nil)
}

func (s *App) ErrorLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, caddy.ErrorLogPath)
}

func (s *App) ClearErrorLog(w http.ResponseWriter, r *http.Request) {
	if _, err := shell.Execf(r.Context(), "cat /dev/null > %s", caddy.ErrorLogPath); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("app error log cleared"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "caddy"), slog.String("path", caddy.ErrorLogPath))

	service.Success(w, nil)
}

func (s *App) reload(ctx context.Context) error {
	ctx = context.WithoutCancel(ctx)
	d, err := webserver.Get(webserver.TypeCaddy)
	if err != nil {
		return err
	}
	if err = d.Reload(ctx); err != nil {
		return fmt.Errorf("%s", s.t.Get("failed to reload caddy: %v", err))
	}

	return nil
}

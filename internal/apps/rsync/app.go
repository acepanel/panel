package rsync

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
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
	r.Get("/modules", s.List)
	r.Post("/modules", s.Create)
	r.Post("/modules/{name}", s.Update)
	r.Delete("/modules/{name}", s.Delete)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "rsyncd")
	return types.AggregateAppStatus(ok)
}

func (s *App) List(w http.ResponseWriter, r *http.Request) {
	modules, err := listModules()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := service.Paginate(r, modules)

	s.log.Info(s.t.Get("rsync module list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rsync"), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))))

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *App) Create(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Module](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if io.Exists(modulePath(req.Name)) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s already exists", req.Name))
		return
	}

	s.save(r, w, req, s.t.Get("rsync module created"))
}

func (s *App) Update(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Module](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !io.Exists(modulePath(req.Name)) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Name))
		return
	}

	s.save(r, w, req, s.t.Get("rsync module updated"))
}

func (s *App) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ModuleName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 删模块与重启不跟随请求取消，否则配置已删而运行中的 rsyncd 仍在提供该模块
	ctx := context.WithoutCancel(r.Context())
	if err = io.Remove(modulePath(req.Name)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Remove(secretsPath(req.Name)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(ctx, "rsyncd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("rsync module deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rsync"), slog.String("name", req.Name))

	service.Success(w, nil)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rsync"), slog.String("path", rsyncdConf))
	common.ServeConfig(w, rsyncdConf)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "rsync", rsyncdConf, 0644, "rsyncd")
}

func (s *App) save(r *http.Request, w http.ResponseWriter, req *Module, msg string) {
	if err := writeModule(*req); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err := systemctl.Restart(r.Context(), "rsyncd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(msg, slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rsync"), slog.String("name", req.Name), slog.String("path", req.Path), slog.String("auth_user", req.AuthUser), slog.Bool("read_only", req.ReadOnly), slog.String("hosts_allow", req.HostsAllow))

	service.Success(w, nil)
}

package common

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/webserver"
)

// WebServer Web 服务器子应用共用的主配置、错误日志与重载处理，由各应用内嵌
type WebServer struct {
	t            *gotext.Locale
	log          *slog.Logger
	slug         string
	typ          webserver.Type
	confPath     string
	errorLogPath string
}

// NewWebServer slug 是操作日志里的应用标识，typ 决定重载走哪个方言
func NewWebServer(
	t *gotext.Locale,
	log *slog.Logger,
	slug string,
	typ webserver.Type,
	confPath, errorLogPath string,
) *WebServer {
	return &WebServer{
		t:            t,
		log:          log,
		slug:         slug,
		typ:          typ,
		confPath:     confPath,
		errorLogPath: errorLogPath,
	}
}

func (s *WebServer) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.confPath))
	ServeConfig(w, s.confPath)
}

func (s *WebServer) SaveConfig(w http.ResponseWriter, r *http.Request) {
	SaveConfig(w, r, s.log, s.t, s.slug, s.confPath, 0600, s.Reload)
}

func (s *WebServer) ErrorLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, s.errorLogPath)
}

func (s *WebServer) ClearErrorLog(w http.ResponseWriter, r *http.Request) {
	// 日志还没生成时没有可清的内容，不算失败
	if err := os.Truncate(s.errorLogPath, 0); err != nil && !errors.Is(err, fs.ErrNotExist) {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("app error log cleared"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.errorLogPath))

	service.Success(w, nil)
}

// Reload 经方言重载，错误里已带服务名与配置检查输出
func (s *WebServer) Reload(ctx context.Context) error {
	d, err := webserver.Get(s.typ)
	if err != nil {
		return err
	}

	// 配置已落盘，重载不跟随请求取消，否则磁盘配置与运行中的服务不一致
	return d.Reload(context.WithoutCancel(ctx))
}

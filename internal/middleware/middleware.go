package middleware

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/logrotate"
	"github.com/libtnb/sessions"
	sessionmiddleware "github.com/libtnb/sessions/middleware"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

type Middlewares struct {
	conf        *config.Config
	log         *slog.Logger
	session     *sessions.Manager
	appRepo     biz.AppRepo
	userSession biz.UserSessionRepo
	userToken   biz.UserTokenRepo
}

func NewMiddlewares(
	conf *config.Config,
	session *sessions.Manager,
	appRepo biz.AppRepo,
	userSessionRepo biz.UserSessionRepo,
	userTokenRepo biz.UserTokenRepo,
) (*Middlewares, func() error, error) {
	// http 访问日志写入轮转文件
	w, err := logrotate.New(filepath.Join(app.Root, "panel/storage/logs/http.log"),
		logrotate.WithMaxSize(10*logrotate.MB),
		logrotate.WithMaxAge(30*logrotate.Day),
		logrotate.WithRotateAt("00:00"),
		logrotate.WithFileMode(0o600),
		logrotate.WithLocation(time.Local),
	)
	if err != nil {
		return nil, nil, err
	}

	return &Middlewares{
		conf:        conf,
		log:         slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})),
		session:     session,
		appRepo:     appRepo,
		userSession: userSessionRepo,
		userToken:   userTokenRepo,
	}, w.Close, nil
}

// Globals 全局中间件集合，应用到每个请求；whitelist 为登录白名单路径。
func (r *Middlewares) Globals(t *gotext.Locale, whitelist []string) []func(http.Handler) http.Handler {
	compressor := chimiddleware.NewCompressor(6)

	return []func(http.Handler) http.Handler{
		recoverer(t),
		httplog.RequestLogger(r.log, &httplog.Options{
			Level:             slog.LevelInfo,
			LogRequestHeaders: []string{"User-Agent"},
			Skip: func(req *http.Request, respStatus int) bool {
				return respStatus == 404 || respStatus == 405
			},
			LogRequestBody: func(req *http.Request) bool {
				return r.conf.App.Debug && req.Header.Get("X-Debug-Request") == "1"
			},
			LogResponseBody: func(req *http.Request) bool {
				return r.conf.App.Debug && req.Header.Get("X-Debug-Response") == "1"
			},
		}),
		compressor.Handler,
		status(t),
		sessionmiddleware.StartSession(r.session),
		entrance(r.conf, r.session),
		mustLogin(t, r.conf, r.userSession, r.userToken, whitelist),
		mustInstall(t, r.appRepo),
	}
}

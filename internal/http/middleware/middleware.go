package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/google/wire"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sessions"

	"github.com/tnborg/panel/internal/biz"
)

var ProviderSet = wire.NewSet(NewMiddlewares)

type Middlewares struct {
	conf      *koanf.Koanf
	log       *slog.Logger
	session   *sessions.Manager
	app       biz.AppRepo
	userToken biz.UserTokenRepo
}

func NewMiddlewares(conf *koanf.Koanf, log *slog.Logger, session *sessions.Manager, app biz.AppRepo, userToken biz.UserTokenRepo) *Middlewares {
	return &Middlewares{
		conf:      conf,
		log:       log,
		session:   session,
		app:       app,
		userToken: userToken,
	}
}

// Globals applies global middleware to the Fiber app
func (r *Middlewares) Globals(t *gotext.Locale, app *fiber.App) {
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} - ${latency}\n",
	}))
	app.Use(compress.New())
	app.Use(StartSession(r.session))
	app.Use(Status(t))
	app.Use(Entrance(t, r.conf, r.session))
	app.Use(MustLogin(t, r.session, r.userToken))
	app.Use(MustInstall(t, r.app))
}

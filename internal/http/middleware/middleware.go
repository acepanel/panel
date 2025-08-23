package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
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

// Globals is a collection of global middleware that will be applied to every request.
func (r *Middlewares) Globals(t *gotext.Locale, app *fiber.App) {
	// Recovery middleware
	app.Use(recover.New())

	// Logger middleware
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path}\n",
	}))

	// Compression middleware
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	// Custom session middleware - will need to be adapted
	app.Use(SessionMiddleware(r.session))

	// Custom middlewares
	app.Use(StatusMiddleware(t))
	app.Use(EntranceMiddleware(t, r.conf, r.session))
	app.Use(MustLoginMiddleware(t, r.session, r.userToken))
	app.Use(MustInstallMiddleware(t, r.app))
}

// SessionMiddleware adapts the session middleware for Fiber
func SessionMiddleware(session *sessions.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Convert Fiber context to standard HTTP request/response for session compatibility
		// This is a placeholder - will need proper session handling adaptation
		return c.Next()
	}
}

// StatusMiddleware checks application status
func StatusMiddleware(t *gotext.Locale) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Import app package after fixing imports
		// This is a placeholder for status checking
		return c.Next()
	}
}

// EntranceMiddleware validates request entrance
func EntranceMiddleware(t *gotext.Locale, conf *koanf.Koanf, session *sessions.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Placeholder - will need adaptation from the original entrance middleware
		return c.Next()
	}
}

// MustLoginMiddleware validates user login
func MustLoginMiddleware(t *gotext.Locale, session *sessions.Manager, userToken biz.UserTokenRepo) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Placeholder - will need adaptation from the original must login middleware
		return c.Next()
	}
}

// MustInstallMiddleware validates installation
func MustInstallMiddleware(t *gotext.Locale, app biz.AppRepo) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Placeholder - will need adaptation from the original must install middleware
		return c.Next()
	}
}

package bootstrap

import (
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/http/middleware"
	"github.com/tnborg/panel/internal/route"
)

func NewRouter(t *gotext.Locale, middlewares *middleware.Middlewares, http *route.Http, ws *route.Ws, conf *koanf.Koanf) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		DisableKeepalive:  !conf.Bool("http.keepalive"),
		ReadBufferSize:    8192,
		WriteBufferSize:   8192,
		RequestMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"},
	})

	// add middleware
	middlewares.Globals(t, app)
	// add http route
	http.Register(app)
	// add ws route
	ws.Register(app)

	return app, nil
}

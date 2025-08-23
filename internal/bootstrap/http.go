package bootstrap

import (
	"crypto/tls"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/http/middleware"
	"github.com/tnborg/panel/internal/route"
)

func NewRouter(t *gotext.Locale, middlewares *middleware.Middlewares, http *route.Http, ws *route.Ws) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		MaxRequestBodySize: 2048 << 20, // 2GB max request body size
	})

	// add middleware
	middlewares.Globals(t, app)
	// add http route
	http.Register(app)
	// add ws route
	ws.Register(app)

	return app, nil
}

func NewHttp(conf *koanf.Koanf, app *fiber.App) error {
	// Configure TLS if enabled
	if conf.Bool("http.tls") {
		app.Server().TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		
		certFile := conf.String("http.cert_file")
		keyFile := conf.String("http.key_file")
		
		return app.ListenTLS(fmt.Sprintf(":%d", conf.MustInt("http.port")), certFile, keyFile)
	}

	return app.Listen(fmt.Sprintf(":%d", conf.MustInt("http.port")))
}

package bootstrap

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/bddjr/hlfhr"
	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/quic-go/quic-go/http3"

	"github.com/acepanel/panel/internal/http/middleware"
	"github.com/acepanel/panel/internal/route"
	"github.com/acepanel/panel/pkg/config"
)

func NewRouter(t *gotext.Locale, middlewares *middleware.Middlewares, http *route.Http, ws *route.Ws) (*chi.Mux, error) {
	r := chi.NewRouter()

	// add middleware
	r.Use(middlewares.Globals(t, r)...)
	// add http route
	http.Register(r)
	// add ws route
	ws.Register(r)

	return r, nil
}

func NewHttp(conf *config.Config, mux *chi.Mux) (*hlfhr.Server, error) {
	handler := http.Handler(mux)

	// 启用 TLS 时，添加 Alt-Svc 响应头通告 HTTP/3 支持
	if conf.HTTP.TLS {
		altSvc := fmt.Sprintf(`h3=":%d"; ma=2592000`, conf.HTTP.Port)
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Alt-Svc", altSvc)
			mux.ServeHTTP(w, r)
		})
	}

	srv := hlfhr.New(&http.Server{
		Addr:           fmt.Sprintf(":%d", conf.HTTP.Port),
		Handler:        handler,
		MaxHeaderBytes: 4 << 20,
	})
	srv.Listen80RedirectTo443 = true

	if conf.HTTP.TLS {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return srv, nil
}

// NewHTTP3 创建 HTTP/3 (QUIC) 服务器，TLS 启用时自动启用
func NewHTTP3(conf *config.Config, mux *chi.Mux) *http3.Server {
	if !conf.HTTP.TLS {
		return nil
	}

	return &http3.Server{
		Addr:    fmt.Sprintf(":%d", conf.HTTP.Port),
		Handler: mux,
	}
}

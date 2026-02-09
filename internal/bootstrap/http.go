package bootstrap

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/bddjr/hlfhr"
	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/quic-go/quic-go/http3"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/http/middleware"
	"github.com/acepanel/panel/internal/route"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/tlscert"
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

func NewTLSReloader(conf *config.Config) (*tlscert.Reloader, error) {
	if !conf.HTTP.TLS {
		return nil, nil
	}

	certFile := filepath.Join(app.Root, "panel/storage/cert.pem")
	keyFile := filepath.Join(app.Root, "panel/storage/cert.key")
	reloader, err := tlscert.NewReloader(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}
	return reloader, nil
}

func NewHttp(conf *config.Config, mux *chi.Mux, reloader *tlscert.Reloader) (*hlfhr.Server, error) {
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

	if conf.HTTP.TLS && reloader != nil {
		srv.TLSConfig = &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: reloader.GetCertificate,
		}
	}

	return srv, nil
}

func NewHTTP3(conf *config.Config, mux *chi.Mux, srv *hlfhr.Server) *http3.Server {
	// 必须启用 TLS 才能使用 HTTP/3
	if !conf.HTTP.TLS {
		return nil
	}

	return &http3.Server{
		Addr:      fmt.Sprintf(":%d", conf.HTTP.Port),
		Handler:   mux,
		TLSConfig: srv.TLSConfig,
	}
}

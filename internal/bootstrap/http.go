package bootstrap

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/bddjr/hlfhr"
	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"github.com/libtnb/validator/contrib/openapi"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/middleware"
	"github.com/acepanel/panel/v3/internal/route"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/apploader"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/embed"
	"github.com/acepanel/panel/v3/pkg/tlscert"
)

func NewRouter(loader *apploader.Loader, conf *config.Config, t *gotext.Locale, middlewares *middleware.Middlewares, v *validator.Validator, endpoints []route.Endpoints) (*chi.Mux, error) {
	mws := middlewares

	// 供 service.Bind 使用
	service.SetValidator(v)

	// 数据驱动的登录白名单
	public := route.PublicPaths(endpoints)

	r := chi.NewRouter()
	r.Use(mws.Globals(t, r, public)...)

	// 注册各域路由
	route.HTTP(conf, endpoints, r)

	// 动态应用子路由
	r.Route("/api/apps", func(r chi.Router) {
		loader.Register(r)
	})

	// 仅调试模式挂载 OpenAPI 文档
	if conf.App.Debug {
		spec, err := route.SpecJSON(endpoints, "AcePanel", v)
		if err != nil {
			return nil, err
		}
		docs := openapi.DocsHTML("AcePanel", "/openapi.json")
		r.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(spec)
		})
		r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(docs)
		})
	}

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		// /api 开头的返回 404
		if strings.HasPrefix(req.URL.Path, "/api") {
			http.NotFound(w, req)
			return
		}
		// 其他返回前端页面
		frontend, _ := fs.Sub(embed.PublicFS, "frontend")
		newPrecompressedSPAHandler(http.FS(frontend)).ServeHTTP(w, req)
	})

	return r, nil
}

func NewTLSReloader(conf *config.Config) (*tlscert.Reloader, error) {

	if !conf.HTTP.IsHTTPS() {
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

func NewHttp(router *chi.Mux, conf *config.Config, reloader *tlscert.Reloader) *hlfhr.Server {

	mux := router

	srv := hlfhr.New(&http.Server{
		Addr:           fmt.Sprintf(":%d", conf.HTTP.Port),
		Handler:        mux,
		MaxHeaderBytes: 4 << 20,
	})
	srv.Listen80RedirectTo443 = true

	if conf.HTTP.IsHTTPS() && reloader != nil {
		srv.TLSConfig = &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: reloader.GetCertificate,
		}
	}

	return srv
}

func newPrecompressedSPAHandler(fsys http.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		acceptBr := strings.Contains(r.Header.Get("Accept-Encoding"), "br")

		if served := serveFileWithBr(w, r, fsys, path, acceptBr); served {
			return
		}

		// 文件不存在，SPA fallback 到 index.html
		serveFileWithBr(w, r, fsys, "/index.html", acceptBr)
	}
}

func serveFileWithBr(w http.ResponseWriter, r *http.Request, fsys http.FileSystem, path string, acceptBr bool) bool {
	name := filepath.Base(path)
	// 尝试打开 .br 版本
	if f, err := fsys.Open(path + ".br"); err == nil {
		defer func(f http.File) { _ = f.Close() }(f)
		fi, err := f.Stat()
		if err != nil || fi.IsDir() {
			return false
		}

		ct := mime.TypeByExtension(filepath.Ext(path))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Vary", "Accept-Encoding")

		if acceptBr {
			// 客户端支持 br，直接透传
			w.Header().Set("Content-Encoding", "br")
			http.ServeContent(w, r, name, fi.ModTime(), f)
		} else {
			// 客户端不支持 br，解压后返回（由中间件处理 gzip）
			decoded, err := io.ReadAll(brotli.NewReader(f))
			if err != nil {
				return false
			}
			http.ServeContent(w, r, name, fi.ModTime(), bytes.NewReader(decoded))
		}
		return true
	}

	// 回退到原始文件（字体、图片等未压缩的资源）
	f, err := fsys.Open(path)
	if err != nil {
		return false
	}
	defer func(f http.File) { _ = f.Close() }(f)
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		return false
	}
	http.ServeContent(w, r, name, fi.ModTime(), f)
	return true
}

package middleware

import (
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/libtnb/chix/v2"
	"github.com/libtnb/sessions"

	"github.com/acepanel/panel/v3/pkg/clientip"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/embed"
	"github.com/acepanel/panel/v3/pkg/punycode"
)

// entrance 确保通过正确的入口访问
func entrance(conf *config.Config, session *sessions.Manager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := session.GetSession(r)
			if err != nil {
				abort(w, http.StatusInternalServerError, "%v", err)
				return
			}

			entrance := strings.TrimSuffix(conf.HTTP.Entrance, "/")
			if !strings.HasPrefix(entrance, "/") {
				entrance = "/" + entrance
			}

			// 情况一：设置了绑定域名、IP、UA，且请求不符合要求，返回错误
			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			if strings.Contains(host, "xn--") {
				if decoded, err := punycode.DecodeDomain(host); err == nil {
					host = decoded
				}
			}
			if len(conf.HTTP.BindDomain) > 0 && !slices.Contains(conf.HTTP.BindDomain, host) {
				abortEntrance(w, r, conf)
				return
			}

			if len(conf.HTTP.BindIP) > 0 && !clientip.Match(clientip.FromRequest(r, conf.HTTP.IPHeader), conf.HTTP.BindIP) {
				abortEntrance(w, r, conf)
				return
			}
			if len(conf.HTTP.BindUA) > 0 && !slices.Contains(conf.HTTP.BindUA, r.UserAgent()) {
				abortEntrance(w, r, conf)
				return
			}

			// 情况二：请求路径与入口路径相同或未设置访问入口，标记通过验证并重定向
			if (strings.TrimSuffix(r.URL.Path, "/") == entrance || entrance == "/") &&
				r.Header.Get("Authorization") == "" {
				if sess.Missing("verify_entrance") {
					sess.Put("verify_entrance", true)
				}
				// 设置入口的情况下进行重定向
				if entrance != "/" {
					render := chix.NewRender(w, r)
					defer render.Release()
					render.Redirect("/login")
					return
				}
			}

			// 情况三：通过 APIKey+入口路径访问 API，重写请求路径并跳过入口验证
			apiPath, ok := trimEntranceAPIPath(r.URL.Path, entrance)
			if ok && r.Header.Get("Authorization") != "" {
				if rctx := chi.RouteContext(r.Context()); rctx != nil {
					rctx.RoutePath = apiPath
				}
				r.URL.Path = apiPath
				next.ServeHTTP(w, r)
				return
			}

			// 情况四：Webhook 与文件分享下载访问，跳过验证
			if strings.HasPrefix(r.URL.Path, "/webhook/") || strings.HasPrefix(r.URL.Path, "/download/") {
				next.ServeHTTP(w, r)
				return
			}

			// 情况五：非调试模式且未通过验证的请求，返回错误
			if !conf.App.Debug &&
				sess.Missing("verify_entrance") &&
				r.URL.Path != "/robots.txt" {
				abortEntrance(w, r, conf)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func abortEntrance(w http.ResponseWriter, r *http.Request, conf *config.Config) {
	errorType := conf.HTTP.EntranceError

	switch errorType {
	case "close":
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, err := hj.Hijack()
			if err == nil {
				_ = conn.Close()
				return
			}
		}
		// 如果无法 hijack，则返回空响应
		w.WriteHeader(http.StatusTeapot)
		return
	case "nginx":
		content, err := embed.ErrorFS.ReadFile("error/nginx_404.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Server", "nginx")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(content)
		return
	default:
		fileName := "error/418.html"
		if conf.App.Locale == "zh_CN" {
			fileName = "error/418_zh_CN.html"
		}
		content, _ := embed.ErrorFS.ReadFile(fileName)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write(content)
		return
	}
}

func trimEntranceAPIPath(path string, entrance string) (string, bool) {
	if entrance == "/" {
		if path == "/api" || strings.HasPrefix(path, "/api/") {
			return path, true
		}
		return "", false
	}

	if path != entrance && !strings.HasPrefix(path, entrance+"/") {
		return "", false
	}

	apiPath := strings.TrimPrefix(path, entrance)
	if apiPath == "" {
		return "", false
	}
	if apiPath != "/api" && !strings.HasPrefix(apiPath, "/api/") {
		return "", false
	}

	return apiPath, true
}

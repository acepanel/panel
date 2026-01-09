package middleware

import (
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/sessions"

	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/embed"
	"github.com/acepanel/panel/pkg/punycode"
)

// Entrance 确保通过正确的入口访问
func Entrance(t *gotext.Locale, conf *config.Config, session *sessions.Manager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := session.GetSession(r)
			if err != nil {
				Abort(w, http.StatusInternalServerError, "%v", err)
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
				abortEntrance(w, r, conf, conf.App.Locale)
				return
			}

			// 取请求 IP
			ip := r.RemoteAddr
			ipHeader := conf.HTTP.IPHeader
			if ipHeader != "" && r.Header.Get(ipHeader) != "" {
				ip = strings.Split(r.Header.Get(ipHeader), ",")[0]
			}
			ip, _, err = net.SplitHostPort(strings.TrimSpace(ip))
			if err != nil {
				ip = r.RemoteAddr
			}

			if len(conf.HTTP.BindIP) > 0 {
				allowed := false
				requestIP := net.ParseIP(ip)
				if requestIP != nil {
					for _, allowedIP := range conf.HTTP.BindIP {
						if strings.Contains(allowedIP, "/") {
							// CIDR
							if _, ipNet, err := net.ParseCIDR(allowedIP); err == nil && ipNet.Contains(requestIP) {
								allowed = true
								break
							}
						} else {
							// IP
							if allowedIP == ip {
								allowed = true
								break
							}
						}
					}
				}
				if !allowed {
					abortEntrance(w, r, conf, conf.App.Locale)
					return
				}
			}
			if len(conf.HTTP.BindUA) > 0 && !slices.Contains(conf.HTTP.BindUA, r.UserAgent()) {
				abortEntrance(w, r, conf, conf.App.Locale)
				return
			}

			// 情况二：请求路径与入口路径相同或未设置访问入口，标记通过验证并重定向
			if (strings.TrimSuffix(r.URL.Path, "/") == entrance || entrance == "/") &&
				r.Header.Get("Authorization") == "" {
				sess.Put("verify_entrance", true)
				// 设置入口的情况下进行重定向
				if entrance != "/" {
					render := chix.NewRender(w, r)
					defer render.Release()
					render.Redirect("/login")
					return
				}
			}

			// 情况三：通过APIKey+入口路径访问，重写请求路径并跳过验证
			if strings.HasPrefix(r.URL.Path, entrance) && r.Header.Get("Authorization") != "" {
				// 只在设置了入口路径的情况下，才进行重写
				if entrance != "/" {
					if rctx := chi.RouteContext(r.Context()); rctx != nil {
						rctx.RoutePath = strings.TrimPrefix(rctx.RoutePath, entrance)
						r.URL.Path = strings.TrimPrefix(r.URL.Path, entrance)
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			// 情况四：Webhook 访问，跳过验证
			if strings.HasPrefix(r.URL.Path, "/webhook/") {
				next.ServeHTTP(w, r)
				return
			}

			// 情况五：非调试模式且未通过验证的请求，返回错误
			if !conf.App.Debug &&
				sess.Missing("verify_entrance") &&
				r.URL.Path != "/robots.txt" {
				abortEntrance(w, r, conf, conf.App.Locale)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func abortEntrance(w http.ResponseWriter, r *http.Request, conf *config.Config, locale string) {
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
		if locale == "zh_CN" {
			fileName = "error/418_zh_CN.html"
		}
		content, _ := embed.ErrorFS.ReadFile(fileName)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write(content)
		return
	}
}

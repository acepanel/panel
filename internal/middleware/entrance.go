package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/pkg/clientip"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/cookie"
	"github.com/acepanel/panel/v3/pkg/embed"
	"github.com/acepanel/panel/v3/pkg/punycode"
)

// entranceCookie 存放入口凭证的 Cookie 名
const entranceCookie = "ace_entrance"

// entrance 确保通过正确的入口访问
func entrance(conf *config.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ttl := conf.Session.TTL()
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
				AbortEntrance(w, r, conf)
				return
			}

			if len(conf.HTTP.BindIP) > 0 && !clientip.Match(clientip.FromRequest(r, conf.HTTP.IPHeader, conf.HTTP.TrustedProxies), conf.HTTP.BindIP) {
				AbortEntrance(w, r, conf)
				return
			}
			if len(conf.HTTP.BindUA) > 0 && !slices.Contains(conf.HTTP.BindUA, r.UserAgent()) {
				AbortEntrance(w, r, conf)
				return
			}

			// 情况二：访问入口路径，下发入口凭证并重定向到登录页
			if entrance != "/" && strings.TrimSuffix(r.URL.Path, "/") == entrance && r.Header.Get("Authorization") == "" {
				cookie.Set(w, r, entranceCookie, entrancePass(conf, entrance, time.Now().Add(ttl)), ttl)
				render := chix.NewRender(w, r)
				defer render.Release()
				render.Redirect("/login")
				return
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
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// 情况五：设置了入口时，非调试模式下没有有效凭证的请求返回错误
			if entrance != "/" && !conf.App.Debug && r.URL.Path != "/robots.txt" {
				expires := entranceExpires(conf, entrance, cookie.Get(r, entranceCookie))
				if expires.IsZero() {
					AbortEntrance(w, r, conf)
					return
				}
				// 有效期过半即续期
				if time.Until(expires) < ttl/2 {
					cookie.Set(w, r, entranceCookie, entrancePass(conf, entrance, time.Now().Add(ttl)), ttl)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func AbortEntrance(w http.ResponseWriter, r *http.Request, conf *config.Config) {
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

// entrancePass 签发入口凭证，对入口路径和过期时间签名
func entrancePass(conf *config.Config, entrance string, expires time.Time) string {
	unix := strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(conf.App.Key))
	mac.Write([]byte("entrance\n" + entrance + "\n" + unix))
	return unix + "." + hex.EncodeToString(mac.Sum(nil))
}

// entranceExpires 返回入口凭证的过期时间，凭证无效或已过期时返回零值
func entranceExpires(conf *config.Config, entrance string, pass string) time.Time {
	unix, _, _ := strings.Cut(pass, ".")
	seconds, err := strconv.ParseInt(unix, 10, 64)
	if err != nil {
		return time.Time{}
	}

	expires := time.Unix(seconds, 0)
	if !hmac.Equal([]byte(pass), []byte(entrancePass(conf, entrance, expires))) || time.Now().After(expires) {
		return time.Time{}
	}

	return expires
}

// isPublicPath 判断是否为 Webhook 回调或文件分享下载
func isPublicPath(path string) bool {
	for _, prefix := range []string{"/webhook/", "/download/"} {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			return rest != "" && !strings.Contains(rest, "/")
		}
	}

	return false
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

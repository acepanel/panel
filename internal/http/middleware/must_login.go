package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sessions"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

// MustLogin 确保已登录
func MustLogin(t *gotext.Locale, conf *config.Config, session *sessions.Manager, userToken biz.UserTokenRepo) func(next http.Handler) http.Handler {
	// 白名单
	whiteList := []string{
		"/api/user/key",
		"/api/user/captcha",
		"/api/user/login",
		"/api/user/logout",
		"/api/user/passkey/enabled",
		"/api/user/passkey/login",
		"/api/user/is_login",
		"/api/user/is_2fa",
		"/api/home/panel",
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := session.GetSession(r)
			if err != nil {
				Abort(w, http.StatusInternalServerError, "%v", err)
				return
			}

			// 对白名单和非 API 请求放行
			if slices.Contains(whiteList, r.URL.Path) || !strings.HasPrefix(r.URL.Path, "/api") {
				next.ServeHTTP(w, r)
				return
			}

			userID := uint(0)
			if r.Header.Get("Authorization") != "" {
				// 禁止访问 ws 相关的接口
				if strings.HasPrefix(r.URL.Path, "/api/ws") {
					Abort(w, http.StatusForbidden, t.Get("ws not allowed"))
					return
				}
				// API 请求验证
				if userID, err = userToken.ValidateReq(r); err != nil {
					Abort(w, http.StatusUnauthorized, "%v", err)
					return
				}
			} else {
				if sess.Missing("user_id") {
					Abort(w, http.StatusUnauthorized, t.Get("session expired, please login again"))
					return
				}

				safeLogin := cast.ToBool(sess.Get("safe_login"))
				if safeLogin {
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
					clientHash := fmt.Sprintf("%x", sha256.Sum256([]byte(ip)))
					safeClientHash := cast.ToString(sess.Get("safe_client"))
					if safeClientHash != clientHash || safeClientHash == "" {
						sess.Forget("user_id") // 清除 user_id，否则会来回跳转
						Abort(w, http.StatusUnauthorized, t.Get("client ip changed, please login again"))
						return
					}
				}

				userID = cast.ToUint(sess.Get("user_id"))
				refreshAt := cast.ToInt64(sess.Get("refresh_at")) // 上次刷新的时间戳
				// 距离上次刷新时间超过 10 分钟刷新 Cookie 有效期
				if time.Now().Unix()-refreshAt > 600 {
					sess.Put("refresh_at", time.Now().Unix())
					// 重新设置 Cookie
					http.SetCookie(w, &http.Cookie{
						Name:     sess.GetName(),
						Value:    sess.GetID(),
						Expires:  time.Now().Add(time.Duration(session.Lifetime) * time.Minute),
						Path:     "/",
						HttpOnly: true,
						Secure:   conf.HTTP.TLS,
						SameSite: http.SameSiteLaxMode,
					})
				}
			}

			if userID == 0 {
				Abort(w, http.StatusUnauthorized, "%v", t.Get("invalid user id, please login again"))
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), "user_id", userID)) // nolint:staticcheck
			next.ServeHTTP(w, r)
		})
	}
}

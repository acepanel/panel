package middleware

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/clientip"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/cookie"
)

// maxSignedBody API 令牌请求的请求体上限
const maxSignedBody = 32 << 20

// SessionCookie 存放登录会话令牌的 Cookie 名
const SessionCookie = "ace_session"

// mustLogin 确保已登录
func mustLogin(
	t *gotext.Locale,
	conf *config.Config,
	userSession biz.UserSessionRepo,
	userToken biz.UserTokenRepo,
	whitelist []string,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 对白名单和非 API 请求放行
			if slices.Contains(whitelist, r.URL.Path) || !strings.HasPrefix(r.URL.Path, "/api") {
				next.ServeHTTP(w, r)
				return
			}

			var userID uint
			if r.Header.Get("Authorization") != "" {
				// 禁止访问 ws 相关的接口
				if strings.HasPrefix(r.URL.Path, "/api/ws") {
					abort(w, http.StatusForbidden, t.Get("ws not allowed"))
					return
				}
				// 验签要先读完请求体，限制大小避免未认证的请求占满内存
				r.Body = http.MaxBytesReader(w, r.Body, maxSignedBody)
				// API 请求验证
				id, err := userToken.ValidateReq(r)
				if err != nil {
					if errors.As(err, new(*http.MaxBytesError)) {
						abort(w, http.StatusRequestEntityTooLarge, t.Get("request body is too large, the maximum is %d MB", maxSignedBody>>20))
						return
					}
					abort(w, http.StatusUnauthorized, "%v", err)
					return
				}
				userID = id
			} else {
				token := cookie.Get(r, SessionCookie)
				session, err := userSession.Authenticate(token, clientip.FromRequest(r, conf.HTTP.IPHeader, conf.HTTP.TrustedProxies))
				if err != nil {
					abort(w, http.StatusUnauthorized, "%v", err)
					return
				}
				// 活跃的会话顺延 Cookie 的有效期
				cookie.Set(w, r, SessionCookie, token, conf.Session.TTL())
				userID = session.UserID
			}

			if userID == 0 {
				abort(w, http.StatusUnauthorized, "%v", t.Get("invalid user id, please login again"))
				return
			}

			//nolint:staticcheck
			r = r.WithContext(context.WithValue(r.Context(), "user_id", userID))
			next.ServeHTTP(w, r)
		})
	}
}

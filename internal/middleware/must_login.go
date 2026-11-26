package middleware

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sessions"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/clientip"
	"github.com/acepanel/panel/v3/pkg/config"
)

// maxSignedBody API 令牌请求的请求体上限
const maxSignedBody = 32 << 20

// mustLogin 确保已登录
func mustLogin(
	t *gotext.Locale,
	conf *config.Config,
	session *sessions.Manager,
	user biz.UserRepo,
	userToken biz.UserTokenRepo,
	whitelist []string,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := session.GetSession(r)
			if err != nil {
				abort(w, http.StatusInternalServerError, "%v", err)
				return
			}

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
				if userID, err = userToken.ValidateReq(r); err != nil {
					if errors.As(err, new(*http.MaxBytesError)) {
						abort(w, http.StatusRequestEntityTooLarge, t.Get("request body is too large, the maximum is %d MB", maxSignedBody>>20))
						return
					}
					abort(w, http.StatusUnauthorized, "%v", err)
					return
				}
			} else {
				if sess.Missing("user_id") {
					abort(w, http.StatusUnauthorized, t.Get("session expired, please login again"))
					return
				}

				safeLogin := cast.ToBool(sess.Get("safe_login"))
				if safeLogin {
					clientHash := fmt.Sprintf("%x", sha256.Sum256([]byte(clientip.FromRequest(r, conf.HTTP.IPHeader, conf.HTTP.TrustedProxies))))
					safeClientHash := cast.ToString(sess.Get("safe_client"))
					if safeClientHash != clientHash {
						sess.Forget("user_id") // 清除 user_id，否则会来回跳转
						abort(w, http.StatusUnauthorized, t.Get("client ip changed, please login again"))
						return
					}
				}

				userID = cast.ToUint(sess.Get("user_id"))

				// 用户已删除或密码已变更时，旧会话随之失效
				if u, err := user.Get(userID); err != nil || u.SessionStamp() != cast.ToString(sess.Get("user_stamp")) {
					sess.Forget("user_id")
					abort(w, http.StatusUnauthorized, t.Get("session expired, please login again"))
					return
				}
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

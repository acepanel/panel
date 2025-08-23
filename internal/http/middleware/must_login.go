package middleware

import (
	"crypto/sha256"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sessions"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/biz"
)

// MustLogin 确保已登录
func MustLogin(t *gotext.Locale, session *sessions.Manager, userToken biz.UserTokenRepo) fiber.Handler {
	// 白名单
	whiteList := []string{
		"/api/user/key",
		"/api/user/login",
		"/api/user/logout",
		"/api/user/is_login",
		"/api/user/is_2fa",
		"/api/dashboard/panel",
	}
	return func(c fiber.Ctx) error {
		sess, err := GetSession(c)
		if err != nil {
			return Abort(c, fiber.StatusInternalServerError, "%v", err)
		}

		// 对白名单和非 API 请求放行
		if slices.Contains(whiteList, c.Path()) || !strings.HasPrefix(c.Path(), "/api") {
			return c.Next()
		}

		userID := uint(0)
		if c.Get("Authorization") != "" {
			// 禁止访问 ws 相关的接口
			if strings.HasPrefix(c.Path(), "/api/ws") {
				return Abort(c, fiber.StatusForbidden, t.Get("ws not allowed"))
			}
			// API 请求验证 - 需要实现 ValidateReq for Fiber
			// Note: This may need adjustment based on userToken implementation
			// For now, we'll skip this part until we can see the userToken interface
			return Abort(c, fiber.StatusNotImplemented, "API token validation not yet implemented for Fiber")
		} else {
			if sess.Missing("user_id") {
				return Abort(c, fiber.StatusUnauthorized, t.Get("session expired, please login again"))
			}

			safeLogin := cast.ToBool(sess.Get("safe_login"))
			if safeLogin {
				safeClientHash := cast.ToString(sess.Get("safe_client"))
				ip, _, _ := net.SplitHostPort(strings.TrimSpace(c.IP()))
				clientHash := fmt.Sprintf("%x", sha256.Sum256([]byte(ip)))
				if safeClientHash != clientHash || safeClientHash == "" {
					sess.Forget("user_id") // 清除 user_id，否则会来回跳转
					return Abort(c, fiber.StatusUnauthorized, t.Get("client ip/ua changed, please login again"))
				}
			}

			userID = cast.ToUint(sess.Get("user_id"))
		}

		if userID == 0 {
			return Abort(c, fiber.StatusUnauthorized, "%v", t.Get("invalid user id, please login again"))
		}

		c.Locals("user_id", userID)
		return c.Next()
	}
}

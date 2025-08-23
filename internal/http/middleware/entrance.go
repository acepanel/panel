package middleware

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sessions"
	"github.com/tnborg/panel/pkg/punycode"
)

// Entrance 确保通过正确的入口访问
func Entrance(t *gotext.Locale, conf *koanf.Koanf, session *sessions.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		sess, err := GetSession(c)
		if err != nil {
			return Abort(c, fiber.StatusInternalServerError, "%v", err)
		}

		entrance := strings.TrimSuffix(conf.String("http.entrance"), "/")
		if !strings.HasPrefix(entrance, "/") {
			entrance = "/" + entrance
		}

		// 情况一：设置了绑定域名、IP、UA，且请求不符合要求，返回错误
		host, _, err := net.SplitHostPort(c.Get("Host"))
		if err != nil {
			host = c.Get("Host")
		}
		if strings.Contains(host, "xn--") {
			if decoded, err := punycode.DecodeDomain(host); err == nil {
				host = decoded
			}
		}

		domains := conf.Strings("http.bind_domain")
		if len(domains) > 0 {
			domainOK := false
			for _, domain := range domains {
				if strings.EqualFold(host, domain) {
					domainOK = true
					break
				}
			}
			if !domainOK {
				return Abort(c, fiber.StatusTeapot, t.Get("invalid request domain: %s", c.Get("Host")))
			}
		}

		ip := c.IP()
		if len(conf.Strings("http.bind_ip")) > 0 {
			allowed := false
			requestIP := net.ParseIP(ip)
			if requestIP != nil {
				for _, allowedIP := range conf.Strings("http.bind_ip") {
					if strings.Contains(allowedIP, "/") {
						_, subnet, err := net.ParseCIDR(allowedIP)
						if err == nil && subnet.Contains(requestIP) {
							allowed = true
							break
						}
					} else {
						if allowedIP == ip {
							allowed = true
							break
						}
					}
				}
			}
			if !allowed {
				return Abort(c, fiber.StatusTeapot, t.Get("invalid request ip: %s", ip))
			}
		}

		if len(conf.Strings("http.bind_ua")) > 0 {
			userAgent := c.Get("User-Agent")
			uaOK := false
			for _, ua := range conf.Strings("http.bind_ua") {
				if strings.Contains(userAgent, ua) {
					uaOK = true
					break
				}
			}
			if !uaOK {
				return Abort(c, fiber.StatusTeapot, t.Get("invalid request ua: %s", userAgent))
			}
		}

		// 情况二：请求路径与入口路径相同或未设置访问入口，标记通过验证并重定向
		if (strings.TrimSuffix(c.Path(), "/") == entrance || entrance == "/") &&
			c.Get("Authorization") == "" {
			sess.Put("verify_entrance", true)
			// 设置入口的情况下进行重定向
			if entrance != "/" {
				c.Set("Location", "/login")
				return c.SendStatus(fiber.StatusFound)
			}
		}

		// 情况三：通过APIKey+入口路径访问，重写请求路径并跳过验证
		if strings.HasPrefix(c.Path(), entrance) && c.Get("Authorization") != "" {
			// 只在设置了入口路径的情况下，才进行重写
			if entrance != "/" {
				// For Fiber, we need to modify the path differently
				newPath := strings.TrimPrefix(c.Path(), entrance)
				c.Request().Header.Set("X-Original-Path", c.Path())
				c.Request().URI().SetPath(newPath)
			}
			return c.Next()
		}

		// 情况四：非调试模式且未通过验证的请求，返回错误
		if !conf.Bool("app.debug") &&
			sess.Missing("verify_entrance") &&
			c.Path() != "/robots.txt" {
			return Abort(c, fiber.StatusTeapot, t.Get("invalid access entrance"))
		}

		return c.Next()
	}
}
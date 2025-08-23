package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/biz"
)

// MustInstall 确保已安装应用
func MustInstall(t *gotext.Locale, app biz.AppRepo) fiber.Handler {
	return func(c fiber.Ctx) error {
		var slugs []string
		if strings.HasPrefix(c.Path(), "/api/website") {
			slugs = append(slugs, "nginx")
		} else if strings.HasPrefix(c.Path(), "/api/container") {
			slugs = append(slugs, "podman", "docker")
		} else if strings.HasPrefix(c.Path(), "/api/apps/") {
			pathArr := strings.Split(c.Path(), "/")
			if len(pathArr) < 4 {
				return Abort(c, fiber.StatusForbidden, t.Get("app not found"))
			}
			slugs = append(slugs, pathArr[3])
		}

		flag := false
		for _, s := range slugs {
			if installed, _ := app.IsInstalled("slug = ?", s); installed {
				flag = true
				break
			}
		}
		if !flag && len(slugs) > 0 {
			return Abort(c, fiber.StatusForbidden, t.Get("app %s not installed", slugs))
		}

		return c.Next()
	}
}

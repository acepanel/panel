package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/app"
)

// Status 检查程序状态
func Status(t *gotext.Locale) fiber.Handler {
	return func(c fiber.Ctx) error {
		switch app.Status {
		case app.StatusUpgrade:
			return Abort(c, fiber.StatusServiceUnavailable, t.Get("panel is upgrading, please refresh later"))
		case app.StatusMaintain:
			return Abort(c, fiber.StatusServiceUnavailable, t.Get("panel is maintaining, please refresh later"))
		case app.StatusClosed:
			return Abort(c, fiber.StatusServiceUnavailable, t.Get("panel is closed"))
		case app.StatusFailed:
			return Abort(c, fiber.StatusInternalServerError, t.Get("panel run error, please check or contact support"))
		default:
			return c.Next()
		}
	}
}

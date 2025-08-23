package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func Abort(c fiber.Ctx, code int, format string, args ...any) error {
	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}
	return c.Status(code).JSON(fiber.Map{
		"msg": format,
	})
}

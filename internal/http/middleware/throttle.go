package middleware

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

// Throttle 限流器
func Throttle(tokens int, interval time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        tokens,
		Expiration: interval,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"msg": "Rate limit exceeded",
			})
		},
	})
}

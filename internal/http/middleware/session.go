package middleware

import (
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/libtnb/sessions"
)

// StartSession creates a Fiber middleware for session management
func StartSession(manager *sessions.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Store session manager in context for later use
		c.Locals("session_manager", manager)
		return c.Next()
	}
}

// GetSession helper to get session from Fiber context
func GetSession(c fiber.Ctx) (*sessions.Session, error) {
	manager := c.Locals("session_manager").(*sessions.Manager)
	
	// Create URL from path and query string
	u, err := url.Parse(c.OriginalURL())
	if err != nil {
		return nil, err
	}
	
	// Create a temporary HTTP request from Fiber context
	req := &http.Request{
		Method:     c.Method(),
		URL:        u,
		Header:     make(http.Header),
		RemoteAddr: c.IP(),
	}
	
	// Copy headers
	c.Request().Header.VisitAll(func(key, value []byte) {
		req.Header.Set(string(key), string(value))
	})
	
	return manager.GetSession(req)
}
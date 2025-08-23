package route

import (
	"github.com/gofiber/fiber/v3"

	"github.com/tnborg/panel/internal/service"
)

type Ws struct {
	ws *service.WsService
}

func NewWs(ws *service.WsService) *Ws {
	return &Ws{
		ws: ws,
	}
}

func (route *Ws) Register(app *fiber.App) {
	// TODO: WebSocket routes need special handling for Fiber v3
	// Temporarily disabled during migration
	// app.Get("/api/ws/ssh", route.ws.Session)
	// app.Get("/api/ws/exec", route.ws.Exec)
}

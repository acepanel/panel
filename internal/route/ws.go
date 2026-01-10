package route

import (
	"github.com/go-chi/chi/v5"

	"github.com/acepanel/panel/internal/service"
)

type Ws struct {
	ws *service.WsService
}

func NewWs(ws *service.WsService) *Ws {
	return &Ws{
		ws: ws,
	}
}

func (route *Ws) Register(r *chi.Mux) {
	r.Route("/api/ws", func(r chi.Router) {
		r.Get("/ssh", route.ws.Session)
		r.Get("/exec", route.ws.Exec)
		r.Get("/pty", route.ws.PTY)
		r.Get("/container/{id}", route.ws.ContainerTerminal)
		r.Get("/container/image/pull", route.ws.ContainerImagePull)
	})
}

package route

import (
	"github.com/go-chi/chi/v5"

	"github.com/acepanel/panel/v3/internal/service"
)

type Ws struct {
	ws               *service.WsService
	toolboxMigration *service.ToolboxMigrationService
}

func NewWs(ws *service.WsService, toolboxMigration *service.ToolboxMigrationService) *Ws {
	return &Ws{
		ws:               ws,
		toolboxMigration: toolboxMigration,
	}
}

func (route *Ws) Register(r *chi.Mux) {
	r.Route("/api/ws", func(r chi.Router) {
		r.Get("/exec", route.ws.Exec)
		r.Get("/pty", route.ws.PTY)
		r.Get("/follow", route.ws.Follow)
		r.Get("/ssh", route.ws.Session)
		r.Get("/container/{id}", route.ws.ContainerTerminal)
		r.Get("/container/image/pull", route.ws.ContainerImagePull)
		r.Get("/migration/progress", route.toolboxMigration.Progress)
		r.Get("/cert/obtain", route.ws.CertObtain)
		r.Get("/cert/renew", route.ws.CertRenew)
	})
}

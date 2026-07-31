package acewaf

import (
	"github.com/go-chi/chi/v5"

	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct{}

func NewApp() (*App, error) {
	return &App{}, nil
}

func (s *App) Route(_ chi.Router) {
	// WAF 管理路由由 route 贡献（route/waf.go）统一提供
}

func (s *App) Status() string {
	ok, _ := systemctl.Status("acewaf")
	return types.AggregateAppStatus(ok)
}

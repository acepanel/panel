package bootstrap

import (
	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/config"
)

func NewAPI(conf *config.Config) *api.API {
	return api.NewAPI(app.Version, conf.App.Locale)
}

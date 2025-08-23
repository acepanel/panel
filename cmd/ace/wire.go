//go:build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/apps"
	"github.com/tnborg/panel/internal/bootstrap"
	"github.com/tnborg/panel/internal/data"
	"github.com/tnborg/panel/internal/http/middleware"
	"github.com/tnborg/panel/internal/job"
	"github.com/tnborg/panel/internal/route"
	"github.com/tnborg/panel/internal/service"
)

// initWeb init application.
func initWeb() (*app.Ace, error) {
	panic(wire.Build(bootstrap.ProviderSet, middleware.ProviderSet, route.ProviderSet, service.ProviderSet, data.ProviderSet, apps.ProviderSet, job.ProviderSet, app.NewWeb))
}

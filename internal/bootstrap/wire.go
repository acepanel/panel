//go:build wireinject

package bootstrap

import (
	"log/slog"

	"github.com/bddjr/hlfhr"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/cron"
	"github.com/libtnb/validator"
	"github.com/libtnb/wire"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/middleware"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/apploader"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/tlscert"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/websitestat"
)

// Module 装配基础设施层，*Logger、*middleware.Middlewares 与 *chi.Mux 仅本层内部使用，不导出
var Module = wire.New().
	Provide(NewConf).
	Provide(NewT).
	Provide(NewAPI).
	Provide(NewLogger).
	Provide(NewSlog).
	Provide(NewDB).
	Provide(NewMigrate).
	Provide(NewRunner).
	Provide(NewValidator).
	Provide(middleware.NewMiddlewares).
	Provide(NewLoader).
	Provide(NewRouter).
	Provide(NewTLSReloader).
	Provide(NewHttp).
	Provide(NewCron).
	Provide(NewCli).
	Provide(websitestat.NewAggregator).
	Export[*config.Config]().
	Export[*gotext.Locale]().
	Export[*api.API]().
	Export[*slog.Logger]().
	Export[*gorm.DB]().
	Export[*gormigrate.Gormigrate]().
	Export[types.TaskRunner]().
	Export[*validator.Validator]().
	Export[*apploader.Loader]().
	Export[*tlscert.Reloader]().
	Export[*hlfhr.Server]().
	Export[*cron.Cron]().
	Export[app.CliBuilder]().
	Export[*websitestat.Aggregator]()

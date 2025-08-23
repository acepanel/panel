package route

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/tnborg/panel/internal/http/middleware"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/apploader"
)

type Http struct {
	user             *service.UserService
	userToken        *service.UserTokenService
	dashboard        *service.DashboardService
	task             *service.TaskService
	website          *service.WebsiteService
	database         *service.DatabaseService
	databaseServer   *service.DatabaseServerService
	databaseUser     *service.DatabaseUserService
	backup           *service.BackupService
	cert             *service.CertService
	certDNS          *service.CertDNSService
	certAccount      *service.CertAccountService
	app              *service.AppService
	cron             *service.CronService
	process          *service.ProcessService
	safe             *service.SafeService
	firewall         *service.FirewallService
	ssh              *service.SSHService
	container        *service.ContainerService
	containerCompose *service.ContainerComposeService
	containerNetwork *service.ContainerNetworkService
	containerImage   *service.ContainerImageService
	containerVolume  *service.ContainerVolumeService
	file             *service.FileService
	monitor          *service.MonitorService
	setting          *service.SettingService
	systemctl        *service.SystemctlService
	toolboxSystem    *service.ToolboxSystemService
	toolboxBenchmark *service.ToolboxBenchmarkService
	apps             *apploader.Loader
}

func NewHttp(
	user *service.UserService,
	userToken *service.UserTokenService,
	dashboard *service.DashboardService,
	task *service.TaskService,
	website *service.WebsiteService,
	database *service.DatabaseService,
	databaseServer *service.DatabaseServerService,
	databaseUser *service.DatabaseUserService,
	backup *service.BackupService,
	cert *service.CertService,
	certDNS *service.CertDNSService,
	certAccount *service.CertAccountService,
	app *service.AppService,
	cron *service.CronService,
	process *service.ProcessService,
	safe *service.SafeService,
	firewall *service.FirewallService,
	ssh *service.SSHService,
	container *service.ContainerService,
	containerCompose *service.ContainerComposeService,
	containerNetwork *service.ContainerNetworkService,
	containerImage *service.ContainerImageService,
	containerVolume *service.ContainerVolumeService,
	file *service.FileService,
	monitor *service.MonitorService,
	setting *service.SettingService,
	systemctl *service.SystemctlService,
	toolboxSystem *service.ToolboxSystemService,
	toolboxBenchmark *service.ToolboxBenchmarkService,
	apps *apploader.Loader,
) *Http {
	return &Http{
		user:             user,
		userToken:        userToken,
		dashboard:        dashboard,
		task:             task,
		website:          website,
		database:         database,
		databaseServer:   databaseServer,
		databaseUser:     databaseUser,
		backup:           backup,
		cert:             cert,
		certDNS:          certDNS,
		certAccount:      certAccount,
		app:              app,
		cron:             cron,
		process:          process,
		safe:             safe,
		firewall:         firewall,
		ssh:              ssh,
		container:        container,
		containerCompose: containerCompose,
		containerNetwork: containerNetwork,
		containerImage:   containerImage,
		containerVolume:  containerVolume,
		file:             file,
		monitor:          monitor,
		setting:          setting,
		systemctl:        systemctl,
		toolboxSystem:    toolboxSystem,
		toolboxBenchmark: toolboxBenchmark,
		apps:             apps,
	}
}

func (route *Http) Register(app *fiber.App) {
	api := app.Group("/api")
	
	user := api.Group("/user")
	user.Get("/key", route.user.GetKey)
	user.Post("/login", route.user.Login, middleware.Throttle(5, time.Minute))
	user.Post("/logout", route.user.Logout)
	user.Get("/is_login", route.user.IsLogin)
	user.Get("/is_2fa", route.user.IsTwoFA)
	user.Get("/info", route.user.Info)

	users := api.Group("/users")
	users.Get("/", route.user.List)
	users.Post("/", route.user.Create)
	users.Post("/:id/username", route.user.UpdateUsername)
	users.Post("/:id/password", route.user.UpdatePassword)
	users.Post("/:id/email", route.user.UpdateEmail)
	users.Get("/:id/2fa", route.user.GenerateTwoFA)
	users.Post("/:id/2fa", route.user.UpdateTwoFA)
	users.Delete("/:id", route.user.Delete)

	userTokens := api.Group("/user_tokens")
	userTokens.Get("/", route.userToken.List)
	userTokens.Post("/", route.userToken.Create)
	userTokens.Put("/:id", route.userToken.Update)
	userTokens.Delete("/:id", route.userToken.Delete)

	dashboard := api.Group("/dashboard")
	dashboard.Get("/panel", route.dashboard.Panel)
	dashboard.Get("/home_apps", route.dashboard.HomeApps)
	dashboard.Post("/current", route.dashboard.Current)
	dashboard.Get("/system_info", route.dashboard.SystemInfo)
	dashboard.Get("/count_info", route.dashboard.CountInfo)
	dashboard.Get("/installed_db_and_php", route.dashboard.InstalledDbAndPhp)
	dashboard.Get("/check_update", route.dashboard.CheckUpdate)
	dashboard.Get("/update_info", route.dashboard.UpdateInfo)
	dashboard.Post("/update", route.dashboard.Update)
	dashboard.Post("/restart", route.dashboard.Restart)

	task := api.Group("/task")
	task.Get("/status", route.task.Status)
	task.Get("/", route.task.List)
	task.Get("/:id", route.task.Get)
	task.Delete("/:id", route.task.Delete)

	// App management routes
	appGroup := api.Group("/app")
	appGroup.Get("/list", route.app.List)
	appGroup.Post("/install", route.app.Install)
	appGroup.Post("/uninstall", route.app.Uninstall)
	appGroup.Post("/update", route.app.Update)
	appGroup.Post("/update_show", route.app.UpdateShow)
	appGroup.Get("/is_installed", route.app.IsInstalled)
	appGroup.Get("/update_cache", route.app.UpdateCache)

	// Individual app routes (nginx, redis, etc.)
	apps := api.Group("/apps")
	route.apps.Register(apps)

	// TODO: Add remaining routes (database, backup, cert, etc.)
	// This will be continued in future migrations

	// Add static file serving for frontend
	app.Use("/", static.New("./storage/frontend", static.Config{
		Browse: false,
	}))
	
	// Handle SPA routing - serve index.html for routes that don't start with /api
	app.Use(func(c fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api") {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendFile("./storage/frontend/index.html")
	})
}



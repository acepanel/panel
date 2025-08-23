package route

import (
	"io/fs"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/apploader"
	"github.com/tnborg/panel/pkg/embed"
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

	// User routes
	user := api.Group("/user")
	user.Get("/key", route.user.GetKey)
	user.Post("/login", route.user.Login) // TODO: Add throttle middleware
	user.Post("/logout", route.user.Logout)
	user.Get("/is_login", route.user.IsLogin)
	user.Get("/is_2fa", route.user.IsTwoFA)
	user.Get("/info", route.user.Info)

	// Users management routes
	users := api.Group("/users")
	users.Get("/", route.user.List)
	users.Post("/", route.user.Create)
	users.Post("/:id/username", route.user.UpdateUsername)
	users.Post("/:id/password", route.user.UpdatePassword)
	users.Post("/:id/email", route.user.UpdateEmail)
	users.Get("/:id/2fa", route.user.GenerateTwoFA)
	users.Post("/:id/2fa", route.user.UpdateTwoFA)
	users.Delete("/:id", route.user.Delete)

	// User tokens routes
	userTokens := api.Group("/user_tokens")
	userTokens.Get("/", route.userToken.List)
	userTokens.Post("/", route.userToken.Create)
	userTokens.Put("/:id", route.userToken.Update)
	userTokens.Delete("/:id", route.userToken.Delete)

	// Dashboard routes
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

	// Task routes
	task := api.Group("/task")
	task.Get("/status", route.task.Status)
	task.Get("/", route.task.List)
	task.Get("/:id", route.task.Get)
	task.Delete("/:id", route.task.Delete)

	// Website routes
	website := api.Group("/website")
	website.Get("/rewrites", route.website.GetRewrites)
	website.Get("/default_config", route.website.GetDefaultConfig)
	website.Post("/default_config", route.website.UpdateDefaultConfig)
	website.Post("/cert", route.website.UpdateCert)
	website.Get("/", route.website.List)
	website.Post("/", route.website.Create)
	website.Get("/:id", route.website.Get)
	website.Put("/:id", route.website.Update)
	website.Delete("/:id", route.website.Delete)
	website.Delete("/:id/log", route.website.ClearLog)
	website.Post("/:id/update_remark", route.website.UpdateRemark)
	website.Post("/:id/reset_config", route.website.ResetConfig)
	website.Post("/:id/status", route.website.UpdateStatus)
	website.Post("/:id/obtain_cert", route.website.ObtainCert)

	// Database routes
	database := api.Group("/database")
	database.Get("/", route.database.List)
	database.Post("/", route.database.Create)
	database.Delete("/", route.database.Delete)
	database.Post("/comment", route.database.Comment)

	// Database server routes
	databaseServer := api.Group("/database_server")
	databaseServer.Get("/", route.databaseServer.List)
	databaseServer.Post("/", route.databaseServer.Create)
	databaseServer.Get("/:id", route.databaseServer.Get)
	databaseServer.Put("/:id", route.databaseServer.Update)
	databaseServer.Put("/:id/remark", route.databaseServer.UpdateRemark)
	databaseServer.Delete("/:id", route.databaseServer.Delete)
	databaseServer.Post("/:id/sync", route.databaseServer.Sync)

	// Database user routes
	databaseUser := api.Group("/database_user")
	databaseUser.Get("/", route.databaseUser.List)
	databaseUser.Post("/", route.databaseUser.Create)
	databaseUser.Get("/:id", route.databaseUser.Get)
	databaseUser.Put("/:id", route.databaseUser.Update)
	databaseUser.Put("/:id/remark", route.databaseUser.UpdateRemark)
	databaseUser.Delete("/:id", route.databaseUser.Delete)

	// Backup routes
	backup := api.Group("/backup")
	backup.Get("/:type", route.backup.List)
	backup.Post("/:type", route.backup.Create)
	backup.Post("/:type/upload", route.backup.Upload)
	backup.Delete("/:type/delete", route.backup.Delete)
	backup.Post("/:type/restore", route.backup.Restore)

	// Certificate routes
	cert := api.Group("/cert")
	cert.Get("/ca_providers", route.cert.CAProviders)
	cert.Get("/dns_providers", route.cert.DNSProviders)
	cert.Get("/algorithms", route.cert.Algorithms)

	certCert := cert.Group("/cert")
	certCert.Get("/", route.cert.List)
	certCert.Post("/", route.cert.Create)
	certCert.Post("/upload", route.cert.Upload)
	certCert.Put("/:id", route.cert.Update)
	certCert.Get("/:id", route.cert.Get)
	certCert.Delete("/:id", route.cert.Delete)
	certCert.Post("/:id/obtain_auto", route.cert.ObtainAuto)
	certCert.Post("/:id/obtain_manual", route.cert.ObtainManual)
	certCert.Post("/:id/obtain_self_signed", route.cert.ObtainSelfSigned)
	certCert.Post("/:id/renew", route.cert.Renew)
	certCert.Post("/:id/manual_dns", route.cert.ManualDNS)
	certCert.Post("/:id/deploy", route.cert.Deploy)

	certDNS := cert.Group("/dns")
	certDNS.Get("/", route.certDNS.List)
	certDNS.Post("/", route.certDNS.Create)
	certDNS.Put("/:id", route.certDNS.Update)
	certDNS.Get("/:id", route.certDNS.Get)
	certDNS.Delete("/:id", route.certDNS.Delete)

	certAccount := cert.Group("/account")
	certAccount.Get("/", route.certAccount.List)
	certAccount.Post("/", route.certAccount.Create)
	certAccount.Put("/:id", route.certAccount.Update)
	certAccount.Get("/:id", route.certAccount.Get)
	certAccount.Delete("/:id", route.certAccount.Delete)

	// App routes
	appGroup := api.Group("/app")
	appGroup.Get("/list", route.app.List)
	appGroup.Post("/install", route.app.Install)
	appGroup.Post("/uninstall", route.app.Uninstall)
	appGroup.Post("/update", route.app.Update)
	appGroup.Post("/update_show", route.app.UpdateShow)
	appGroup.Get("/is_installed", route.app.IsInstalled)
	appGroup.Get("/update_cache", route.app.UpdateCache)

	// Cron routes
	cron := api.Group("/cron")
	cron.Get("/", route.cron.List)
	cron.Post("/", route.cron.Create)
	cron.Put("/:id", route.cron.Update)
	cron.Get("/:id", route.cron.Get)
	cron.Delete("/:id", route.cron.Delete)
	cron.Post("/:id/status", route.cron.Status)

	// Process routes
	process := api.Group("/process")
	process.Get("/", route.process.List)
	process.Post("/kill", route.process.Kill)

	// Safe routes
	safe := api.Group("/safe")
	safe.Get("/ssh", route.safe.GetSSH)
	safe.Post("/ssh", route.safe.UpdateSSH)
	safe.Get("/ping", route.safe.GetPingStatus)
	safe.Post("/ping", route.safe.UpdatePingStatus)

	// Firewall routes
	firewall := api.Group("/firewall")
	firewall.Get("/status", route.firewall.GetStatus)
	firewall.Post("/status", route.firewall.UpdateStatus)
	firewall.Get("/rule", route.firewall.GetRules)
	firewall.Post("/rule", route.firewall.CreateRule)
	firewall.Delete("/rule", route.firewall.DeleteRule)
	firewall.Get("/ip_rule", route.firewall.GetIPRules)
	firewall.Post("/ip_rule", route.firewall.CreateIPRule)
	firewall.Delete("/ip_rule", route.firewall.DeleteIPRule)
	firewall.Get("/forward", route.firewall.GetForwards)
	firewall.Post("/forward", route.firewall.CreateForward)
	firewall.Delete("/forward", route.firewall.DeleteForward)

	// SSH routes
	ssh := api.Group("/ssh")
	ssh.Get("/", route.ssh.List)
	ssh.Post("/", route.ssh.Create)
	ssh.Put("/:id", route.ssh.Update)
	ssh.Get("/:id", route.ssh.Get)
	ssh.Delete("/:id", route.ssh.Delete)

	// Container routes
	container := api.Group("/container")

	containerContainer := container.Group("/container")
	containerContainer.Get("/", route.container.List)
	containerContainer.Get("/search", route.container.Search)
	containerContainer.Post("/", route.container.Create)
	containerContainer.Delete("/:id", route.container.Remove)
	containerContainer.Post("/:id/start", route.container.Start)
	containerContainer.Post("/:id/stop", route.container.Stop)
	containerContainer.Post("/:id/restart", route.container.Restart)
	containerContainer.Post("/:id/pause", route.container.Pause)
	containerContainer.Post("/:id/unpause", route.container.Unpause)
	containerContainer.Post("/:id/kill", route.container.Kill)
	containerContainer.Post("/:id/rename", route.container.Rename)
	containerContainer.Get("/:id/logs", route.container.Logs)
	containerContainer.Post("/prune", route.container.Prune)

	containerCompose := container.Group("/compose")
	containerCompose.Get("/", route.containerCompose.List)
	containerCompose.Get("/:name", route.containerCompose.Get)
	containerCompose.Post("/", route.containerCompose.Create)
	containerCompose.Put("/:name", route.containerCompose.Update)
	containerCompose.Post("/:name/up", route.containerCompose.Up)
	containerCompose.Post("/:name/down", route.containerCompose.Down)
	containerCompose.Delete("/:name", route.containerCompose.Remove)

	containerNetwork := container.Group("/network")
	containerNetwork.Get("/", route.containerNetwork.List)
	containerNetwork.Post("/", route.containerNetwork.Create)
	containerNetwork.Delete("/:id", route.containerNetwork.Remove)
	containerNetwork.Post("/prune", route.containerNetwork.Prune)

	containerImage := container.Group("/image")
	containerImage.Get("/", route.containerImage.List)
	containerImage.Post("/", route.containerImage.Pull)
	containerImage.Delete("/:id", route.containerImage.Remove)
	containerImage.Post("/prune", route.containerImage.Prune)

	containerVolume := container.Group("/volume")
	containerVolume.Get("/", route.containerVolume.List)
	containerVolume.Post("/", route.containerVolume.Create)
	containerVolume.Delete("/:id", route.containerVolume.Remove)
	containerVolume.Post("/prune", route.containerVolume.Prune)

	// File routes
	file := api.Group("/file")
	file.Post("/create", route.file.Create)
	file.Get("/content", route.file.Content)
	file.Post("/save", route.file.Save)
	file.Post("/delete", route.file.Delete)
	file.Post("/upload", route.file.Upload)
	file.Post("/exist", route.file.Exist)
	file.Post("/move", route.file.Move)
	file.Post("/copy", route.file.Copy)
	file.Get("/download", route.file.Download)
	file.Post("/remote_download", route.file.RemoteDownload)
	file.Get("/info", route.file.Info)
	file.Post("/permission", route.file.Permission)
	file.Post("/compress", route.file.Compress)
	file.Post("/un_compress", route.file.UnCompress)
	file.Get("/search", route.file.Search)
	file.Get("/list", route.file.List)

	// Monitor routes
	monitor := api.Group("/monitor")
	monitor.Get("/setting", route.monitor.GetSetting)
	monitor.Post("/setting", route.monitor.UpdateSetting)
	monitor.Post("/clear", route.monitor.Clear)
	monitor.Get("/list", route.monitor.List)

	// Setting routes
	setting := api.Group("/setting")
	setting.Get("/", route.setting.Get)
	setting.Post("/", route.setting.Update)
	setting.Post("/cert", route.setting.UpdateCert)

	// Systemctl routes
	systemctl := api.Group("/systemctl")
	systemctl.Get("/status", route.systemctl.Status)
	systemctl.Get("/is_enabled", route.systemctl.IsEnabled)
	systemctl.Post("/enable", route.systemctl.Enable)
	systemctl.Post("/disable", route.systemctl.Disable)
	systemctl.Post("/restart", route.systemctl.Restart)
	systemctl.Post("/reload", route.systemctl.Reload)
	systemctl.Post("/start", route.systemctl.Start)
	systemctl.Post("/stop", route.systemctl.Stop)

	// Toolbox system routes
	toolboxSystem := api.Group("/toolbox_system")
	toolboxSystem.Get("/dns", route.toolboxSystem.GetDNS)
	toolboxSystem.Post("/dns", route.toolboxSystem.UpdateDNS)
	toolboxSystem.Get("/swap", route.toolboxSystem.GetSWAP)
	toolboxSystem.Post("/swap", route.toolboxSystem.UpdateSWAP)
	toolboxSystem.Get("/timezone", route.toolboxSystem.GetTimezone)
	toolboxSystem.Post("/timezone", route.toolboxSystem.UpdateTimezone)
	toolboxSystem.Post("/time", route.toolboxSystem.UpdateTime)
	toolboxSystem.Post("/sync_time", route.toolboxSystem.SyncTime)
	toolboxSystem.Get("/hostname", route.toolboxSystem.GetHostname)
	toolboxSystem.Post("/hostname", route.toolboxSystem.UpdateHostname)
	toolboxSystem.Get("/hosts", route.toolboxSystem.GetHosts)
	toolboxSystem.Post("/hosts", route.toolboxSystem.UpdateHosts)
	toolboxSystem.Post("/root_password", route.toolboxSystem.UpdateRootPassword)

	// Toolbox benchmark routes
	toolboxBenchmark := api.Group("/toolbox_benchmark")
	toolboxBenchmark.Post("/test", route.toolboxBenchmark.Test)

	// Apps routes
	apps := api.Group("/apps")
	route.apps.Register(apps)

	// Static file serving for frontend
	frontend, _ := fs.Sub(embed.PublicFS, "frontend")
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       frontend,
		PathPrefix: "/",
		Browse:     false,
		Index:      "index.html",
		NotFoundFile: "index.html",
	}))
}

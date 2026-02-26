package route

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/go-chi/chi/v5"

	"github.com/acepanel/panel/v3/internal/http/middleware"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/apploader"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/embed"
)

type Http struct {
	conf              *config.Config
	user              *service.UserService
	userPasskey       *service.UserPasskeyService
	userToken         *service.UserTokenService
	home              *service.HomeService
	task              *service.TaskService
	website           *service.WebsiteService
	project           *service.ProjectService
	database          *service.DatabaseService
	databaseServer    *service.DatabaseServerService
	databaseRedis     *service.DatabaseRedisService
	databaseUser      *service.DatabaseUserService
	backup            *service.BackupService
	backupStorage     *service.BackupStorageService
	cert              *service.CertService
	certDNS           *service.CertDNSService
	certAccount       *service.CertAccountService
	app               *service.AppService
	environment       *service.EnvironmentService
	environmentGo     *service.EnvironmentGoService
	environmentJava   *service.EnvironmentJavaService
	environmentNodejs *service.EnvironmentNodejsService
	environmentPHP    *service.EnvironmentPHPService
	environmentPython *service.EnvironmentPythonService
	environmentDotnet *service.EnvironmentDotnetService
	cron              *service.CronService
	process           *service.ProcessService
	safe              *service.SafeService
	firewall          *service.FirewallService
	firewallScan      *service.FirewallScanService
	ssh               *service.SSHService
	container         *service.ContainerService
	containerCompose  *service.ContainerComposeService
	containerNetwork  *service.ContainerNetworkService
	containerImage    *service.ContainerImageService
	containerVolume   *service.ContainerVolumeService
	file              *service.FileService
	log               *service.LogService
	monitor           *service.MonitorService
	setting           *service.SettingService
	systemctl         *service.SystemctlService
	toolboxNetwork    *service.ToolboxNetworkService
	toolboxSystem     *service.ToolboxSystemService
	toolboxBenchmark  *service.ToolboxBenchmarkService
	toolboxSSH        *service.ToolboxSSHService
	toolboxDisk       *service.ToolboxDiskService
	toolboxLog        *service.ToolboxLogService
	toolboxMigration  *service.ToolboxMigrationService
	webhook           *service.WebHookService
	template          *service.TemplateService
	websiteStat       *service.WebsiteStatService
	apps              *apploader.Loader
}

func NewHttp(
	conf *config.Config,
	user *service.UserService,
	userPasskey *service.UserPasskeyService,
	userToken *service.UserTokenService,
	home *service.HomeService,
	task *service.TaskService,
	website *service.WebsiteService,
	project *service.ProjectService,
	database *service.DatabaseService,
	databaseServer *service.DatabaseServerService,
	databaseRedis *service.DatabaseRedisService,
	databaseUser *service.DatabaseUserService,
	backup *service.BackupService,
	backupStorage *service.BackupStorageService,
	cert *service.CertService,
	certDNS *service.CertDNSService,
	certAccount *service.CertAccountService,
	app *service.AppService,
	environment *service.EnvironmentService,
	environmentGo *service.EnvironmentGoService,
	environmentJava *service.EnvironmentJavaService,
	environmentNodejs *service.EnvironmentNodejsService,
	environmentPHP *service.EnvironmentPHPService,
	environmentPython *service.EnvironmentPythonService,
	environmentDotnet *service.EnvironmentDotnetService,
	cron *service.CronService,
	process *service.ProcessService,
	safe *service.SafeService,
	firewall *service.FirewallService,
	firewallScan *service.FirewallScanService,
	ssh *service.SSHService,
	container *service.ContainerService,
	containerCompose *service.ContainerComposeService,
	containerNetwork *service.ContainerNetworkService,
	containerImage *service.ContainerImageService,
	containerVolume *service.ContainerVolumeService,
	file *service.FileService,
	log *service.LogService,
	monitor *service.MonitorService,
	setting *service.SettingService,
	systemctl *service.SystemctlService,
	toolboxNetwork *service.ToolboxNetworkService,
	toolboxSystem *service.ToolboxSystemService,
	toolboxBenchmark *service.ToolboxBenchmarkService,
	toolboxSSH *service.ToolboxSSHService,
	toolboxDisk *service.ToolboxDiskService,
	toolboxLog *service.ToolboxLogService,
	toolboxMigration *service.ToolboxMigrationService,
	webhook *service.WebHookService,
	template *service.TemplateService,
	websiteStat *service.WebsiteStatService,
	apps *apploader.Loader,
) *Http {
	return &Http{
		conf:              conf,
		user:              user,
		userPasskey:       userPasskey,
		userToken:         userToken,
		home:              home,
		task:              task,
		website:           website,
		project:           project,
		database:          database,
		databaseServer:    databaseServer,
		databaseRedis:     databaseRedis,
		databaseUser:      databaseUser,
		backup:            backup,
		backupStorage:     backupStorage,
		cert:              cert,
		certDNS:           certDNS,
		certAccount:       certAccount,
		app:               app,
		environment:       environment,
		environmentGo:     environmentGo,
		environmentJava:   environmentJava,
		environmentNodejs: environmentNodejs,
		environmentPHP:    environmentPHP,
		environmentPython: environmentPython,
		environmentDotnet: environmentDotnet,
		cron:              cron,
		process:           process,
		safe:              safe,
		firewall:          firewall,
		firewallScan:      firewallScan,
		ssh:               ssh,
		container:         container,
		containerCompose:  containerCompose,
		containerNetwork:  containerNetwork,
		containerImage:    containerImage,
		containerVolume:   containerVolume,
		file:              file,
		log:               log,
		monitor:           monitor,
		setting:           setting,
		systemctl:         systemctl,
		toolboxNetwork:    toolboxNetwork,
		toolboxSystem:     toolboxSystem,
		toolboxBenchmark:  toolboxBenchmark,
		toolboxSSH:        toolboxSSH,
		toolboxDisk:       toolboxDisk,
		toolboxLog:        toolboxLog,
		toolboxMigration:  toolboxMigration,
		webhook:           webhook,
		template:          template,
		websiteStat:       websiteStat,
		apps:              apps,
	}
}

func (route *Http) Register(r *chi.Mux) {
	r.Route("/api", func(r chi.Router) {
		r.Route("/user", func(r chi.Router) {
			r.Get("/key", route.user.GetKey)
			r.Get("/captcha", route.user.GetCaptcha)
			r.With(middleware.Throttle(route.conf.HTTP.IPHeader, 5, time.Minute)).Post("/login", route.user.Login)
			r.Post("/logout", route.user.Logout)
			r.Get("/is_login", route.user.IsLogin)
			r.Get("/is_2fa", route.user.IsTwoFA)
			r.Get("/info", route.user.Info)
			// 通行密钥
			r.Get("/passkey/enabled", route.userPasskey.Enabled)
			r.Post("/passkey/register", route.userPasskey.BeginRegister)
			r.Put("/passkey/register", route.userPasskey.FinishRegister)
			r.With(middleware.Throttle(route.conf.HTTP.IPHeader, 5, time.Minute)).Post("/passkey/login", route.userPasskey.BeginLogin)
			r.Put("/passkey/login", route.userPasskey.FinishLogin)
		})

		r.Route("/users", func(r chi.Router) {
			r.Get("/", route.user.List)
			r.Post("/", route.user.Create)
			r.Post("/{id}/username", route.user.UpdateUsername)
			r.Post("/{id}/password", route.user.UpdatePassword)
			r.Post("/{id}/email", route.user.UpdateEmail)
			r.Get("/{id}/2fa", route.user.GenerateTwoFA)
			r.Post("/{id}/2fa", route.user.UpdateTwoFA)
			r.Delete("/{id}", route.user.Delete)
		})

		r.Route("/user_tokens", func(r chi.Router) {
			r.Get("/", route.userToken.List)
			r.Post("/", route.userToken.Create)
			r.Put("/{id}", route.userToken.Update)
			r.Delete("/{id}", route.userToken.Delete)
		})

		r.Route("/user_passkeys", func(r chi.Router) {
			r.Get("/", route.userPasskey.List)
			r.Get("/supported", route.userPasskey.Supported)
			r.Delete("/{id}", route.userPasskey.Delete)
		})

		r.Route("/home", func(r chi.Router) {
			r.Get("/panel", route.home.Panel)
			r.Get("/apps", route.home.Apps)
			r.Post("/current", route.home.Current)
			r.Get("/system_info", route.home.SystemInfo)
			r.Get("/count_info", route.home.CountInfo)
			r.Get("/installed_environment", route.home.InstalledEnvironment)
			r.Get("/check_update", route.home.CheckUpdate)
			r.Get("/update_info", route.home.UpdateInfo)
			r.Post("/update", route.home.Update)
			r.Post("/restart", route.home.Restart)
			r.Get("/top_processes", route.home.TopProcesses)
			r.Post("/restart_server", route.home.RestartServer)
			r.Get("/runtime_info", route.home.RuntimeInfo)
			r.Get("/goroutines", route.home.Goroutines)
		})

		r.Route("/task", func(r chi.Router) {
			r.Get("/status", route.task.Status)
			r.Get("/", route.task.List)
			r.Get("/{id}", route.task.Get)
			r.Delete("/{id}", route.task.Delete)
		})

		r.Route("/website", func(r chi.Router) {
			r.Get("/rewrites", route.website.GetRewrites)
			r.Get("/default_config", route.website.GetDefaultConfig)
			r.Post("/default_config", route.website.UpdateDefaultConfig)
			r.Post("/cert", route.website.UpdateCert)
			r.Get("/", route.website.List)
			r.Post("/", route.website.Create)
			r.Get("/{id}", route.website.Get)
			r.Put("/{id}", route.website.Update)
			r.Delete("/{id}", route.website.Delete)
			r.Delete("/{id}/log", route.website.ClearLog)
			r.Post("/{id}/update_remark", route.website.UpdateRemark)
			r.Post("/{id}/reset_config", route.website.ResetConfig)
			r.Post("/{id}/status", route.website.UpdateStatus)
			r.Post("/{id}/obtain_cert", route.website.ObtainCert)

			// 网站统计
			r.Get("/stat/overview", route.websiteStat.Overview)
			r.Get("/stat/realtime", route.websiteStat.Realtime)
			r.Get("/stat/sites", route.websiteStat.SiteStats)
			r.Get("/stat/spiders", route.websiteStat.SpiderStats)
			r.Get("/stat/clients", route.websiteStat.ClientStats)
			r.Get("/stat/ips", route.websiteStat.IPStats)
			r.Get("/stat/geos", route.websiteStat.GeoStats)
			r.Get("/stat/uris", route.websiteStat.URIStats)
			r.Get("/stat/slow_uris", route.websiteStat.SlowURIStats)
			r.Get("/stat/errors", route.websiteStat.ErrorStats)
			r.Get("/stat/setting", route.websiteStat.GetSetting)
			r.Post("/stat/setting", route.websiteStat.UpdateSetting)
			r.Post("/stat/clear", route.websiteStat.Clear)
		})

		r.Route("/project", func(r chi.Router) {
			r.Get("/", route.project.List)
			r.Post("/", route.project.Create)
			r.Get("/{id}", route.project.Get)
			r.Put("/{id}", route.project.Update)
			r.Delete("/{id}", route.project.Delete)
		})

		r.Route("/database", func(r chi.Router) {
			r.Get("/", route.database.List)
			r.Post("/", route.database.Create)
			r.Delete("/", route.database.Delete)
			r.Post("/comment", route.database.Comment)
		})

		r.Route("/database_server", func(r chi.Router) {
			r.Get("/", route.databaseServer.List)
			r.Post("/", route.databaseServer.Create)
			r.Get("/{id}", route.databaseServer.Get)
			r.Put("/{id}", route.databaseServer.Update)
			r.Put("/{id}/remark", route.databaseServer.UpdateRemark)
			r.Delete("/{id}", route.databaseServer.Delete)
			r.Post("/{id}/sync", route.databaseServer.Sync)
		})

		r.Route("/database_user", func(r chi.Router) {
			r.Get("/", route.databaseUser.List)
			r.Post("/", route.databaseUser.Create)
			r.Get("/{id}", route.databaseUser.Get)
			r.Put("/{id}", route.databaseUser.Update)
			r.Put("/{id}/remark", route.databaseUser.UpdateRemark)
			r.Delete("/{id}", route.databaseUser.Delete)
		})

		r.Route("/database_redis", func(r chi.Router) {
			r.Get("/databases", route.databaseRedis.Databases)
			r.Get("/data", route.databaseRedis.Data)
			r.Get("/key", route.databaseRedis.KeyGet)
			r.Post("/key", route.databaseRedis.KeySet)
			r.Delete("/key", route.databaseRedis.KeyDelete)
			r.Post("/key/ttl", route.databaseRedis.KeyTTL)
			r.Post("/key/rename", route.databaseRedis.KeyRename)
			r.Post("/clear", route.databaseRedis.Clear)
		})

		r.Route("/backup", func(r chi.Router) {
			r.Get("/{type}", route.backup.List)
			r.Post("/{type}", route.backup.Create)
			r.Post("/{type}/upload", route.backup.Upload)
			r.Delete("/{type}/delete", route.backup.Delete)
			r.Post("/{type}/restore", route.backup.Restore)
		})

		r.Route("/backup_storage", func(r chi.Router) {
			r.Get("/", route.backupStorage.List)
			r.Post("/", route.backupStorage.Create)
			r.Put("/{id}", route.backupStorage.Update)
			r.Get("/{id}", route.backupStorage.Get)
			r.Delete("/{id}", route.backupStorage.Delete)
		})

		r.Route("/cert", func(r chi.Router) {
			r.Get("/ca_providers", route.cert.CAProviders)
			r.Get("/dns_providers", route.cert.DNSProviders)
			r.Get("/algorithms", route.cert.Algorithms)
			r.Route("/cert", func(r chi.Router) {
				r.Get("/", route.cert.List)
				r.Post("/", route.cert.Create)
				r.Post("/upload", route.cert.Upload)
				r.Put("/{id}", route.cert.Update)
				r.Get("/{id}", route.cert.Get)
				r.Delete("/{id}", route.cert.Delete)
				r.Post("/{id}/obtain_auto", route.cert.ObtainAuto)
				r.Post("/{id}/obtain_self_signed", route.cert.ObtainSelfSigned)
				r.Post("/{id}/renew", route.cert.Renew)
				r.Post("/{id}/deploy", route.cert.Deploy)
			})
			r.Route("/dns", func(r chi.Router) {
				r.Get("/", route.certDNS.List)
				r.Post("/", route.certDNS.Create)
				r.Put("/{id}", route.certDNS.Update)
				r.Get("/{id}", route.certDNS.Get)
				r.Delete("/{id}", route.certDNS.Delete)
			})
			r.Route("/account", func(r chi.Router) {
				r.Get("/", route.certAccount.List)
				r.Post("/", route.certAccount.Create)
				r.Put("/{id}", route.certAccount.Update)
				r.Get("/{id}", route.certAccount.Get)
				r.Delete("/{id}", route.certAccount.Delete)
			})
		})

		r.Route("/app", func(r chi.Router) {
			r.Get("/categories", route.app.Categories)
			r.Get("/list", route.app.List)
			r.Post("/install", route.app.Install)
			r.Post("/uninstall", route.app.Uninstall)
			r.Post("/update", route.app.Update)
			r.Post("/update_show", route.app.UpdateShow)
			r.Post("/update_order", route.app.UpdateOrder)
			r.Get("/is_installed", route.app.IsInstalled)
			r.Get("/update_cache", route.app.UpdateCache)
		})

		r.Route("/environment", func(r chi.Router) {
			r.Get("/types", route.environment.Types)
			r.Get("/list", route.environment.List)
			r.Post("/install", route.environment.Install)
			r.Post("/uninstall", route.environment.Uninstall)
			r.Post("/update", route.environment.Update)
			r.Get("/is_installed", route.environment.IsInstalled)
			r.Route("/go", func(r chi.Router) {
				r.Post("/{slug}/set_cli", route.environmentGo.SetCli)
				r.Get("/{slug}/proxy", route.environmentGo.GetProxy)
				r.Post("/{slug}/proxy", route.environmentGo.SetProxy)
			})
			r.Route("/java", func(r chi.Router) {
				r.Post("/{slug}/set_cli", route.environmentJava.SetCli)
			})
			r.Route("/nodejs", func(r chi.Router) {
				r.Post("/{slug}/set_cli", route.environmentNodejs.SetCli)
				r.Get("/{slug}/registry", route.environmentNodejs.GetRegistry)
				r.Post("/{slug}/registry", route.environmentNodejs.SetRegistry)
			})
			r.Route("/php", func(r chi.Router) {
				r.Post("/{version}/set_cli", route.environmentPHP.SetCli)
				r.Get("/{version}/phpinfo", route.environmentPHP.PHPInfo)
				r.Get("/{version}/config", route.environmentPHP.GetConfig)
				r.Post("/{version}/config", route.environmentPHP.UpdateConfig)
				r.Get("/{version}/fpm_config", route.environmentPHP.GetFPMConfig)
				r.Post("/{version}/fpm_config", route.environmentPHP.UpdateFPMConfig)
				r.Get("/{version}/load", route.environmentPHP.Load)
				r.Get("/{version}/log", route.environmentPHP.Log)
				r.Get("/{version}/slow_log", route.environmentPHP.SlowLog)
				r.Post("/{version}/clear_log", route.environmentPHP.ClearLog)
				r.Post("/{version}/clear_slow_log", route.environmentPHP.ClearSlowLog)
				r.Get("/{version}/modules", route.environmentPHP.ModuleList)
				r.Post("/{version}/modules", route.environmentPHP.InstallModule)
				r.Delete("/{version}/modules", route.environmentPHP.UninstallModule)
				r.Get("/{version}/config_tune", route.environmentPHP.GetConfigTune)
				r.Post("/{version}/config_tune", route.environmentPHP.UpdateConfigTune)
				r.Post("/{version}/clean_session", route.environmentPHP.CleanSession)
			})
			r.Route("/python", func(r chi.Router) {
				r.Post("/{slug}/set_cli", route.environmentPython.SetCli)
				r.Get("/{slug}/mirror", route.environmentPython.GetMirror)
				r.Post("/{slug}/mirror", route.environmentPython.SetMirror)
			})
			r.Route("/dotnet", func(r chi.Router) {
				r.Post("/{slug}/set_cli", route.environmentDotnet.SetCli)
			})
		})

		r.Route("/cron", func(r chi.Router) {
			r.Get("/", route.cron.List)
			r.Post("/", route.cron.Create)
			r.Put("/{id}", route.cron.Update)
			r.Get("/{id}", route.cron.Get)
			r.Delete("/{id}", route.cron.Delete)
			r.Post("/{id}/status", route.cron.Status)
		})

		r.Route("/process", func(r chi.Router) {
			r.Get("/", route.process.List)
			r.Get("/detail", route.process.Detail)
			r.Post("/kill", route.process.Kill)
			r.Post("/signal", route.process.Signal)
		})

		r.Route("/safe", func(r chi.Router) {
			r.Get("/ping", route.safe.GetPingStatus)
			r.Post("/ping", route.safe.UpdatePingStatus)
		})

		r.Route("/firewall", func(r chi.Router) {
			r.Get("/status", route.firewall.GetStatus)
			r.Post("/status", route.firewall.UpdateStatus)
			r.Get("/rule", route.firewall.GetRules)
			r.Get("/rule/port_usage", route.firewall.GetPortUsage)
			r.Post("/rule", route.firewall.CreateRule)
			r.Delete("/rule", route.firewall.DeleteRule)
			r.Get("/ip_rule", route.firewall.GetIPRules)
			r.Post("/ip_rule", route.firewall.CreateIPRule)
			r.Delete("/ip_rule", route.firewall.DeleteIPRule)
			r.Get("/forward", route.firewall.GetForwards)
			r.Post("/forward", route.firewall.CreateForward)
			r.Delete("/forward", route.firewall.DeleteForward)

			// 扫描感知
			r.Get("/scan/setting", route.firewallScan.GetSetting)
			r.Post("/scan/setting", route.firewallScan.UpdateSetting)
			r.Get("/scan/interfaces", route.firewallScan.GetInterfaces)
			r.Get("/scan/summary", route.firewallScan.GetSummary)
			r.Get("/scan/trend", route.firewallScan.GetTrend)
			r.Get("/scan/top_ips", route.firewallScan.GetTopSourceIPs)
			r.Get("/scan/top_ports", route.firewallScan.GetTopPorts)
			r.Get("/scan/events", route.firewallScan.ListEvents)
			r.Post("/scan/clear", route.firewallScan.Clear)
		})

		r.Route("/ssh", func(r chi.Router) {
			r.Get("/", route.ssh.List)
			r.Post("/", route.ssh.Create)
			r.Put("/{id}", route.ssh.Update)
			r.Get("/{id}", route.ssh.Get)
			r.Delete("/{id}", route.ssh.Delete)
		})

		r.Route("/container", func(r chi.Router) {
			r.Route("/container", func(r chi.Router) {
				r.Get("/", route.container.List)
				r.Get("/search", route.container.Search)
				r.Post("/", route.container.Create)
				r.Delete("/{id}", route.container.Remove)
				r.Post("/{id}/start", route.container.Start)
				r.Post("/{id}/stop", route.container.Stop)
				r.Post("/{id}/restart", route.container.Restart)
				r.Post("/{id}/pause", route.container.Pause)
				r.Post("/{id}/unpause", route.container.Unpause)
				r.Post("/{id}/kill", route.container.Kill)
				r.Post("/{id}/rename", route.container.Rename)
				r.Get("/{id}/logs", route.container.Logs)
				r.Post("/prune", route.container.Prune)
			})
			r.Route("/compose", func(r chi.Router) {
				r.Get("/", route.containerCompose.List)
				r.Get("/{name}", route.containerCompose.Get)
				r.Post("/", route.containerCompose.Create)
				r.Put("/{name}", route.containerCompose.Update)
				r.Post("/{name}/up", route.containerCompose.Up)
				r.Post("/{name}/down", route.containerCompose.Down)
				r.Delete("/{name}", route.containerCompose.Remove)
			})
			r.Route("/network", func(r chi.Router) {
				r.Get("/", route.containerNetwork.List)
				r.Post("/", route.containerNetwork.Create)
				r.Delete("/{id}", route.containerNetwork.Remove)
				r.Post("/prune", route.containerNetwork.Prune)
			})
			r.Route("/image", func(r chi.Router) {
				r.Get("/", route.containerImage.List)
				r.Get("/exist", route.containerImage.Exist)
				r.Post("/", route.containerImage.Pull)
				r.Delete("/{id}", route.containerImage.Remove)
				r.Post("/prune", route.containerImage.Prune)
			})
			r.Route("/volume", func(r chi.Router) {
				r.Get("/", route.containerVolume.List)
				r.Post("/", route.containerVolume.Create)
				r.Delete("/{id}", route.containerVolume.Remove)
				r.Post("/prune", route.containerVolume.Prune)
			})
		})

		r.Route("/file", func(r chi.Router) {
			r.Post("/create", route.file.Create)
			r.Get("/content", route.file.Content)
			r.Post("/save", route.file.Save)
			r.Post("/delete", route.file.Delete)
			r.Post("/upload", route.file.Upload)
			r.Post("/exist", route.file.Exist)
			r.Post("/move", route.file.Move)
			r.Post("/copy", route.file.Copy)
			r.Get("/download", route.file.Download)
			r.Post("/remote_download", route.file.RemoteDownload)
			r.Get("/info", route.file.Info)
			r.Get("/size", route.file.Size)
			r.Post("/permission", route.file.Permission)
			r.Post("/compress", route.file.Compress)
			r.Post("/un_compress", route.file.UnCompress)
			r.Get("/list", route.file.List)
			// 分块上传
			r.Post("/chunk/start", route.file.ChunkUploadStart)
			r.Post("/chunk/upload", route.file.ChunkUploadChunk)
			r.Post("/chunk/finish", route.file.ChunkUploadFinish)
		})

		r.Route("/log", func(r chi.Router) {
			r.Get("/list", route.log.List)
			r.Get("/dates", route.log.Dates)
			r.Get("/ssh", route.log.SSH)
		})

		r.Route("/monitor", func(r chi.Router) {
			r.Get("/setting", route.monitor.GetSetting)
			r.Post("/setting", route.monitor.UpdateSetting)
			r.Post("/clear", route.monitor.Clear)
			r.Get("/list", route.monitor.List)
		})

		r.Route("/setting", func(r chi.Router) {
			r.Get("/", route.setting.Get)
			r.Post("/", route.setting.Update)
			r.Post("/cert", route.setting.UpdateCert)
			r.Post("/obtain_cert", route.setting.ObtainCert)
			r.Get("/memo", route.setting.GetMemo)
			r.Post("/memo", route.setting.UpdateMemo)
		})

		r.Route("/systemctl", func(r chi.Router) {
			r.Get("/status", route.systemctl.Status)
			r.Get("/is_enabled", route.systemctl.IsEnabled)
			r.Post("/enable", route.systemctl.Enable)
			r.Post("/disable", route.systemctl.Disable)
			r.Post("/restart", route.systemctl.Restart)
			r.Post("/reload", route.systemctl.Reload)
			r.Post("/start", route.systemctl.Start)
			r.Post("/stop", route.systemctl.Stop)
		})

		r.Route("/toolbox_network", func(r chi.Router) {
			r.Get("/list", route.toolboxNetwork.List)
		})

		r.Route("/toolbox_system", func(r chi.Router) {
			r.Get("/dns", route.toolboxSystem.GetDNS)
			r.Post("/dns", route.toolboxSystem.UpdateDNS)
			r.Get("/swap", route.toolboxSystem.GetSWAP)
			r.Post("/swap", route.toolboxSystem.UpdateSWAP)
			r.Get("/timezone", route.toolboxSystem.GetTimezone)
			r.Post("/timezone", route.toolboxSystem.UpdateTimezone)
			r.Post("/time", route.toolboxSystem.UpdateTime)
			r.Post("/sync_time", route.toolboxSystem.SyncTime)
			r.Get("/ntp_servers", route.toolboxSystem.GetNTPServers)
			r.Post("/ntp_servers", route.toolboxSystem.UpdateNTPServers)
			r.Get("/hostname", route.toolboxSystem.GetHostname)
			r.Post("/hostname", route.toolboxSystem.UpdateHostname)
			r.Get("/hosts", route.toolboxSystem.GetHosts)
			r.Post("/hosts", route.toolboxSystem.UpdateHosts)
		})

		r.Route("/toolbox_benchmark", func(r chi.Router) {
			r.Post("/test", route.toolboxBenchmark.Test)
		})

		r.Route("/toolbox_ssh", func(r chi.Router) {
			r.Get("/info", route.toolboxSSH.GetInfo)
			r.Post("/port", route.toolboxSSH.UpdatePort)
			r.Post("/password_auth", route.toolboxSSH.UpdatePasswordAuth)
			r.Post("/pubkey_auth", route.toolboxSSH.UpdatePubKeyAuth)
			r.Post("/root_login", route.toolboxSSH.UpdateRootLogin)
			r.Post("/root_password", route.toolboxSSH.UpdateRootPassword)
			r.Get("/root_key", route.toolboxSSH.GetRootKey)
			r.Post("/root_key", route.toolboxSSH.GenerateRootKey)
		})

		r.Route("/toolbox_disk", func(r chi.Router) {
			r.Get("/list", route.toolboxDisk.List)
			r.Post("/partitions", route.toolboxDisk.GetPartitions)
			r.Post("/mount", route.toolboxDisk.Mount)
			r.Post("/umount", route.toolboxDisk.Umount)
			r.Post("/format", route.toolboxDisk.Format)
			r.Post("/init", route.toolboxDisk.Init)
			r.Get("/fstab", route.toolboxDisk.GetFstab)
			r.Delete("/fstab", route.toolboxDisk.DeleteFstab)
			r.Get("/lvm", route.toolboxDisk.GetLVMInfo)
			r.Post("/lvm/pv", route.toolboxDisk.CreatePV)
			r.Delete("/lvm/pv", route.toolboxDisk.RemovePV)
			r.Post("/lvm/vg", route.toolboxDisk.CreateVG)
			r.Delete("/lvm/vg", route.toolboxDisk.RemoveVG)
			r.Post("/lvm/lv", route.toolboxDisk.CreateLV)
			r.Delete("/lvm/lv", route.toolboxDisk.RemoveLV)
			r.Post("/lvm/lv/extend", route.toolboxDisk.ExtendLV)
			r.Get("/smart/disks", route.toolboxDisk.GetSmartDisks)
			r.Get("/smart/info", route.toolboxDisk.GetSmartInfo)
			r.Get("/raid/info", route.toolboxDisk.GetRaidInfo)
		})

		r.Route("/toolbox_log", func(r chi.Router) {
			r.Get("/scan", route.toolboxLog.Scan)
			r.Post("/clean", route.toolboxLog.Clean)
		})

		r.Route("/toolbox_migration", func(r chi.Router) {
			r.Get("/status", route.toolboxMigration.GetStatus)
			r.Post("/precheck", route.toolboxMigration.PreCheck)
			r.Get("/items", route.toolboxMigration.GetItems)
			r.Post("/start", route.toolboxMigration.Start)
			r.Post("/reset", route.toolboxMigration.Reset)
			r.Get("/results", route.toolboxMigration.GetResults)
			r.Get("/log", route.toolboxMigration.DownloadLog)
			r.Post("/exec", route.toolboxMigration.Exec)
		})

		r.Route("/webhook", func(r chi.Router) {
			r.Get("/", route.webhook.List)
			r.Post("/", route.webhook.Create)
			r.Put("/{id}", route.webhook.Update)
			r.Get("/{id}", route.webhook.Get)
			r.Delete("/{id}", route.webhook.Delete)
		})

		r.Route("/template", func(r chi.Router) {
			r.Get("/", route.template.List)
			r.Get("/{slug}", route.template.Get)
			r.Post("/", route.template.Create)
			r.Post("/{slug}/callback", route.template.Callback)
		})

		r.Route("/apps", func(r chi.Router) {
			route.apps.Register(r)
		})
	})

	// WebHook 调用接口
	r.Get("/webhook/{key}", route.webhook.Call)
	r.Post("/webhook/{key}", route.webhook.Call)

	r.NotFound(func(writer http.ResponseWriter, request *http.Request) {
		// /api 开头的返回 404
		if strings.HasPrefix(request.URL.Path, "/api") {
			http.NotFound(writer, request)
			return
		}
		// 其他返回前端页面
		frontend, _ := fs.Sub(embed.PublicFS, "frontend")
		spaHandler := newPrecompressedSPAHandler(http.FS(frontend))
		spaHandler.ServeHTTP(writer, request)
	})
}

func newPrecompressedSPAHandler(fsys http.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		acceptBr := strings.Contains(r.Header.Get("Accept-Encoding"), "br")

		if served := serveFileWithBr(w, r, fsys, path, acceptBr); served {
			return
		}

		// 文件不存在，SPA fallback 到 index.html
		serveFileWithBr(w, r, fsys, "/index.html", acceptBr)
	}
}

func serveFileWithBr(w http.ResponseWriter, r *http.Request, fsys http.FileSystem, path string, acceptBr bool) bool {
	name := filepath.Base(path)
	// 尝试打开 .br 版本
	if f, err := fsys.Open(path + ".br"); err == nil {
		defer func(f http.File) { _ = f.Close() }(f)
		fi, err := f.Stat()
		if err != nil || fi.IsDir() {
			return false
		}

		ct := mime.TypeByExtension(filepath.Ext(path))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Vary", "Accept-Encoding")

		if acceptBr {
			// 客户端支持 br，直接透传
			w.Header().Set("Content-Encoding", "br")
			http.ServeContent(w, r, name, fi.ModTime(), f)
		} else {
			// 客户端不支持 br，解压后返回（由中间件处理 gzip）
			decoded, err := io.ReadAll(brotli.NewReader(f))
			if err != nil {
				return false
			}
			http.ServeContent(w, r, name, fi.ModTime(), bytes.NewReader(decoded))
		}
		return true
	}

	// 回退到原始文件（字体、图片等未压缩的资源）
	f, err := fsys.Open(path)
	if err != nil {
		return false
	}
	defer func(f http.File) { _ = f.Close() }(f)
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		return false
	}
	http.ServeContent(w, r, name, fi.ModTime(), f)
	return true
}

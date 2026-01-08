package route

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/acepanel/panel/internal/http/middleware"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/apploader"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/embed"
)

type Http struct {
	conf             *config.Config
	user             *service.UserService
	userToken        *service.UserTokenService
	home             *service.HomeService
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
	environment      *service.EnvironmentService
	environmentPHP   *service.EnvironmentPHPService
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
	webhook          *service.WebHookService
	apps             *apploader.Loader
}

func NewHttp(
	conf *config.Config,
	user *service.UserService,
	userToken *service.UserTokenService,
	home *service.HomeService,
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
	environment *service.EnvironmentService,
	environmentPHP *service.EnvironmentPHPService,
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
	webhook *service.WebHookService,
	apps *apploader.Loader,
) *Http {
	return &Http{
		conf:             conf,
		user:             user,
		userToken:        userToken,
		home:             home,
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
		environment:      environment,
		environmentPHP:   environmentPHP,
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
		webhook:          webhook,
		apps:             apps,
	}
}

func (route *Http) Register(r *chi.Mux) {
	r.Route("/api", func(r chi.Router) {
		r.Route("/user", func(r chi.Router) {
			r.Get("/key", route.user.GetKey)
			r.With(middleware.Throttle(route.conf.HTTP.IPHeader, 5, time.Minute)).Post("/login", route.user.Login)
			r.Post("/logout", route.user.Logout)
			r.Get("/is_login", route.user.IsLogin)
			r.Get("/is_2fa", route.user.IsTwoFA)
			r.Get("/info", route.user.Info)
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

		r.Route("/home", func(r chi.Router) {
			r.Get("/panel", route.home.Panel)
			r.Get("/apps", route.home.Apps)
			r.Post("/current", route.home.Current)
			r.Get("/system_info", route.home.SystemInfo)
			r.Get("/count_info", route.home.CountInfo)
			r.Get("/installed_db_and_php", route.home.InstalledDbAndPhp)
			r.Get("/check_update", route.home.CheckUpdate)
			r.Get("/update_info", route.home.UpdateInfo)
			r.Post("/update", route.home.Update)
			r.Post("/restart", route.home.Restart)
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

		r.Route("/backup", func(r chi.Router) {
			r.Get("/{type}", route.backup.List)
			r.Post("/{type}", route.backup.Create)
			r.Post("/{type}/upload", route.backup.Upload)
			r.Delete("/{type}/delete", route.backup.Delete)
			r.Post("/{type}/restore", route.backup.Restore)
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
				r.Post("/{id}/obtain_manual", route.cert.ObtainManual)
				r.Post("/{id}/obtain_self_signed", route.cert.ObtainSelfSigned)
				r.Post("/{id}/renew", route.cert.Renew)
				r.Post("/{id}/manual_dns", route.cert.ManualDNS)
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
			r.Get("/is_installed", route.app.IsInstalled)
			r.Get("/update_cache", route.app.UpdateCache)
		})

		r.Route("/environment", func(r chi.Router) {
			r.Get("/types", route.environment.Types)
			r.Get("/list", route.environment.List)
			r.Post("/install", route.environment.Install)
			r.Get("/uninstall", route.environment.Uninstall)
			r.Put("/update", route.environment.Update)
			r.Get("/is_installed", route.environment.IsInstalled)
			r.Route("/php", func(r chi.Router) {
				r.Post("/{version}/set_cli", route.environmentPHP.SetCli)
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
			r.Get("/ssh", route.safe.GetSSH)
			r.Post("/ssh", route.safe.UpdateSSH)
			r.Get("/ping", route.safe.GetPingStatus)
			r.Post("/ping", route.safe.UpdatePingStatus)
		})

		r.Route("/firewall", func(r chi.Router) {
			r.Get("/status", route.firewall.GetStatus)
			r.Post("/status", route.firewall.UpdateStatus)
			r.Get("/rule", route.firewall.GetRules)
			r.Post("/rule", route.firewall.CreateRule)
			r.Delete("/rule", route.firewall.DeleteRule)
			r.Get("/ip_rule", route.firewall.GetIPRules)
			r.Post("/ip_rule", route.firewall.CreateIPRule)
			r.Delete("/ip_rule", route.firewall.DeleteIPRule)
			r.Get("/forward", route.firewall.GetForwards)
			r.Post("/forward", route.firewall.CreateForward)
			r.Delete("/forward", route.firewall.DeleteForward)
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
			r.Post("/permission", route.file.Permission)
			r.Post("/compress", route.file.Compress)
			r.Post("/un_compress", route.file.UnCompress)
			r.Get("/list", route.file.List)
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

		r.Route("/toolbox_system", func(r chi.Router) {
			r.Get("/dns", route.toolboxSystem.GetDNS)
			r.Post("/dns", route.toolboxSystem.UpdateDNS)
			r.Get("/swap", route.toolboxSystem.GetSWAP)
			r.Post("/swap", route.toolboxSystem.UpdateSWAP)
			r.Get("/timezone", route.toolboxSystem.GetTimezone)
			r.Post("/timezone", route.toolboxSystem.UpdateTimezone)
			r.Post("/time", route.toolboxSystem.UpdateTime)
			r.Post("/sync_time", route.toolboxSystem.SyncTime)
			r.Get("/hostname", route.toolboxSystem.GetHostname)
			r.Post("/hostname", route.toolboxSystem.UpdateHostname)
			r.Get("/hosts", route.toolboxSystem.GetHosts)
			r.Post("/hosts", route.toolboxSystem.UpdateHosts)
			r.Post("/root_password", route.toolboxSystem.UpdateRootPassword)
		})

		r.Route("/toolbox_benchmark", func(r chi.Router) {
			r.Post("/test", route.toolboxBenchmark.Test)
		})

		r.Route("/webhook", func(r chi.Router) {
			r.Get("/", route.webhook.List)
			r.Post("/", route.webhook.Create)
			r.Put("/{id}", route.webhook.Update)
			r.Get("/{id}", route.webhook.Get)
			r.Delete("/{id}", route.webhook.Delete)
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
		spaHandler := func(fs http.FileSystem) http.HandlerFunc {
			fileServer := http.FileServer(fs)
			return func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				f, err := fs.Open(path)
				if err != nil {
					indexFile, err := fs.Open("index.html")
					if err != nil {
						http.NotFound(w, r)
						return
					}
					defer func(indexFile http.File) {
						_ = indexFile.Close()
					}(indexFile)

					fi, err := indexFile.Stat()
					if err != nil {
						http.NotFound(w, r)
						return
					}

					http.ServeContent(w, r, "index.html", fi.ModTime(), indexFile)
					return
				}
				defer func(f http.File) {
					_ = f.Close()
				}(f)
				fileServer.ServeHTTP(w, r)
			}
		}
		spaHandler(http.FS(frontend)).ServeHTTP(writer, request)
	})
}

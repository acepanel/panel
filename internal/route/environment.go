package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// EnvironmentRoutes 运行环境路由
func EnvironmentRoutes(environmentDotnetService *service.EnvironmentDotnetService, environmentGoService *service.EnvironmentGoService, environmentJavaService *service.EnvironmentJavaService, environmentNodejsService *service.EnvironmentNodejsService, environmentPHPService *service.EnvironmentPHPService, environmentPythonService *service.EnvironmentPythonService, environmentService *service.EnvironmentService) Endpoints {
	environment := environmentService
	environmentGo := environmentGoService
	environmentJava := environmentJavaService
	environmentNodejs := environmentNodejsService
	environmentPHP := environmentPHPService
	environmentPython := environmentPythonService
	environmentDotnet := environmentDotnetService

	tags := []string{"运行环境"}

	return Endpoints{
		// 顶层
		{Method: http.MethodGet, Path: "/api/environment/types", Handler: environment.Types,
			Summary: "运行环境类型", Tags: tags},
		{Method: http.MethodGet, Path: "/api/environment/list", Handler: environment.List,
			Summary: "运行环境列表", Tags: tags},
		{Method: http.MethodPost, Path: "/api/environment/install", Handler: environment.Install,
			Summary: "安装运行环境", Tags: tags, Document: DescribeReq[request.EnvironmentAction]()},
		{Method: http.MethodPost, Path: "/api/environment/uninstall", Handler: environment.Uninstall,
			Summary: "卸载运行环境", Tags: tags, Document: DescribeReq[request.EnvironmentAction]()},
		{Method: http.MethodPost, Path: "/api/environment/update", Handler: environment.Update,
			Summary: "更新运行环境", Tags: tags, Document: DescribeReq[request.EnvironmentAction]()},
		{Method: http.MethodGet, Path: "/api/environment/is_installed", Handler: environment.IsInstalled,
			Summary: "检查运行环境是否已安装", Tags: tags, Document: DescribeReq[request.EnvironmentAction]()},

		// Go
		{Method: http.MethodPost, Path: "/api/environment/go/{slug}/set_cli", Handler: environmentGo.SetCli,
			Summary: "设置 Go 命令行", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},
		{Method: http.MethodGet, Path: "/api/environment/go/{slug}/proxy", Handler: environmentGo.GetProxy,
			Summary: "获取 Go 代理", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},
		{Method: http.MethodPost, Path: "/api/environment/go/{slug}/proxy", Handler: environmentGo.SetProxy,
			Summary: "设置 Go 代理", Tags: tags, Document: DescribeReq[request.EnvironmentProxy]()},

		// Java
		{Method: http.MethodPost, Path: "/api/environment/java/{slug}/set_cli", Handler: environmentJava.SetCli,
			Summary: "设置 Java 命令行", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},

		// Node.js
		{Method: http.MethodPost, Path: "/api/environment/nodejs/{slug}/set_cli", Handler: environmentNodejs.SetCli,
			Summary: "设置 Node.js 命令行", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},
		{Method: http.MethodGet, Path: "/api/environment/nodejs/{slug}/registry", Handler: environmentNodejs.GetRegistry,
			Summary: "获取 Node.js 镜像源", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},
		{Method: http.MethodPost, Path: "/api/environment/nodejs/{slug}/registry", Handler: environmentNodejs.SetRegistry,
			Summary: "设置 Node.js 镜像源", Tags: tags, Document: DescribeReq[request.EnvironmentRegistry]()},

		// PHP
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/set_cli", Handler: environmentPHP.SetCli,
			Summary: "设置 PHP 命令行", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/phpinfo", Handler: environmentPHP.PHPInfo,
			Summary: "获取 phpinfo", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/config", Handler: environmentPHP.GetConfig,
			Summary: "获取 PHP 配置", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/config", Handler: environmentPHP.UpdateConfig,
			Summary: "更新 PHP 配置", Tags: tags, Document: DescribeReq[request.EnvironmentPHPUpdateConfig]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/fpm_config", Handler: environmentPHP.GetFPMConfig,
			Summary: "获取 PHP-FPM 配置", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/fpm_config", Handler: environmentPHP.UpdateFPMConfig,
			Summary: "更新 PHP-FPM 配置", Tags: tags, Document: DescribeReq[request.EnvironmentPHPUpdateConfig]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/load", Handler: environmentPHP.Load,
			Summary: "获取 PHP-FPM 负载", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/log", Handler: environmentPHP.Log,
			Summary: "获取 PHP 日志路径", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/slow_log", Handler: environmentPHP.SlowLog,
			Summary: "获取 PHP 慢日志路径", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/modules", Handler: environmentPHP.ModuleList,
			Summary: "PHP 扩展列表", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/modules", Handler: environmentPHP.InstallModule,
			Summary: "安装 PHP 扩展", Tags: tags, Document: DescribeReq[request.EnvironmentPHPModule]()},
		{Method: http.MethodDelete, Path: "/api/environment/php/{version}/modules", Handler: environmentPHP.UninstallModule,
			Summary: "卸载 PHP 扩展", Tags: tags, Document: DescribeReq[request.EnvironmentPHPModule]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/config_tune", Handler: environmentPHP.GetConfigTune,
			Summary: "获取 PHP 配置调优", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/config_tune", Handler: environmentPHP.UpdateConfigTune,
			Summary: "更新 PHP 配置调优", Tags: tags, Document: DescribeReq[request.EnvironmentPHPConfigTune]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/clean_session", Handler: environmentPHP.CleanSession,
			Summary: "清理 PHP Session", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/processes", Handler: environmentPHP.Processes,
			Summary: "获取 PHP-FPM 进程列表", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/opcache", Handler: environmentPHP.Opcache,
			Summary: "获取 OPcache 状态", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/opcache/reset", Handler: environmentPHP.ResetOpcache,
			Summary: "重置 OPcache", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodGet, Path: "/api/environment/php/{version}/composer", Handler: environmentPHP.Composer,
			Summary: "获取 Composer 状态", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/composer/install", Handler: environmentPHP.InstallComposer,
			Summary: "安装 Composer", Tags: tags, Document: DescribeReq[request.EnvironmentPHPVersion]()},
		{Method: http.MethodPost, Path: "/api/environment/php/{version}/composer/mirror", Handler: environmentPHP.SetComposerMirror,
			Summary: "设置 Composer 镜像源", Tags: tags, Document: DescribeReq[request.EnvironmentPHPComposerMirror]()},

		// Python
		{Method: http.MethodPost, Path: "/api/environment/python/{slug}/set_cli", Handler: environmentPython.SetCli,
			Summary: "设置 Python 命令行", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},
		{Method: http.MethodGet, Path: "/api/environment/python/{slug}/mirror", Handler: environmentPython.GetMirror,
			Summary: "获取 Python 镜像源", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},
		{Method: http.MethodPost, Path: "/api/environment/python/{slug}/mirror", Handler: environmentPython.SetMirror,
			Summary: "设置 Python 镜像源", Tags: tags, Document: DescribeReq[request.EnvironmentMirror]()},

		// .NET
		{Method: http.MethodPost, Path: "/api/environment/dotnet/{slug}/set_cli", Handler: environmentDotnet.SetCli,
			Summary: "设置 .NET 命令行", Tags: tags, Document: DescribeReq[request.EnvironmentSlug]()},
	}
}

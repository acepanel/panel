package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// HomeRoutes 首页路由
func HomeRoutes(homeService *service.HomeService) Endpoints {
	svc := homeService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/home/panel", Handler: svc.Panel, Summary: "获取面板基础信息", Tags: []string{"首页"}, Public: true},
		{Method: http.MethodGet, Path: "/api/home/apps", Handler: svc.Apps, Summary: "获取首页展示应用", Tags: []string{"首页"}},
		{Method: http.MethodPost, Path: "/api/home/current", Handler: svc.Current, Summary: "获取实时负载", Tags: []string{"首页"}, Document: DescribeReq[request.HomeCurrent]()},
		{Method: http.MethodGet, Path: "/api/home/system_info", Handler: svc.SystemInfo, Summary: "获取系统信息", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/count_info", Handler: svc.CountInfo, Summary: "获取统计信息", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/installed_environment", Handler: svc.InstalledEnvironment, Summary: "获取已安装环境", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/check_update", Handler: svc.CheckUpdate, Summary: "检查更新", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/update_info", Handler: svc.UpdateInfo, Summary: "获取更新信息", Tags: []string{"首页"}},
		{Method: http.MethodPost, Path: "/api/home/update", Handler: svc.Update, Summary: "更新面板", Tags: []string{"首页"}},
		{Method: http.MethodPost, Path: "/api/home/restart", Handler: svc.Restart, Summary: "重启面板", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/top_processes", Handler: svc.TopProcesses, Summary: "获取占用最高进程", Tags: []string{"首页"}, Document: DescribeReq[request.HomeTopProcesses]()},
		{Method: http.MethodPost, Path: "/api/home/restart_server", Handler: svc.RestartServer, Summary: "重启服务器", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/runtime_info", Handler: svc.RuntimeInfo, Summary: "获取运行时信息", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/goroutines", Handler: svc.Goroutines, Summary: "获取协程堆栈", Tags: []string{"首页"}},
		{Method: http.MethodGet, Path: "/api/home/health", Handler: svc.Health, Summary: "获取健康问题", Tags: []string{"首页"}},
	}
}

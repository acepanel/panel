package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// SystemctlRoutes 系统服务路由
func SystemctlRoutes(systemctlService *service.SystemctlService) Endpoints {
	svc := systemctlService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/systemctl/status", Handler: svc.Status,
			Summary: "获取服务运行状态", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodGet, Path: "/api/systemctl/is_enabled", Handler: svc.IsEnabled,
			Summary: "获取服务自启状态", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodPost, Path: "/api/systemctl/enable", Handler: svc.Enable,
			Summary: "启用服务自启", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodPost, Path: "/api/systemctl/disable", Handler: svc.Disable,
			Summary: "禁用服务自启", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodPost, Path: "/api/systemctl/restart", Handler: svc.Restart,
			Summary: "重启服务", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodPost, Path: "/api/systemctl/reload", Handler: svc.Reload,
			Summary: "重载服务", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodPost, Path: "/api/systemctl/start", Handler: svc.Start,
			Summary: "启动服务", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodPost, Path: "/api/systemctl/stop", Handler: svc.Stop,
			Summary: "停止服务", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
		{Method: http.MethodPost, Path: "/api/systemctl/clear_log", Handler: svc.ClearLog,
			Summary: "清空服务日志", Tags: []string{"系统服务"},
			Document: DescribeReq[request.SystemctlService]()},
	}
}

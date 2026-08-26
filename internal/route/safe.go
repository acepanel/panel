package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// SafeRoutes 安全路由
func SafeRoutes(safeService *service.SafeService) Endpoints {
	svc := safeService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/safe/ping", Handler: svc.GetPingStatus, Summary: "获取 Ping 状态", Tags: []string{"安全"}},
		{Method: http.MethodPost, Path: "/api/safe/ping", Handler: svc.UpdatePingStatus, Summary: "更新 Ping 状态", Tags: []string{"安全"}, Document: DescribeReq[request.SafeUpdatePingStatus]()},
	}
}

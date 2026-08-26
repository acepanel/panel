package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// MonitorRoutes 监控路由
func MonitorRoutes(monitorService *service.MonitorService) Endpoints {
	svc := monitorService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/monitor/setting", Handler: svc.GetSetting, Summary: "获取监控设置", Tags: []string{"监控"}},
		{Method: http.MethodPost, Path: "/api/monitor/setting", Handler: svc.UpdateSetting, Summary: "更新监控设置", Tags: []string{"监控"}, Document: DescribeReq[request.MonitorSetting]()},
		{Method: http.MethodPost, Path: "/api/monitor/clear", Handler: svc.Clear, Summary: "清空监控数据", Tags: []string{"监控"}},
		{Method: http.MethodGet, Path: "/api/monitor/list", Handler: svc.List, Summary: "监控数据列表", Tags: []string{"监控"}, Document: DescribeReq[request.MonitorList]()},
	}
}

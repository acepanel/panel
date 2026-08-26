package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// LogRoutes 日志路由
func LogRoutes(logService *service.LogService) Endpoints {
	svc := logService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/log/list", Handler: svc.List, Summary: "日志列表", Tags: []string{"日志"}, Document: DescribeReq[request.LogList]()},
		{Method: http.MethodGet, Path: "/api/log/dates", Handler: svc.Dates, Summary: "日志日期列表", Tags: []string{"日志"}, Document: DescribeReq[request.LogDates]()},
		{Method: http.MethodGet, Path: "/api/log/ssh", Handler: svc.SSH, Summary: "SSH 登录日志", Tags: []string{"日志"}},
	}
}

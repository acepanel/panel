package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// ProcessRoutes 进程路由
func ProcessRoutes(processService *service.ProcessService) Endpoints {
	svc := processService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/process", Handler: svc.List,
			Summary: "进程列表", Tags: []string{"进程"},
			Document: DescribeReq[request.ProcessList]()},
		{Method: http.MethodGet, Path: "/api/process/detail", Handler: svc.Detail,
			Summary: "进程详情", Tags: []string{"进程"},
			Document: DescribeReq[request.ProcessDetail]()},
		{Method: http.MethodPost, Path: "/api/process/kill", Handler: svc.Kill,
			Summary: "结束进程", Tags: []string{"进程"},
			Document: DescribeReq[request.ProcessKill]()},
		{Method: http.MethodPost, Path: "/api/process/signal", Handler: svc.Signal,
			Summary: "向进程发送信号", Tags: []string{"进程"},
			Document: DescribeReq[request.ProcessSignal]()},
	}
}

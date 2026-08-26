package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// TaskRoutes 后台任务路由
func TaskRoutes(taskService *service.TaskService) Endpoints {
	svc := taskService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/task/status", Handler: svc.Status, Summary: "获取任务运行状态", Tags: []string{"任务"}},
		{Method: http.MethodGet, Path: "/api/task", Handler: svc.List, Summary: "获取任务列表", Tags: []string{"任务"}, Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.Task]]]()},
		{Method: http.MethodGet, Path: "/api/task/{id}", Handler: svc.Get, Summary: "获取任务详情", Tags: []string{"任务"}, Document: Describe[request.ID, service.Envelope[biz.Task]]()},
		{Method: http.MethodDelete, Path: "/api/task/{id}", Handler: svc.Delete, Summary: "删除任务", Tags: []string{"任务"}, Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/task/{id}/cancel", Handler: svc.Cancel, Summary: "取消任务", Tags: []string{"任务"}, Document: DescribeReq[request.ID]()},
	}
}

package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/types"
)

// ProjectRoutes 项目相关路由
func ProjectRoutes(projectService *service.ProjectService) Endpoints {
	svc := projectService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/project", Handler: svc.List,
			Summary: "项目列表", Tags: []string{"项目"},
			Document: Describe[request.Paginate, service.Envelope[service.Page[*types.ProjectDetail]]]()},
		{Method: http.MethodPost, Path: "/api/project", Handler: svc.Create,
			Summary: "创建项目", Tags: []string{"项目"},
			Document: Describe[request.ProjectCreate, service.Envelope[types.ProjectDetail]]()},
		{Method: http.MethodGet, Path: "/api/project/{id}", Handler: svc.Get,
			Summary: "获取项目详情", Tags: []string{"项目"},
			Document: Describe[request.ID, service.Envelope[types.ProjectDetail]]()},
		{Method: http.MethodPut, Path: "/api/project/{id}", Handler: svc.Update,
			Summary: "更新项目", Tags: []string{"项目"}, Document: DescribeReq[request.ProjectUpdate]()},
		{Method: http.MethodDelete, Path: "/api/project/{id}", Handler: svc.Delete,
			Summary: "删除项目", Tags: []string{"项目"}, Document: DescribeReq[request.ID]()},
	}
}

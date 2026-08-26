package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// DatabaseServerRoutes 数据库服务器路由
func DatabaseServerRoutes(databaseServerService *service.DatabaseServerService) Endpoints {
	svc := databaseServerService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/database_server", Handler: svc.List,
			Summary: "获取服务器列表", Tags: []string{"数据库服务器"},
			Document: Describe[request.DatabaseList, service.Envelope[service.Page[*biz.DatabaseServer]]]()},
		{Method: http.MethodPost, Path: "/api/database_server", Handler: svc.Create,
			Summary: "创建服务器", Tags: []string{"数据库服务器"},
			Document: DescribeReq[request.DatabaseServerCreate]()},
		{Method: http.MethodGet, Path: "/api/database_server/{id}", Handler: svc.Get,
			Summary: "获取服务器", Tags: []string{"数据库服务器"},
			Document: Describe[request.ID, service.Envelope[biz.DatabaseServer]]()},
		{Method: http.MethodPut, Path: "/api/database_server/{id}", Handler: svc.Update,
			Summary: "更新服务器", Tags: []string{"数据库服务器"},
			Document: DescribeReq[request.DatabaseServerUpdate]()},
		{Method: http.MethodPut, Path: "/api/database_server/{id}/remark", Handler: svc.UpdateRemark,
			Summary: "更新服务器备注", Tags: []string{"数据库服务器"},
			Document: DescribeReq[request.DatabaseServerUpdateRemark]()},
		{Method: http.MethodDelete, Path: "/api/database_server/{id}", Handler: svc.Delete,
			Summary: "删除服务器", Tags: []string{"数据库服务器"},
			Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/database_server/{id}/sync", Handler: svc.Sync,
			Summary: "同步服务器用户", Tags: []string{"数据库服务器"},
			Document: DescribeReq[request.ID]()},
	}
}

package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// DatabaseUserRoutes 数据库用户路由
func DatabaseUserRoutes(databaseUserService *service.DatabaseUserService) Endpoints {
	svc := databaseUserService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/database_user", Handler: svc.List,
			Summary: "获取用户列表", Tags: []string{"数据库用户"},
			Document: Describe[request.DatabaseList, service.Envelope[service.Page[*biz.DatabaseUser]]]()},
		{Method: http.MethodPost, Path: "/api/database_user", Handler: svc.Create,
			Summary: "创建用户", Tags: []string{"数据库用户"},
			Document: DescribeReq[request.DatabaseUserCreate]()},
		{Method: http.MethodGet, Path: "/api/database_user/{id}", Handler: svc.Get,
			Summary: "获取用户", Tags: []string{"数据库用户"},
			Document: Describe[request.ID, service.Envelope[biz.DatabaseUser]]()},
		{Method: http.MethodPut, Path: "/api/database_user/{id}", Handler: svc.Update,
			Summary: "更新用户", Tags: []string{"数据库用户"},
			Document: DescribeReq[request.DatabaseUserUpdate]()},
		{Method: http.MethodPut, Path: "/api/database_user/{id}/remark", Handler: svc.UpdateRemark,
			Summary: "更新用户备注", Tags: []string{"数据库用户"},
			Document: DescribeReq[request.DatabaseUserUpdateRemark]()},
		{Method: http.MethodDelete, Path: "/api/database_user/{id}", Handler: svc.Delete,
			Summary: "删除用户", Tags: []string{"数据库用户"},
			Document: DescribeReq[request.ID]()},
	}
}

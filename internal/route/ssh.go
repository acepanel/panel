package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// SSHRoutes SSH 路由
func SSHRoutes(sshService *service.SSHService) Endpoints {
	svc := sshService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/ssh", Handler: svc.List,
			Summary: "SSH 列表", Tags: []string{"SSH"},
			Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.SSH]]]()},
		{Method: http.MethodPost, Path: "/api/ssh", Handler: svc.Create,
			Summary: "创建 SSH", Tags: []string{"SSH"},
			Document: DescribeReq[request.SSHCreate]()},
		{Method: http.MethodPut, Path: "/api/ssh/{id}", Handler: svc.Update,
			Summary: "更新 SSH", Tags: []string{"SSH"},
			Document: DescribeReq[request.SSHUpdate]()},
		{Method: http.MethodGet, Path: "/api/ssh/{id}", Handler: svc.Get,
			Summary: "获取 SSH", Tags: []string{"SSH"},
			Document: Describe[request.ID, service.Envelope[biz.SSH]]()},
		{Method: http.MethodDelete, Path: "/api/ssh/{id}", Handler: svc.Delete,
			Summary: "删除 SSH", Tags: []string{"SSH"},
			Document: DescribeReq[request.ID]()},
		{Method: http.MethodGet, Path: "/api/ssh/{id}/file", Handler: svc.ListFiles,
			Summary: "浏览主机文件", Tags: []string{"SSH"},
			Document: Describe[request.SSHFile, service.Envelope[[]*biz.SSHFileInfo]]()},
		{Method: http.MethodPost, Path: "/api/ssh/{id}/mkdir", Handler: svc.Mkdir,
			Summary: "创建主机目录", Tags: []string{"SSH"},
			Document: DescribeReq[request.SSHFile]()},
	}
}

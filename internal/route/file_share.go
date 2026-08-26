package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// FileShareRoutes 文件分享路由
func FileShareRoutes(fileShareService *service.FileShareService) Endpoints {
	svc := fileShareService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/file_share", Handler: svc.List,
			Summary: "文件分享列表", Tags: []string{"文件"},
			Document: DescribeResp[service.Envelope[[]*biz.FileShare]]()},
		{Method: http.MethodPost, Path: "/api/file_share", Handler: svc.Create,
			Summary: "创建文件分享", Tags: []string{"文件"},
			Document: Describe[request.FileShareCreate, service.Envelope[biz.FileShare]]()},
		{Method: http.MethodDelete, Path: "/api/file_share/{id}", Handler: svc.Delete,
			Summary: "取消文件分享", Tags: []string{"文件"},
			Document: DescribeReq[request.ID]()},
		// 顶层免登录下载
		{Method: http.MethodGet, Path: "/download/{token}", Handler: svc.Download},
	}
}

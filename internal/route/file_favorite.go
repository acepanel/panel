package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// FileFavoriteRoutes 文件收藏路由
func FileFavoriteRoutes(fileFavoriteService *service.FileFavoriteService) Endpoints {
	svc := fileFavoriteService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/file_favorite", Handler: svc.List,
			Summary: "文件收藏列表", Tags: []string{"文件"},
			Document: DescribeResp[service.Envelope[[]*biz.FileFavorite]]()},
		{Method: http.MethodPost, Path: "/api/file_favorite", Handler: svc.Create,
			Summary: "添加文件收藏", Tags: []string{"文件"},
			Document: Describe[request.FileFavoriteCreate, service.Envelope[biz.FileFavorite]]()},
		{Method: http.MethodPut, Path: "/api/file_favorite/{id}", Handler: svc.Update,
			Summary: "更新文件收藏备注", Tags: []string{"文件"},
			Document: DescribeReq[request.FileFavoriteUpdate]()},
		{Method: http.MethodDelete, Path: "/api/file_favorite/{id}", Handler: svc.Delete,
			Summary: "删除文件收藏", Tags: []string{"文件"},
			Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/file_favorite/order", Handler: svc.UpdateOrder,
			Summary: "更新文件收藏排序", Tags: []string{"文件"},
			Document: DescribeReq[request.FileFavoriteOrder]()},
	}
}

package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// WebHookRoutes WebHook 管理与回调路由
func WebHookRoutes(webHookService *service.WebHookService) Endpoints {
	svc := webHookService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/webhook", Handler: svc.List, Summary: "WebHook 列表", Tags: []string{"WebHook"}, Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.WebHook]]]()},
		{Method: http.MethodPost, Path: "/api/webhook", Handler: svc.Create, Summary: "创建 WebHook", Tags: []string{"WebHook"}, Document: Describe[request.WebHookCreate, service.Envelope[biz.WebHook]]()},
		{Method: http.MethodPut, Path: "/api/webhook/{id}", Handler: svc.Update, Summary: "更新 WebHook", Tags: []string{"WebHook"}, Document: DescribeReq[request.WebHookUpdate]()},
		{Method: http.MethodGet, Path: "/api/webhook/{id}", Handler: svc.Get, Summary: "获取 WebHook", Tags: []string{"WebHook"}, Document: Describe[request.ID, service.Envelope[biz.WebHook]]()},
		{Method: http.MethodDelete, Path: "/api/webhook/{id}", Handler: svc.Delete, Summary: "删除 WebHook", Tags: []string{"WebHook"}, Document: DescribeReq[request.ID]()},
		// 顶层回调
		{Method: http.MethodGet, Path: "/webhook/{key}", Handler: svc.Call},
		{Method: http.MethodPost, Path: "/webhook/{key}", Handler: svc.Call},
	}
}

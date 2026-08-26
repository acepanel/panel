package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// AppRoutes 应用商店路由
func AppRoutes(appService *service.AppService) Endpoints {
	app := appService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/app/categories", Handler: app.Categories,
			Summary: "应用分类", Tags: []string{"应用"}},
		{Method: http.MethodGet, Path: "/api/app/list", Handler: app.List,
			Summary: "应用列表", Tags: []string{"应用"}},
		{Method: http.MethodPost, Path: "/api/app/install", Handler: app.Install,
			Summary: "安装应用", Tags: []string{"应用"}, Document: DescribeReq[request.App]()},
		{Method: http.MethodPost, Path: "/api/app/uninstall", Handler: app.Uninstall,
			Summary: "卸载应用", Tags: []string{"应用"}, Document: DescribeReq[request.AppSlug]()},
		{Method: http.MethodPost, Path: "/api/app/update", Handler: app.Update,
			Summary: "更新应用", Tags: []string{"应用"}, Document: DescribeReq[request.AppSlug]()},
		{Method: http.MethodPost, Path: "/api/app/update_show", Handler: app.UpdateShow,
			Summary: "更新应用首页显示", Tags: []string{"应用"}, Document: DescribeReq[request.AppUpdateShow]()},
		{Method: http.MethodPost, Path: "/api/app/update_order", Handler: app.UpdateOrder,
			Summary: "更新应用排序", Tags: []string{"应用"}, Document: DescribeReq[request.AppUpdateOrder]()},
		{Method: http.MethodGet, Path: "/api/app/is_installed", Handler: app.IsInstalled,
			Summary: "检查应用是否已安装", Tags: []string{"应用"}, Document: DescribeReq[request.AppSlugs]()},
		{Method: http.MethodGet, Path: "/api/app/custom", Handler: app.GetCustom,
			Summary: "获取自定义编译参数", Tags: []string{"应用"}, Document: DescribeReq[request.AppSlug]()},
		{Method: http.MethodPost, Path: "/api/app/custom", Handler: app.SaveCustom,
			Summary: "保存自定义编译参数", Tags: []string{"应用"}, Document: DescribeReq[request.AppCustomSave]()},
		{Method: http.MethodGet, Path: "/api/app/update_cache", Handler: app.UpdateCache,
			Summary: "更新应用缓存", Tags: []string{"应用"}},
	}
}

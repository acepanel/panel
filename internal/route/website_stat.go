package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// WebsiteStatRoutes 网站统计相关路由
func WebsiteStatRoutes(websiteStatService *service.WebsiteStatService) Endpoints {
	svc := websiteStatService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/website/stat/overview", Handler: svc.Overview,
			Summary: "统计概览", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatDateRange]()},
		{Method: http.MethodGet, Path: "/api/website/stat/realtime", Handler: svc.Realtime,
			Summary: "实时统计", Tags: []string{"网站统计"}},
		{Method: http.MethodGet, Path: "/api/website/stat/sites", Handler: svc.SiteStats,
			Summary: "网站维度汇总", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatDateRange]()},
		{Method: http.MethodGet, Path: "/api/website/stat/spiders", Handler: svc.SpiderStats,
			Summary: "蜘蛛统计", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatDateRange]()},
		{Method: http.MethodGet, Path: "/api/website/stat/clients", Handler: svc.ClientStats,
			Summary: "客户端统计", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatDateRange]()},
		{Method: http.MethodGet, Path: "/api/website/stat/ips", Handler: svc.IPStats,
			Summary: "IP 统计", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatPaginate]()},
		{Method: http.MethodGet, Path: "/api/website/stat/geos", Handler: svc.GeoStats,
			Summary: "地理位置统计", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatGeo]()},
		{Method: http.MethodGet, Path: "/api/website/stat/uris", Handler: svc.URIStats,
			Summary: "URI 统计", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatPaginate]()},
		{Method: http.MethodGet, Path: "/api/website/stat/slow_uris", Handler: svc.SlowURIStats,
			Summary: "慢请求 URI 统计", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatSlowURIs]()},
		{Method: http.MethodGet, Path: "/api/website/stat/errors", Handler: svc.ErrorStats,
			Summary: "错误统计", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatErrors]()},
		{Method: http.MethodGet, Path: "/api/website/stat/setting", Handler: svc.GetSetting,
			Summary: "获取统计设置", Tags: []string{"网站统计"}},
		{Method: http.MethodPost, Path: "/api/website/stat/setting", Handler: svc.UpdateSetting,
			Summary: "保存统计设置", Tags: []string{"网站统计"}, Document: DescribeReq[request.WebsiteStatSetting]()},
		{Method: http.MethodPost, Path: "/api/website/stat/clear", Handler: svc.Clear,
			Summary: "清空统计", Tags: []string{"网站统计"}},
	}
}

package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/types"
)

// WebsiteRoutes 网站相关路由
func WebsiteRoutes(websiteService *service.WebsiteService) Endpoints {
	svc := websiteService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/website/rewrites", Handler: svc.GetRewrites,
			Summary: "获取伪静态规则", Tags: []string{"网站"}},
		{Method: http.MethodGet, Path: "/api/website/default_config", Handler: svc.GetDefaultConfig,
			Summary: "获取默认配置", Tags: []string{"网站"}},
		{Method: http.MethodPost, Path: "/api/website/default_config", Handler: svc.UpdateDefaultConfig,
			Summary: "保存默认配置", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteDefaultConfig]()},
		{Method: http.MethodGet, Path: "/api/website/default_site", Handler: svc.GetDefaultSite,
			Summary: "获取默认站点", Tags: []string{"网站"}},
		{Method: http.MethodPost, Path: "/api/website/default_site", Handler: svc.UpdateDefaultSite,
			Summary: "设置默认站点", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteDefaultSite]()},
		{Method: http.MethodPost, Path: "/api/website/cert", Handler: svc.UpdateCert,
			Summary: "更新证书", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteUpdateCert]()},
		{Method: http.MethodGet, Path: "/api/website", Handler: svc.List,
			Summary: "网站列表", Tags: []string{"网站"},
			Document: Describe[request.WebsiteList, service.Envelope[service.Page[*biz.Website]]]()},
		{Method: http.MethodPost, Path: "/api/website", Handler: svc.Create,
			Summary: "创建网站", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteCreate]()},
		{Method: http.MethodGet, Path: "/api/website/{id}", Handler: svc.Get,
			Summary: "获取网站配置", Tags: []string{"网站"},
			Document: Describe[request.ID, service.Envelope[types.WebsiteSetting]]()},
		{Method: http.MethodPut, Path: "/api/website/{id}", Handler: svc.Update,
			Summary: "保存网站配置", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteUpdate]()},
		{Method: http.MethodPost, Path: "/api/website/{id}/switch_type", Handler: svc.SwitchType,
			Summary: "切换网站类型", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteSwitchType]()},
		{Method: http.MethodDelete, Path: "/api/website/{id}", Handler: svc.Delete,
			Summary: "删除网站", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteDelete]()},
		{Method: http.MethodPost, Path: "/api/website/{id}/update_remark", Handler: svc.UpdateRemark,
			Summary: "更新备注", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteUpdateRemark]()},
		{Method: http.MethodPost, Path: "/api/website/{id}/reset_config", Handler: svc.ResetConfig,
			Summary: "重置配置", Tags: []string{"网站"}, Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/website/{id}/status", Handler: svc.UpdateStatus,
			Summary: "修改状态", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteUpdateStatus]()},
		{Method: http.MethodPost, Path: "/api/website/{id}/expire_at", Handler: svc.UpdateExpireAt,
			Summary: "修改到期时间", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteUpdateExpireAt]()},
		{Method: http.MethodPost, Path: "/api/website/{id}/obtain_cert", Handler: svc.ObtainCert,
			Summary: "签发证书", Tags: []string{"网站"}, Document: DescribeReq[request.WebsiteObtainCert]()},
	}
}

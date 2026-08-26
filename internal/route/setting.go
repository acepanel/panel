package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// SettingRoutes 面板设置路由
func SettingRoutes(settingService *service.SettingService) Endpoints {
	svc := settingService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/setting", Handler: svc.Get, Summary: "获取面板设置", Tags: []string{"设置"}},
		{Method: http.MethodPost, Path: "/api/setting", Handler: svc.Update, Summary: "更新面板设置", Tags: []string{"设置"}, Document: DescribeReq[request.SettingPanel]()},
		{Method: http.MethodPost, Path: "/api/setting/cert", Handler: svc.UpdateCert, Summary: "更新面板证书", Tags: []string{"设置"}, Document: DescribeReq[request.SettingCert]()},
		{Method: http.MethodPost, Path: "/api/setting/obtain_cert", Handler: svc.ObtainCert, Summary: "签发面板证书", Tags: []string{"设置"}},
		{Method: http.MethodGet, Path: "/api/setting/memo", Handler: svc.GetMemo, Summary: "获取便签", Tags: []string{"设置"}},
		{Method: http.MethodPost, Path: "/api/setting/memo", Handler: svc.UpdateMemo, Summary: "更新便签", Tags: []string{"设置"}, Document: DescribeReq[request.SettingMemo]()},
	}
}

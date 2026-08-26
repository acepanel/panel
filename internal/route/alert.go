package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// AlertRoutes 告警路由
func AlertRoutes(alertService *service.AlertService) Endpoints {
	svc := alertService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/alert/rule", Handler: svc.ListRules, Summary: "告警规则列表", Tags: []string{"告警"}, Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.AlertRule]]]()},
		{Method: http.MethodPost, Path: "/api/alert/rule", Handler: svc.CreateRule, Summary: "创建告警规则", Tags: []string{"告警"}, Document: Describe[request.AlertRuleCreate, service.Envelope[biz.AlertRule]]()},
		{Method: http.MethodGet, Path: "/api/alert/rule/{id}", Handler: svc.GetRule, Summary: "获取告警规则", Tags: []string{"告警"}, Document: Describe[request.ID, service.Envelope[biz.AlertRule]]()},
		{Method: http.MethodPut, Path: "/api/alert/rule/{id}", Handler: svc.UpdateRule, Summary: "更新告警规则", Tags: []string{"告警"}, Document: DescribeReq[request.AlertRuleUpdate]()},
		{Method: http.MethodDelete, Path: "/api/alert/rule/{id}", Handler: svc.DeleteRule, Summary: "删除告警规则", Tags: []string{"告警"}, Document: DescribeReq[request.ID]()},
		{Method: http.MethodGet, Path: "/api/alert/record", Handler: svc.List, Summary: "告警记录列表", Tags: []string{"告警"}, Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.Alert]]]()},
		{Method: http.MethodPost, Path: "/api/alert/record/clear", Handler: svc.Clear, Summary: "清空告警记录", Tags: []string{"告警"}},
	}
}

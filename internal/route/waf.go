package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// WafRoutes WAF 管理路由
func WafRoutes(waf *service.WafService) Endpoints {
	return Endpoints{
		// 策略透传
		{Method: http.MethodGet, Path: "/api/waf/policies", Handler: waf.ListPolicies, Summary: "策略列表", Tags: []string{"WAF"}},
		{Method: http.MethodPost, Path: "/api/waf/policies", Handler: waf.CreatePolicy, Summary: "创建策略", Tags: []string{"WAF"}},
		{Method: http.MethodGet, Path: "/api/waf/policies/{policy_id}", Handler: waf.GetPolicy, Summary: "获取策略", Tags: []string{"WAF"}},
		{Method: http.MethodGet, Path: "/api/waf/policies/{policy_id}/status", Handler: waf.GetPolicyStatus, Summary: "获取策略应用状态", Tags: []string{"WAF"}},
		{Method: http.MethodPut, Path: "/api/waf/policies/{policy_id}", Handler: waf.UpdatePolicy, Summary: "更新策略", Tags: []string{"WAF"}},
		{Method: http.MethodDelete, Path: "/api/waf/policies/{policy_id}", Handler: waf.DeletePolicy, Summary: "删除策略", Tags: []string{"WAF"}},
		// 误报加白透传
		{Method: http.MethodGet, Path: "/api/waf/policies/{policy_id}/exclusions", Handler: waf.ListExclusions, Summary: "加白列表", Tags: []string{"WAF"}},
		{Method: http.MethodPost, Path: "/api/waf/policies/{policy_id}/exclusions", Handler: waf.CreateExclusion, Summary: "创建加白", Tags: []string{"WAF"}},
		{Method: http.MethodDelete, Path: "/api/waf/policies/{policy_id}/exclusions", Handler: waf.DeleteExclusion, Summary: "删除加白", Tags: []string{"WAF"}},
		// 决策透传（黑白名单/拉黑）
		{Method: http.MethodGet, Path: "/api/waf/decisions", Handler: waf.ListDecisions, Summary: "决策列表", Tags: []string{"WAF"}},
		{Method: http.MethodPost, Path: "/api/waf/decisions", Handler: waf.CreateDecision, Summary: "创建决策", Tags: []string{"WAF"}},
		{Method: http.MethodDelete, Path: "/api/waf/decisions", Handler: waf.DeleteDecision, Summary: "删除决策", Tags: []string{"WAF"}},
		// 报表透传
		{Method: http.MethodGet, Path: "/api/waf/events", Handler: waf.Events, Summary: "攻击事件", Tags: []string{"WAF"}},
		{Method: http.MethodGet, Path: "/api/waf/stats", Handler: waf.Stats, Summary: "统计", Tags: []string{"WAF"}},
		{Method: http.MethodGet, Path: "/api/waf/attack-map", Handler: waf.AttackMap, Summary: "攻击地图", Tags: []string{"WAF"}},
		// 网站绑定 + 启停
		{Method: http.MethodGet, Path: "/api/waf/bindings", Handler: waf.ListBindings, Summary: "绑定列表", Tags: []string{"WAF"}},
		{Method: http.MethodPost, Path: "/api/waf/website/enable", Handler: waf.EnableWebsite, Summary: "启用网站 WAF", Tags: []string{"WAF"},
			Request: request.WafWebsiteToggle{}},
		{Method: http.MethodPost, Path: "/api/waf/website/{id}/disable", Handler: waf.DisableWebsite, Summary: "停用网站 WAF", Tags: []string{"WAF"}},
	}
}

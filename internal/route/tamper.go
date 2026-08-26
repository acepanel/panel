package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// TamperRoutes 防篡改路由
func TamperRoutes(tamperService *service.TamperService) Endpoints {
	svc := tamperService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/tamper/status", Handler: svc.Status,
			Summary: "防篡改状态", Tags: []string{"防篡改"}},
		{Method: http.MethodGet, Path: "/api/tamper/setting", Handler: svc.GetSetting,
			Summary: "获取设置", Tags: []string{"防篡改"}},
		{Method: http.MethodPost, Path: "/api/tamper/setting", Handler: svc.SaveSetting,
			Summary: "保存设置", Tags: []string{"防篡改"}, Document: DescribeReq[request.TamperSetting]()},
		{Method: http.MethodPost, Path: "/api/tamper/activate_ebpf", Handler: svc.ActivateEBPF,
			Summary: "激活 eBPF 并重启", Tags: []string{"防篡改"}},
		{Method: http.MethodPost, Path: "/api/tamper/check_paths", Handler: svc.CheckPaths,
			Summary: "查询路径保护状态", Tags: []string{"防篡改"}, Document: DescribeReq[request.TamperCheckPaths]()},
		{Method: http.MethodPost, Path: "/api/tamper/protect", Handler: svc.Protect,
			Summary: "切换路径保护", Tags: []string{"防篡改"}, Document: DescribeReq[request.TamperProtect]()},
		{Method: http.MethodGet, Path: "/api/tamper/rule", Handler: svc.ListRules,
			Summary: "规则列表", Tags: []string{"防篡改"}, Document: DescribeReq[request.Paginate]()},
		{Method: http.MethodPost, Path: "/api/tamper/rule", Handler: svc.CreateRule,
			Summary: "新增规则", Tags: []string{"防篡改"}, Document: DescribeReq[request.TamperRuleCreate]()},
		{Method: http.MethodPut, Path: "/api/tamper/rule/{id}", Handler: svc.UpdateRule,
			Summary: "更新规则", Tags: []string{"防篡改"}, Document: DescribeReq[request.TamperRuleUpdate]()},
		{Method: http.MethodDelete, Path: "/api/tamper/rule/{id}", Handler: svc.DeleteRule,
			Summary: "删除规则", Tags: []string{"防篡改"}},
		{Method: http.MethodGet, Path: "/api/tamper/log", Handler: svc.ListLogs,
			Summary: "拦截日志", Tags: []string{"防篡改"}, Document: DescribeReq[request.Paginate]()},
		{Method: http.MethodDelete, Path: "/api/tamper/log", Handler: svc.ClearLogs,
			Summary: "清空日志", Tags: []string{"防篡改"}},
	}
}

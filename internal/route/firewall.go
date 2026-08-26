package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// FirewallRoutes 防火墙路由
func FirewallRoutes(firewallScanService *service.FirewallScanService, firewallService *service.FirewallService) Endpoints {
	svc := firewallService
	scan := firewallScanService

	return Endpoints{
		// /api/firewall
		{Method: http.MethodGet, Path: "/api/firewall/status", Handler: svc.GetStatus,
			Summary: "获取防火墙状态", Tags: []string{"防火墙"}},
		{Method: http.MethodPost, Path: "/api/firewall/status", Handler: svc.UpdateStatus,
			Summary: "设置防火墙状态", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallStatus]()},
		{Method: http.MethodGet, Path: "/api/firewall/rule", Handler: svc.GetRules,
			Summary: "获取端口规则", Tags: []string{"防火墙"}},
		{Method: http.MethodGet, Path: "/api/firewall/rule/port_usage", Handler: svc.GetPortUsage,
			Summary: "获取端口占用", Tags: []string{"防火墙"}},
		{Method: http.MethodGet, Path: "/api/firewall/rule/export", Handler: svc.ExportRules,
			Summary: "导出端口规则", Tags: []string{"防火墙"}},
		{Method: http.MethodPost, Path: "/api/firewall/rule/import", Handler: svc.ImportRules,
			Summary: "导入端口规则", Tags: []string{"防火墙"}},
		{Method: http.MethodPost, Path: "/api/firewall/rule", Handler: svc.CreateRule,
			Summary: "创建端口规则", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallRule]()},
		{Method: http.MethodPut, Path: "/api/firewall/rule", Handler: svc.UpdateRule,
			Summary: "更新端口规则", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallRuleUpdate]()},
		{Method: http.MethodDelete, Path: "/api/firewall/rule", Handler: svc.DeleteRule,
			Summary: "删除端口规则", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallRule]()},
		{Method: http.MethodGet, Path: "/api/firewall/ip_rule", Handler: svc.GetIPRules,
			Summary: "获取 IP 规则", Tags: []string{"防火墙"}},
		{Method: http.MethodPost, Path: "/api/firewall/ip_rule", Handler: svc.CreateIPRule,
			Summary: "创建 IP 规则", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallIPRule]()},
		{Method: http.MethodDelete, Path: "/api/firewall/ip_rule", Handler: svc.DeleteIPRule,
			Summary: "删除 IP 规则", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallIPRule]()},
		{Method: http.MethodGet, Path: "/api/firewall/forward", Handler: svc.GetForwards,
			Summary: "获取端口转发", Tags: []string{"防火墙"}},
		{Method: http.MethodPost, Path: "/api/firewall/forward", Handler: svc.CreateForward,
			Summary: "创建端口转发", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallForward]()},
		{Method: http.MethodDelete, Path: "/api/firewall/forward", Handler: svc.DeleteForward,
			Summary: "删除端口转发", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallForward]()},
		// /api/firewall/scan
		{Method: http.MethodGet, Path: "/api/firewall/scan/setting", Handler: scan.GetSetting,
			Summary: "获取扫描感知设置", Tags: []string{"防火墙"},
			Document: DescribeResp[service.Envelope[biz.ScanSetting]]()},
		{Method: http.MethodPost, Path: "/api/firewall/scan/setting", Handler: scan.UpdateSetting,
			Summary: "更新扫描感知设置", Tags: []string{"防火墙"},
			Document: DescribeReq[request.FirewallScanSetting]()},
		{Method: http.MethodGet, Path: "/api/firewall/scan/interfaces", Handler: scan.GetInterfaces,
			Summary: "获取可用网卡", Tags: []string{"防火墙"}},
		{Method: http.MethodGet, Path: "/api/firewall/scan/summary", Handler: scan.GetSummary,
			Summary: "获取扫描汇总", Tags: []string{"防火墙"},
			Document: DescribeResp[service.Envelope[biz.ScanSummary]]()},
		{Method: http.MethodGet, Path: "/api/firewall/scan/trend", Handler: scan.GetTrend,
			Summary: "获取扫描趋势", Tags: []string{"防火墙"},
			Document: DescribeResp[service.Envelope[[]*biz.ScanDayTrend]]()},
		{Method: http.MethodGet, Path: "/api/firewall/scan/top_ips", Handler: scan.GetTopSourceIPs,
			Summary: "获取 Top 扫描源 IP", Tags: []string{"防火墙"},
			Document: DescribeResp[service.Envelope[[]*biz.ScanSourceRank]]()},
		{Method: http.MethodGet, Path: "/api/firewall/scan/top_ports", Handler: scan.GetTopPorts,
			Summary: "获取 Top 被扫描端口", Tags: []string{"防火墙"},
			Document: DescribeResp[service.Envelope[[]*biz.ScanPortRank]]()},
		{Method: http.MethodGet, Path: "/api/firewall/scan/events", Handler: scan.ListEvents,
			Summary: "获取扫描事件列表", Tags: []string{"防火墙"},
			Document: DescribeResp[service.Envelope[service.Page[*biz.ScanEvent]]]()},
		{Method: http.MethodPost, Path: "/api/firewall/scan/clear", Handler: scan.Clear,
			Summary: "清空扫描数据", Tags: []string{"防火墙"}},
	}
}

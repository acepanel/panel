package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/firewall/scan"
)

type FirewallScanService struct {
	scanRepo *biz.ScanEventUsecase
}

func NewFirewallScanService(i do.Injector) (*FirewallScanService, error) {
	return &FirewallScanService{
		scanRepo: do.MustInvoke[*biz.ScanEventUsecase](i),
	}, nil
}

// GetSetting 获取扫描感知设置
func (s *FirewallScanService) GetSetting(w http.ResponseWriter, r *http.Request) {
	setting, err := s.scanRepo.GetSetting()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, setting)
}

// UpdateSetting 更新扫描感知设置
func (s *FirewallScanService) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallScanSetting](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.scanRepo.UpdateSetting(&biz.ScanSetting{
		Enabled:        req.Enabled,
		Days:           req.Days,
		Interfaces:     req.Interfaces,
		AutoBlock:      req.AutoBlock,
		BlockThreshold: req.BlockThreshold,
		BlockWindow:    req.BlockWindow,
		BlockDuration:  req.BlockDuration,
		Whitelist:      req.Whitelist,
	}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// GetInterfaces 获取可用网卡列表
func (s *FirewallScanService) GetInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := scan.ListInterfaces()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, ifaces)
}

// GetSummary 获取扫描汇总
func (s *FirewallScanService) GetSummary(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	summary, err := s.scanRepo.Summary(start, end)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, summary)
}

// GetTrend 获取扫描趋势
func (s *FirewallScanService) GetTrend(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	trends, err := s.scanRepo.Trend(start, end)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, trends)
}

// GetTopSourceIPs 获取 Top 扫描源 IP
func (s *FirewallScanService) GetTopSourceIPs(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	limit := cast.ToUint(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	ranks, err := s.scanRepo.TopSourceIPs(start, end, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, ranks)
}

// GetTopPorts 获取 Top 被扫描端口
func (s *FirewallScanService) GetTopPorts(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	limit := cast.ToUint(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	ranks, err := s.scanRepo.TopPorts(start, end, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, ranks)
}

// ListEvents 获取事件列表
func (s *FirewallScanService) ListEvents(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	sourceIP := r.URL.Query().Get("source_ip")
	port := cast.ToUint(r.URL.Query().Get("port"))
	location := r.URL.Query().Get("location")
	page := cast.ToUint(r.URL.Query().Get("page"))
	limit := cast.ToUint(r.URL.Query().Get("limit"))
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 20
	}

	items, total, err := s.scanRepo.List(start, end, sourceIP, port, location, page, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": items,
	})
}

// Clear 清空所有扫描数据
func (s *FirewallScanService) Clear(w http.ResponseWriter, r *http.Request) {
	if err := s.scanRepo.Clear(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
}

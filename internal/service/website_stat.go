package service

import (
	"cmp"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/websitestat"
)

type WebsiteStatService struct {
	t           *gotext.Locale
	log         *slog.Logger
	setting     *biz.SettingUsecase
	statRepo    *biz.WebsiteStatUsecase
	websiteRepo *biz.WebsiteUsecase
	aggregator  *websitestat.Aggregator
}

func NewWebsiteStatService(
	t *gotext.Locale,
	log *slog.Logger,
	settingUsecase *biz.SettingUsecase,
	websiteStatUsecase *biz.WebsiteStatUsecase,
	websiteUsecase *biz.WebsiteUsecase,
	aggregator *websitestat.Aggregator,
) *WebsiteStatService {
	return &WebsiteStatService{
		t:           t,
		log:         log,
		setting:     settingUsecase,
		statRepo:    websiteStatUsecase,
		websiteRepo: websiteUsecase,
		aggregator:  aggregator,
	}
}

// Overview 概览数据（汇总 + 时间序列 + 对比 + 站点列表）
func (s *WebsiteStatService) Overview(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatDateRange](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	overview, err := s.statRepo.Overview(req.Start, req.End, req.SiteList())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 获取所有网站列表（用于站点选择器）
	websites, err := s.websiteRepo.All()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	siteList := lo.Map(websites, func(ws *biz.Website, _ int) chix.M {
		return chix.M{"id": ws.ID, "name": ws.Name}
	})

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "overview"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites))

	Success(w, chix.M{
		"current":         overview.Current,
		"previous":        overview.Previous,
		"series":          overview.Series,
		"previous_series": overview.PreviousSeries,
		"sites":           siteList,
	})
}

// Realtime 实时流量/RPS
func (s *WebsiteStatService) Realtime(w http.ResponseWriter, r *http.Request) {
	rt := s.aggregator.Realtime()
	Success(w, rt)
}

// SiteStats 网站维度汇总（每站 PV/UV/IP/带宽/请求/错误/蜘蛛）
func (s *WebsiteStatService) SiteStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatDateRange](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, err := s.statRepo.ListSiteStats(req.Start, req.End, req.SiteList())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "sites"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites))

	Success(w, chix.M{
		"items": items,
	})
}

// SpiderStats 蜘蛛统计排名
func (s *WebsiteStatService) SpiderStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatDateRange](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, err := s.statRepo.TopSpiders(req.Start, req.End, req.SiteList(), 50)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 计算总请求数和百分比
	var total uint64
	for _, item := range items {
		total += item.Requests
	}
	if total > 0 {
		for _, item := range items {
			item.Percent = float64(item.Requests) / float64(total) * 100
		}
	}

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "spiders"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites))

	Success(w, chix.M{
		"items": items,
		"total": total,
	})
}

// ClientStats 客户端统计
func (s *WebsiteStatService) ClientStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatDateRange](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, err := s.statRepo.TopClients(req.Start, req.End, req.SiteList(), 100)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 按浏览器聚合
	browserMap := make(map[string]uint64)
	osMap := make(map[string]uint64)
	for _, item := range items {
		browserMap[item.Browser] += item.Requests
		osMap[item.OS] += item.Requests
	}

	type rankItem struct {
		Name     string `json:"name"`
		Requests uint64 `json:"requests"`
	}
	browsers := lo.MapToSlice(browserMap, func(name string, reqs uint64) rankItem {
		return rankItem{Name: name, Requests: reqs}
	})
	oss := lo.MapToSlice(osMap, func(name string, reqs uint64) rankItem {
		return rankItem{Name: name, Requests: reqs}
	})

	// 按请求数排序
	slices.SortFunc(browsers, func(a, b rankItem) int { return cmp.Compare(b.Requests, a.Requests) })
	slices.SortFunc(oss, func(a, b rankItem) int { return cmp.Compare(b.Requests, a.Requests) })

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "clients"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites))

	Success(w, chix.M{
		"items":    items,
		"browsers": browsers,
		"os":       oss,
	})
}

// IPStats IP 统计（分页）
func (s *WebsiteStatService) IPStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatPaginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, total, err := s.statRepo.TopIPs(req.Start, req.End, req.SiteList(), req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "ips"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"items": items,
		"total": total,
	})
}

// GeoStats 地理位置归类统计
func (s *WebsiteStatService) GeoStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatGeo](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, err := s.statRepo.TopGeos(req.Start, req.End, req.SiteList(), req.GroupBy, req.Country, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "geos"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites), slog.String("group_by", req.GroupBy), slog.String("country", req.Country), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"items": items,
	})
}

// URIStats URI 统计（分页）
func (s *WebsiteStatService) URIStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatPaginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, total, err := s.statRepo.TopURIs(req.Start, req.End, req.SiteList(), req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "uris"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"items": items,
		"total": total,
	})
}

// SlowURIStats 慢请求 URI 统计（按平均响应时间排序，分页）
func (s *WebsiteStatService) SlowURIStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatSlowURIs](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, total, err := s.statRepo.TopSlowURIs(req.Start, req.End, req.SiteList(), req.Threshold, req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "slow_uris"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites), slog.Uint64("threshold", uint64(req.Threshold)), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"items": items,
		"total": total,
	})
}

// ErrorStats 错误日志（分页 + 状态码过滤）
func (s *WebsiteStatService) ErrorStats(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatErrors](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, total, err := s.statRepo.ListErrors(req.Start, req.End, req.SiteList(), req.Status, req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website stats viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("stat", "errors"), slog.String("start", req.Start), slog.String("end", req.End), slog.String("sites", req.Sites), slog.Int("status", req.Status), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"items": items,
		"total": total,
	})
}

// Clear 清空所有统计数据
func (s *WebsiteStatService) Clear(w http.ResponseWriter, r *http.Request) {
	if err := s.statRepo.Clear(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	s.aggregator.Reset()
	Success(w, nil)
}

// GetSetting 获取统计设置
func (s *WebsiteStatService) GetSetting(w http.ResponseWriter, r *http.Request) {
	days, _ := s.setting.GetInt(biz.SettingKeyWebsiteStatDays, 30)
	errBufMax, _ := s.setting.GetInt(biz.SettingKeyWebsiteStatErrBufMax, 10000)
	uvMaxKeys, _ := s.setting.GetInt(biz.SettingKeyWebsiteStatUVMaxKeys, 1000000)
	ipMaxKeys, _ := s.setting.GetInt(biz.SettingKeyWebsiteStatIPMaxKeys, 500000)
	detailMaxKeys, _ := s.setting.GetInt(biz.SettingKeyWebsiteStatDetailMaxKeys, 50000)
	bodyEnabled, _ := s.setting.GetBool(biz.SettingKeyWebsiteStatBodyEnabled)

	s.log.Info(s.t.Get("website stat settings viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())))

	Success(w, chix.M{
		"days":            days,
		"err_buf_max":     errBufMax,
		"uv_max_keys":     uvMaxKeys,
		"ip_max_keys":     ipMaxKeys,
		"detail_max_keys": detailMaxKeys,
		"body_enabled":    bodyEnabled,
	})
}

// UpdateSetting 更新统计设置
func (s *WebsiteStatService) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatSetting](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	set := func(key biz.SettingKey, value string) {
		if err == nil {
			err = s.setting.Set(key, value)
		}
	}
	set(biz.SettingKeyWebsiteStatDays, strconv.FormatUint(uint64(req.Days), 10))
	if req.ErrBufMax > 0 {
		set(biz.SettingKeyWebsiteStatErrBufMax, strconv.Itoa(req.ErrBufMax))
	}
	if req.UVMaxKeys > 0 {
		set(biz.SettingKeyWebsiteStatUVMaxKeys, strconv.Itoa(req.UVMaxKeys))
	}
	if req.IPMaxKeys > 0 {
		set(biz.SettingKeyWebsiteStatIPMaxKeys, strconv.Itoa(req.IPMaxKeys))
	}
	if req.DetailMaxKeys > 0 {
		set(biz.SettingKeyWebsiteStatDetailMaxKeys, strconv.Itoa(req.DetailMaxKeys))
	}
	set(biz.SettingKeyWebsiteStatBodyEnabled, strconv.FormatBool(req.BodyEnabled))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website stat settings updated"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("days", uint64(req.Days)), slog.Int("err_buf_max", req.ErrBufMax), slog.Int("uv_max_keys", req.UVMaxKeys), slog.Int("ip_max_keys", req.IPMaxKeys), slog.Int("detail_max_keys", req.DetailMaxKeys), slog.Bool("body_enabled", req.BodyEnabled))

	Success(w, nil)
}

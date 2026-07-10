package service

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/websitestat"
)

type WebsiteStatService struct {
	setting     *biz.SettingUsecase
	statRepo    *biz.WebsiteStatUsecase
	websiteRepo *biz.WebsiteUsecase
	aggregator  *websitestat.Aggregator
}

func NewWebsiteStatService(i do.Injector) (*WebsiteStatService, error) {
	return &WebsiteStatService{
		setting:     do.MustInvoke[*biz.SettingUsecase](i),
		statRepo:    do.MustInvoke[*biz.WebsiteStatUsecase](i),
		websiteRepo: do.MustInvoke[*biz.WebsiteUsecase](i),
		aggregator:  do.MustInvoke[*websitestat.Aggregator](i),
	}, nil
}

// Overview 概览数据（汇总 + 时间序列 + 对比 + 站点列表）
func (s *WebsiteStatService) Overview(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteStatDateRange](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	start, end, sites := req.Start, req.End, req.SiteList()

	// 计算对比周期
	startDate, _ := time.Parse(time.DateOnly, start)
	endDate, _ := time.Parse(time.DateOnly, end)
	duration := int(endDate.Sub(startDate).Hours()/24) + 1
	prevEnd := startDate.AddDate(0, 0, -1).Format(time.DateOnly)
	prevStart := startDate.AddDate(0, 0, -duration).Format(time.DateOnly)

	isSingleDay := start == end
	today := time.Now().Format(time.DateOnly)

	// 查询当前周期汇总
	current, err := s.queryTotals(start, end, sites, today)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 查询对比周期汇总
	previous, err := s.queryTotals(prevStart, prevEnd, sites, today)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 查询时间序列
	var series []*biz.WebsiteStatSeries
	if isSingleDay {
		series, err = s.queryHourlySeries(start, sites, today)
	} else {
		series, err = s.queryDailySeries(start, end, sites, today)
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 查询对比周期时间序列
	var prevSeries []*biz.WebsiteStatSeries
	prevIsSingleDay := prevStart == prevEnd
	if prevIsSingleDay {
		prevSeries, err = s.queryHourlySeries(prevStart, sites, today)
	} else {
		prevSeries, err = s.queryDailySeries(prevStart, prevEnd, sites, today)
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 获取所有网站列表（用于站点选择器）
	websites, _, err := s.websiteRepo.List("all", 1, 10000)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	siteList := lo.Map(websites, func(ws *biz.Website, _ int) chix.M {
		return chix.M{"id": ws.ID, "name": ws.Name}
	})

	Success(w, chix.M{
		"current":         current,
		"previous":        previous,
		"series":          series,
		"previous_series": prevSeries,
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

	start, end, sites := req.Start, req.End, req.SiteList()
	today := time.Now().Format(time.DateOnly)
	includeToday := start <= today && today <= end

	siteMap := make(map[string]*biz.WebsiteStatSiteItem)

	// DB 查询全部日期范围（含今日）
	dbItems, err := s.statRepo.ListSiteStats(start, end, sites)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	for _, item := range dbItems {
		siteMap[item.Site] = item
	}

	// 叠加今日未刷新的增量
	if includeToday {
		liveStats := s.aggregator.SiteStats()
		siteSet := make(map[string]struct{}, len(sites))
		for _, name := range sites {
			siteSet[name] = struct{}{}
		}

		for name, snap := range liveStats {
			if len(sites) > 0 {
				if _, ok := siteSet[name]; !ok {
					continue
				}
			}
			if existing, ok := siteMap[name]; ok {
				existing.PV += snap.PV
				existing.UV += snap.UV
				existing.IP += snap.IP
				existing.Bandwidth += snap.Bandwidth
				existing.BandwidthIn += snap.BandwidthIn
				existing.Requests += snap.Requests
				existing.Errors += snap.Errors
				existing.Spiders += snap.Spiders
				existing.RequestTimeSum += snap.RequestTimeSum
				existing.RequestTimeCount += snap.RequestTimeCount
				existing.Status2xx += snap.Status2xx
				existing.Status3xx += snap.Status3xx
				existing.Status4xx += snap.Status4xx
				existing.Status5xx += snap.Status5xx
			} else {
				siteMap[name] = &biz.WebsiteStatSiteItem{
					Site:             name,
					PV:               snap.PV,
					UV:               snap.UV,
					IP:               snap.IP,
					Bandwidth:        snap.Bandwidth,
					BandwidthIn:      snap.BandwidthIn,
					Requests:         snap.Requests,
					Errors:           snap.Errors,
					Spiders:          snap.Spiders,
					RequestTimeSum:   snap.RequestTimeSum,
					RequestTimeCount: snap.RequestTimeCount,
					Status2xx:        snap.Status2xx,
					Status3xx:        snap.Status3xx,
					Status4xx:        snap.Status4xx,
					Status5xx:        snap.Status5xx,
				}
			}
		}
	}

	// 转换为切片
	items := lo.Values(siteMap)

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

	Success(w, chix.M{
		"items": items,
		"total": total,
	})
}

// Clear 清空所有统计数据
func (s *WebsiteStatService) Clear(w http.ResponseWriter, r *http.Request) {
	if err := s.statRepo.Clear(); err != nil {
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

	if err = s.setting.Set(biz.SettingKeyWebsiteStatDays, fmt.Sprintf("%d", req.Days)); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if req.ErrBufMax > 0 {
		_ = s.setting.Set(biz.SettingKeyWebsiteStatErrBufMax, fmt.Sprintf("%d", req.ErrBufMax))
	}
	if req.UVMaxKeys > 0 {
		_ = s.setting.Set(biz.SettingKeyWebsiteStatUVMaxKeys, fmt.Sprintf("%d", req.UVMaxKeys))
	}
	if req.IPMaxKeys > 0 {
		_ = s.setting.Set(biz.SettingKeyWebsiteStatIPMaxKeys, fmt.Sprintf("%d", req.IPMaxKeys))
	}
	if req.DetailMaxKeys > 0 {
		_ = s.setting.Set(biz.SettingKeyWebsiteStatDetailMaxKeys, fmt.Sprintf("%d", req.DetailMaxKeys))
	}
	_ = s.setting.Set(biz.SettingKeyWebsiteStatBodyEnabled, fmt.Sprintf("%t", req.BodyEnabled))

	Success(w, nil)
}

type statTotals struct {
	PV               uint64 `json:"pv"`
	UV               uint64 `json:"uv"`
	IP               uint64 `json:"ip"`
	Bandwidth        uint64 `json:"bandwidth"`
	BandwidthIn      uint64 `json:"bandwidth_in"`
	Requests         uint64 `json:"requests"`
	Errors           uint64 `json:"errors"`
	Spiders          uint64 `json:"spiders"`
	RequestTimeSum   uint64 `json:"request_time_sum"`
	RequestTimeCount uint64 `json:"request_time_count"`
	Status2xx        uint64 `json:"status_2xx"`
	Status3xx        uint64 `json:"status_3xx"`
	Status4xx        uint64 `json:"status_4xx"`
	Status5xx        uint64 `json:"status_5xx"`
}

// queryTotals 查询指定日期范围的汇总数据
// DB 包含全部日期（含今日已刷新数据），再叠加内存中未刷新的增量
func (s *WebsiteStatService) queryTotals(start, end string, sites []string, today string) (*statTotals, error) {
	var total statTotals

	// DB 查询全部日期范围（含今日）
	dbStats, err := s.statRepo.ListByDateRange(start, end, sites)
	if err != nil {
		return nil, err
	}
	for _, st := range dbStats {
		total.PV += st.PV
		total.UV += st.UV
		total.IP += st.IP
		total.Bandwidth += st.Bandwidth
		total.BandwidthIn += st.BandwidthIn
		total.Requests += st.Requests
		total.Errors += st.Errors
		total.Spiders += st.Spiders
		total.RequestTimeSum += st.RequestTimeSum
		total.RequestTimeCount += st.RequestTimeCount
		total.Status2xx += st.Status2xx
		total.Status3xx += st.Status3xx
		total.Status4xx += st.Status4xx
		total.Status5xx += st.Status5xx
	}

	// 叠加今日未刷新的增量
	if start <= today && today <= end {
		s.mergeLiveTotals(&total, sites)
	}

	return &total, nil
}

// mergeLiveTotals 合并内存中今日实时数据到汇总
func (s *WebsiteStatService) mergeLiveTotals(total *statTotals, sites []string) {
	liveStats := s.aggregator.SiteStats()
	siteSet := make(map[string]struct{}, len(sites))
	for _, name := range sites {
		siteSet[name] = struct{}{}
	}

	for name, snap := range liveStats {
		if len(sites) > 0 {
			if _, ok := siteSet[name]; !ok {
				continue
			}
		}
		total.PV += snap.PV
		total.UV += snap.UV
		total.IP += snap.IP
		total.Bandwidth += snap.Bandwidth
		total.BandwidthIn += snap.BandwidthIn
		total.Requests += snap.Requests
		total.Errors += snap.Errors
		total.Spiders += snap.Spiders
		total.RequestTimeSum += snap.RequestTimeSum
		total.RequestTimeCount += snap.RequestTimeCount
		total.Status2xx += snap.Status2xx
		total.Status3xx += snap.Status3xx
		total.Status4xx += snap.Status4xx
		total.Status5xx += snap.Status5xx
	}
}

// queryHourlySeries 查询小时级时间序列
func (s *WebsiteStatService) queryHourlySeries(date string, sites []string, today string) ([]*biz.WebsiteStatSeries, error) {
	hourMap := make(map[int]*biz.WebsiteStatSeries, 24)

	// 从 DB 查询小时数据
	dbSeries, err := s.statRepo.HourlySeries(date, sites)
	if err != nil {
		return nil, err
	}
	for _, item := range dbSeries {
		h, _ := strconv.Atoi(item.Key)
		hourMap[h] = item
	}

	// 今天叠加内存中未刷新的增量
	if date == today {
		liveStats := s.aggregator.SiteStats()
		siteSet := make(map[string]struct{}, len(sites))
		for _, name := range sites {
			siteSet[name] = struct{}{}
		}

		for name, snap := range liveStats {
			if len(sites) > 0 {
				if _, ok := siteSet[name]; !ok {
					continue
				}
			}
			for h, hs := range snap.Hours {
				if hs == nil {
					continue
				}
				if existing, ok := hourMap[h]; ok {
					existing.PV += hs.PV
					existing.UV += hs.UV
					existing.IP += hs.IP
					existing.Bandwidth += hs.Bandwidth
					existing.BandwidthIn += hs.BandwidthIn
					existing.Requests += hs.Requests
					existing.Errors += hs.Errors
					existing.Spiders += hs.Spiders
					existing.RequestTimeSum += hs.RequestTimeSum
					existing.RequestTimeCount += hs.RequestTimeCount
					existing.Status2xx += hs.Status2xx
					existing.Status3xx += hs.Status3xx
					existing.Status4xx += hs.Status4xx
					existing.Status5xx += hs.Status5xx
				} else {
					hourMap[h] = &biz.WebsiteStatSeries{
						Key:              strconv.Itoa(h),
						PV:               hs.PV,
						UV:               hs.UV,
						IP:               hs.IP,
						Bandwidth:        hs.Bandwidth,
						BandwidthIn:      hs.BandwidthIn,
						Requests:         hs.Requests,
						Errors:           hs.Errors,
						Spiders:          hs.Spiders,
						RequestTimeSum:   hs.RequestTimeSum,
						RequestTimeCount: hs.RequestTimeCount,
						Status2xx:        hs.Status2xx,
						Status3xx:        hs.Status3xx,
						Status4xx:        hs.Status4xx,
						Status5xx:        hs.Status5xx,
					}
				}
			}
		}
	}

	// 填充完整 24 小时
	result := make([]*biz.WebsiteStatSeries, 24)
	for h := range 24 {
		key := strconv.Itoa(h)
		if s, ok := hourMap[h]; ok {
			result[h] = s
		} else {
			result[h] = &biz.WebsiteStatSeries{Key: key}
		}
	}

	return result, nil
}

// queryDailySeries 查询日级时间序列
func (s *WebsiteStatService) queryDailySeries(start, end string, sites []string, today string) ([]*biz.WebsiteStatSeries, error) {
	includeToday := start <= today && today <= end

	// DB 查询全部日期范围（含今日）
	dateMap := make(map[string]*biz.WebsiteStatSeries)
	dbSeries, err := s.statRepo.DailySeries(start, end, sites)
	if err != nil {
		return nil, err
	}
	for _, item := range dbSeries {
		dateMap[item.Key] = item
	}

	// 今天叠加内存中未刷新的增量
	if includeToday {
		liveStats := s.aggregator.SiteStats()
		siteSet := make(map[string]struct{}, len(sites))
		for _, name := range sites {
			siteSet[name] = struct{}{}
		}

		todayData := dateMap[today]
		if todayData == nil {
			todayData = &biz.WebsiteStatSeries{Key: today}
			dateMap[today] = todayData
		}
		for name, snap := range liveStats {
			if len(sites) > 0 {
				if _, ok := siteSet[name]; !ok {
					continue
				}
			}
			todayData.PV += snap.PV
			todayData.UV += snap.UV
			todayData.IP += snap.IP
			todayData.Bandwidth += snap.Bandwidth
			todayData.BandwidthIn += snap.BandwidthIn
			todayData.Requests += snap.Requests
			todayData.Errors += snap.Errors
			todayData.Spiders += snap.Spiders
			todayData.RequestTimeSum += snap.RequestTimeSum
			todayData.RequestTimeCount += snap.RequestTimeCount
			todayData.Status2xx += snap.Status2xx
			todayData.Status3xx += snap.Status3xx
			todayData.Status4xx += snap.Status4xx
			todayData.Status5xx += snap.Status5xx
		}
	}

	// 填充日期范围内所有天
	startDate, _ := time.Parse(time.DateOnly, start)
	endDate, _ := time.Parse(time.DateOnly, end)
	var result []*biz.WebsiteStatSeries
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		key := d.Format(time.DateOnly)
		if s, ok := dateMap[key]; ok {
			result = append(result, s)
		} else {
			result = append(result, &biz.WebsiteStatSeries{Key: key})
		}
	}

	return result, nil
}

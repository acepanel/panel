package job

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/geoip"
	"github.com/acepanel/panel/v3/pkg/websitestat"
)

// healthKeyStatDB 网站统计辅助数据库健康问题上报 key
const healthKeyStatDB = "database:stat"

// WebsiteStat 网站统计后台任务
type WebsiteStat struct {
	log          *slog.Logger
	setting      *biz.SettingUsecase
	statRepo     *biz.WebsiteStatUsecase
	t            *gotext.Locale
	aggregator   *websitestat.Aggregator
	geoIP        *geoip.GeoIP
	geoIPPath    string
	geoIPModTime time.Time
	started      atomic.Bool
	cleanedAt    time.Time
}

// NewWebsiteStat 构造网站统计任务
func NewWebsiteStat(settingUsecase *biz.SettingUsecase, websiteStatUsecase *biz.WebsiteStatUsecase, t *gotext.Locale, log *slog.Logger, aggregator *websitestat.Aggregator) Job {
	return Job{
		Spec: "* * * * *",
		Task: &WebsiteStat{
			log:        log,
			setting:    settingUsecase,
			statRepo:   websiteStatUsecase,
			t:          t,
			aggregator: aggregator,
		},
	}
}

func (r *WebsiteStat) Run(_ context.Context) error {
	if app.Status != app.StatusNormal {
		return nil
	}

	r.ensureListener()
	r.geoIP, r.geoIPPath, r.geoIPModTime = refreshGeoIP(r.setting, r.geoIP, r.geoIPPath, r.geoIPModTime, r.t, r.log)
	r.flush()
	r.flushErrors()
	r.flushDetails()
	r.cleanup()
	return nil
}

// ensureListener 确保 listener goroutine 已启动
func (r *WebsiteStat) ensureListener() {
	// 防止重复启动
	if !r.started.CompareAndSwap(false, true) {
		return
	}

	if v, err := r.setting.GetInt(biz.SettingKeyWebsiteStatErrBufMax); err == nil && v > 0 {
		r.aggregator.ErrBufMaxSize = v
	}
	if v, err := r.setting.GetInt(biz.SettingKeyWebsiteStatUVMaxKeys); err == nil && v > 0 {
		r.aggregator.UVMaxKeys = v
	}
	if v, err := r.setting.GetInt(biz.SettingKeyWebsiteStatIPMaxKeys); err == nil && v > 0 {
		r.aggregator.IPMaxKeys = v
	}
	if v, err := r.setting.GetInt(biz.SettingKeyWebsiteStatDetailMaxKeys); err == nil && v > 0 {
		r.aggregator.DetailMaxKeys = v
	}
	if v, err := r.setting.GetBool(biz.SettingKeyWebsiteStatBodyEnabled); err == nil {
		r.aggregator.BodyEnabled = v
	}

	listener, err := websitestat.NewListener("/tmp/ace_stats.sock", r.log)
	if err != nil {
		r.log.Warn(r.t.Get("failed to start website stat listener"), slog.Any("err", err))
		r.started.Store(false)
		return
	}

	go r.readLoop(listener)
}

// readLoop 持续读取 syslog 消息并记录到聚合器
func (r *WebsiteStat) readLoop(listener *websitestat.Listener) {
	for {
		tag, data, err := listener.Read()
		if err != nil {
			r.log.Warn(r.t.Get("failed to read from website stat listener"), slog.Any("err", err))
			_ = listener.Close()
			r.started.Store(false)
			return
		}

		entry, err := websitestat.ParseLogEntry(tag, data)
		if err != nil {
			continue
		}

		r.aggregator.Record(entry)
	}
}

// flush 将增量快照写入数据库（含每日汇总和小时数据）
func (r *WebsiteStat) flush() {
	snapshotsByDate, commit := r.aggregator.DrainSnapshot()
	if len(snapshotsByDate) == 0 {
		return
	}

	now := time.Now()
	var stats []*biz.WebsiteStat

	for date, snapshots := range snapshotsByDate {
		for site, snap := range snapshots {
			siteHasData := !isZeroSiteSnapshot(snap)

			// 每日汇总行 (hour = -1)
			if siteHasData {
				stats = append(stats, &biz.WebsiteStat{
					Site:             site,
					Date:             date,
					Hour:             -1,
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
					UpdatedAt:        now,
				})
			}

			// 每小时行 (hour = 0-23)
			for h, hs := range snap.Hours {
				if hs == nil || isZeroHourSnapshot(hs) {
					continue
				}
				stats = append(stats, &biz.WebsiteStat{
					Site:             site,
					Date:             date,
					Hour:             h,
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
					UpdatedAt:        now,
				})
			}
		}
	}
	if len(stats) == 0 {
		commit()
		return
	}

	if err := r.statRepo.Upsert(stats); err != nil {
		r.log.Warn(r.t.Get("failed to upsert website stats"), slog.Any("err", err))
		app.Health.Report(healthKeyStatDB, app.HealthLevelError, err.Error())
		// 直接丢弃已 drain 的增量，避免 DB 长期不可写时内存无界累积
		commit()
		return
	}
	commit()
	app.Health.Clear(healthKeyStatDB)
}

// flushErrors 将错误日志缓冲写入数据库
func (r *WebsiteStat) flushErrors() {
	entries, commit := r.aggregator.DrainErrors()
	if len(entries) == 0 {
		return
	}

	now := time.Now()
	errors := lo.Map(entries, func(e *websitestat.ErrorEntry, _ int) *biz.WebsiteErrorLog {
		return &biz.WebsiteErrorLog{
			Site:      e.Site,
			URI:       e.URI,
			Method:    e.Method,
			Status:    e.Status,
			IP:        e.IP,
			UA:        e.UA,
			Body:      e.Body,
			CreatedAt: now,
		}
	})

	if err := r.statRepo.InsertErrors(errors); err != nil {
		r.log.Warn(r.t.Get("failed to insert website error logs"), slog.Any("err", err))
		app.Health.Report(healthKeyStatDB, app.HealthLevelError, err.Error())
		// 直接丢弃已 snapshot 的错误缓冲，避免内存无界累积
		commit()
		return
	}
	commit()
	app.Health.Clear(healthKeyStatDB)
}

// flushDetails 将详细统计增量写入数据库（蜘蛛/客户端/IP/URI）
func (r *WebsiteStat) flushDetails() {
	detailsByDate, commit := r.aggregator.DrainDetailStats()
	if len(detailsByDate) == 0 {
		return
	}

	now := time.Now()

	var spiders []*biz.WebsiteStatSpider
	var clients []*biz.WebsiteStatClient
	var ips []*biz.WebsiteStatIP
	var uris []*biz.WebsiteStatURI

	for date, details := range detailsByDate {
		for site, snap := range details {
			for spider, requests := range snap.Spiders {
				spiders = append(spiders, &biz.WebsiteStatSpider{
					Site:      site,
					Date:      date,
					Spider:    spider,
					Requests:  requests,
					UpdatedAt: now,
				})
			}

			for key, cc := range snap.Clients {
				parts := strings.SplitN(key, "|", 2)
				if len(parts) != 2 {
					continue
				}
				clients = append(clients, &biz.WebsiteStatClient{
					Site:      site,
					Date:      date,
					Browser:   parts[0],
					OS:        parts[1],
					Requests:  cc.Requests,
					UpdatedAt: now,
				})
			}

			for ip, ic := range snap.IPs {
				rec := &biz.WebsiteStatIP{
					Site:      site,
					Date:      date,
					IP:        ip,
					Requests:  ic.Requests,
					Bandwidth: ic.Bandwidth,
					UpdatedAt: now,
				}
				if r.geoIP != nil {
					geo := r.geoIP.Lookup(ip)
					rec.Country = geo.CountryCode
					rec.Region = geo.Region
					rec.City = geo.City
					rec.ISP = geo.ISP
				}
				ips = append(ips, rec)
			}

			for uri, uc := range snap.URIs {
				uris = append(uris, &biz.WebsiteStatURI{
					Site:             site,
					Date:             date,
					URI:              uri,
					Requests:         uc.Requests,
					Bandwidth:        uc.Bandwidth,
					Errors:           uc.Errors,
					RequestTimeSum:   uc.RequestTimeSum,
					RequestTimeCount: uc.RequestTimeCount,
					UpdatedAt:        now,
				})
			}
		}
	}

	var lastErr error
	if err := r.statRepo.UpsertSpiders(spiders); err != nil {
		r.log.Warn(r.t.Get("failed to upsert spider stats"), slog.Any("err", err))
		lastErr = err
	}
	if err := r.statRepo.UpsertClients(clients); err != nil {
		r.log.Warn(r.t.Get("failed to upsert client stats"), slog.Any("err", err))
		lastErr = err
	}
	if err := r.statRepo.UpsertIPs(ips); err != nil {
		r.log.Warn(r.t.Get("failed to upsert ip stats"), slog.Any("err", err))
		lastErr = err
	}
	if err := r.statRepo.UpsertURIs(uris); err != nil {
		r.log.Warn(r.t.Get("failed to upsert uri stats"), slog.Any("err", err))
		lastErr = err
	}
	// 无论成败都 commit：成功→减去已入库增量；失败→直接丢弃，避免内存无界累积
	commit()
	if lastErr != nil {
		app.Health.Report(healthKeyStatDB, app.HealthLevelError, lastErr.Error())
	} else {
		app.Health.Clear(healthKeyStatDB)
	}
}

// cleanup 清理过期数据
// 任务每分钟触发，但数据按天过期，故限流到 6 小时一次，避免高频 DELETE 抢占单连接写锁
func (r *WebsiteStat) cleanup() {
	if time.Since(r.cleanedAt) < 6*time.Hour {
		return
	}
	r.cleanedAt = time.Now()

	days, err := r.setting.GetInt(biz.SettingKeyWebsiteStatDays, 30)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -days).Format(time.DateOnly)
	if err = r.statRepo.ClearBefore(cutoff); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired website stats"), slog.Any("err", err))
	}

	errCutoff := time.Now().AddDate(0, 0, -days)
	if err = r.statRepo.ClearErrorsBefore(errCutoff); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired website error logs"), slog.Any("err", err))
	}

	// 清理详细统计表
	if err = r.statRepo.ClearSpidersBefore(cutoff); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired spider stats"), slog.Any("err", err))
	}
	if err = r.statRepo.ClearClientsBefore(cutoff); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired client stats"), slog.Any("err", err))
	}
	if err = r.statRepo.ClearIPsBefore(cutoff); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired ip stats"), slog.Any("err", err))
	}
	if err = r.statRepo.ClearURIsBefore(cutoff); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired uri stats"), slog.Any("err", err))
	}
}

// isZeroSiteSnapshot 判断站点快照是否全为零
func isZeroSiteSnapshot(s *websitestat.SiteSnapshot) bool {
	return s.PV == 0 &&
		s.UV == 0 &&
		s.IP == 0 &&
		s.Bandwidth == 0 &&
		s.BandwidthIn == 0 &&
		s.Requests == 0 &&
		s.Errors == 0 &&
		s.Spiders == 0 &&
		s.RequestTimeSum == 0 &&
		s.RequestTimeCount == 0 &&
		s.Status2xx == 0 &&
		s.Status3xx == 0 &&
		s.Status4xx == 0 &&
		s.Status5xx == 0
}

// isZeroHourSnapshot 判断小时快照是否全为零
func isZeroHourSnapshot(h *websitestat.HourSnapshot) bool {
	return h.PV == 0 &&
		h.UV == 0 &&
		h.IP == 0 &&
		h.Bandwidth == 0 &&
		h.BandwidthIn == 0 &&
		h.Requests == 0 &&
		h.Errors == 0 &&
		h.Spiders == 0 &&
		h.RequestTimeSum == 0 &&
		h.RequestTimeCount == 0 &&
		h.Status2xx == 0 &&
		h.Status3xx == 0 &&
		h.Status4xx == 0 &&
		h.Status5xx == 0
}

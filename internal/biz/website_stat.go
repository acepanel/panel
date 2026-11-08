package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
)

// WebsiteStat 网站统计（每站每天一行或每小时一行）
type WebsiteStat struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Site             string    `gorm:"not null;uniqueIndex:idx_wstat_unique" json:"site"`
	Date             string    `gorm:"not null;uniqueIndex:idx_wstat_unique;index" json:"date"`
	Hour             int       `gorm:"not null;uniqueIndex:idx_wstat_unique;default:-1" json:"hour"` // -1=每日汇总, 0-23=小时
	PV               uint64    `gorm:"not null;default:0" json:"pv"`
	UV               uint64    `gorm:"not null;default:0" json:"uv"`
	IP               uint64    `gorm:"not null;default:0" json:"ip"`
	Bandwidth        uint64    `gorm:"not null;default:0" json:"bandwidth"`
	BandwidthIn      uint64    `gorm:"not null;default:0" json:"bandwidth_in"`
	Requests         uint64    `gorm:"not null;default:0" json:"requests"`
	Errors           uint64    `gorm:"not null;default:0" json:"errors"`
	Spiders          uint64    `gorm:"not null;default:0" json:"spiders"`
	RequestTimeSum   uint64    `gorm:"not null;default:0" json:"request_time_sum"`
	RequestTimeCount uint64    `gorm:"not null;default:0" json:"request_time_count"`
	Status2xx        uint64    `gorm:"not null;default:0" json:"status_2xx"`
	Status3xx        uint64    `gorm:"not null;default:0" json:"status_3xx"`
	Status4xx        uint64    `gorm:"not null;default:0" json:"status_4xx"`
	Status5xx        uint64    `gorm:"not null;default:0" json:"status_5xx"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// WebsiteErrorLog 网站错误日志（400-599 状态码详情）
type WebsiteErrorLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Site      string    `gorm:"not null;index:idx_werr_site_time" json:"site"`
	URI       string    `gorm:"not null;serializer:zstd" json:"uri"`
	Method    string    `gorm:"not null" json:"method"`
	Status    int       `gorm:"not null;index" json:"status"`
	IP        string    `gorm:"not null" json:"ip"`
	UA        string    `gorm:"not null;serializer:zstd" json:"ua"`
	Body      string    `gorm:"type:text;serializer:zstd" json:"body"`
	CreatedAt time.Time `gorm:"index:idx_werr_site_time" json:"created_at"`
}

// WebsiteStatSpider 蜘蛛统计（site, date, spider 唯一）
type WebsiteStatSpider struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Site      string    `gorm:"not null;uniqueIndex:idx_wspider_unique" json:"site"`
	Date      string    `gorm:"not null;uniqueIndex:idx_wspider_unique;index" json:"date"`
	Spider    string    `gorm:"not null;uniqueIndex:idx_wspider_unique" json:"spider"`
	Requests  uint64    `gorm:"not null;default:0" json:"requests"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebsiteStatClient 客户端统计（site, date, browser, os 唯一）
type WebsiteStatClient struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Site      string    `gorm:"not null;uniqueIndex:idx_wclient_unique" json:"site"`
	Date      string    `gorm:"not null;uniqueIndex:idx_wclient_unique;index" json:"date"`
	Browser   string    `gorm:"not null;uniqueIndex:idx_wclient_unique" json:"browser"`
	OS        string    `gorm:"not null;uniqueIndex:idx_wclient_unique" json:"os"`
	Requests  uint64    `gorm:"not null;default:0" json:"requests"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebsiteStatIP IP 统计（site, date, ip 唯一）
type WebsiteStatIP struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Site      string    `gorm:"not null;uniqueIndex:idx_wip_unique" json:"site"`
	Date      string    `gorm:"not null;uniqueIndex:idx_wip_unique;index" json:"date"`
	IP        string    `gorm:"not null;uniqueIndex:idx_wip_unique" json:"ip"`
	Country   string    `gorm:"not null;default:''" json:"country"`
	Region    string    `gorm:"not null;default:''" json:"region"`
	City      string    `gorm:"not null;default:''" json:"city"`
	ISP       string    `gorm:"not null;default:''" json:"isp"`
	Requests  uint64    `gorm:"not null;default:0" json:"requests"`
	Bandwidth uint64    `gorm:"not null;default:0" json:"bandwidth"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebsiteStatURI URI 统计（site, date, uri 唯一）
type WebsiteStatURI struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Site             string    `gorm:"not null;uniqueIndex:idx_wuri_unique" json:"site"`
	Date             string    `gorm:"not null;uniqueIndex:idx_wuri_unique;index" json:"date"`
	URI              string    `gorm:"not null;uniqueIndex:idx_wuri_unique" json:"uri"`
	Requests         uint64    `gorm:"not null;default:0" json:"requests"`
	Bandwidth        uint64    `gorm:"not null;default:0" json:"bandwidth"`
	Errors           uint64    `gorm:"not null;default:0" json:"errors"`
	RequestTimeSum   uint64    `gorm:"not null;default:0" json:"request_time_sum"`
	RequestTimeCount uint64    `gorm:"not null;default:0" json:"request_time_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// WebsiteStatSeries 时间序列数据点（用于 API 返回）
type WebsiteStatSeries struct {
	Key              string `json:"key"` // 小时 "0"-"23" 或日期 "2026-02-18"
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

// WebsiteStatSpiderRank 蜘蛛排名
type WebsiteStatSpiderRank struct {
	Spider   string  `json:"spider"`
	Requests uint64  `json:"requests"`
	Percent  float64 `json:"percent"`
}

// WebsiteStatClientRank 客户端排名
type WebsiteStatClientRank struct {
	Browser  string `json:"browser"`
	OS       string `json:"os"`
	Requests uint64 `json:"requests"`
}

// WebsiteStatIPRank IP 排名
type WebsiteStatIPRank struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	Region    string `json:"region"`
	City      string `json:"city"`
	ISP       string `json:"isp"`
	Requests  uint64 `json:"requests"`
	Bandwidth uint64 `json:"bandwidth"`
}

// WebsiteStatGeoRank 地理位置归类统计
type WebsiteStatGeoRank struct {
	Country   string `json:"country"`
	Region    string `json:"region"`
	City      string `json:"city"`
	Requests  uint64 `json:"requests"`
	Bandwidth uint64 `json:"bandwidth"`
}

// WebsiteStatURIRank URI 排名
type WebsiteStatURIRank struct {
	URI              string `json:"uri"`
	Requests         uint64 `json:"requests"`
	Bandwidth        uint64 `json:"bandwidth"`
	Errors           uint64 `json:"errors"`
	RequestTimeSum   uint64 `json:"request_time_sum"`
	RequestTimeCount uint64 `json:"request_time_count"`
}

// WebsiteStatSiteItem 网站维度汇总
type WebsiteStatSiteItem struct {
	Site             string `json:"site"`
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

// WebsiteStatRepo 网站统计数据访问接口
type WebsiteStatRepo interface {
	// Upsert 批量写入/更新统计数据（含每日汇总和小时数据）
	Upsert(stats []*WebsiteStat) error
	// ListByDateRange 按日期范围查询每站汇总（仅 hour=-1 的每日行）
	ListByDateRange(start, end string, sites []string) ([]*WebsiteStat, error)
	// DailySeries 按日分组的时间序列
	DailySeries(start, end string, sites []string) ([]*WebsiteStatSeries, error)
	// HourlySeries 按小时的时间序列（单天查询）
	HourlySeries(date string, sites []string) ([]*WebsiteStatSeries, error)
	// ClearBefore 清理过期数据
	ClearBefore(date string) error
	// InsertErrors 批量写入错误日志
	InsertErrors(errors []*WebsiteErrorLog) error
	// ClearErrorsBefore 清理过期错误日志
	ClearErrorsBefore(t time.Time) error
	// Clear 清空所有统计数据
	Clear() error
	// DeleteBySite 清空指定站点在六张统计表中的全部数据
	DeleteBySite(site string) error
	VacuumDB() error

	// 蜘蛛统计
	UpsertSpiders(stats []*WebsiteStatSpider) error
	TopSpiders(start, end string, sites []string, limit uint) ([]*WebsiteStatSpiderRank, error)
	ClearSpidersBefore(date string) error

	// 客户端统计
	UpsertClients(stats []*WebsiteStatClient) error
	TopClients(start, end string, sites []string, limit uint) ([]*WebsiteStatClientRank, error)
	ClearClientsBefore(date string) error

	// IP 统计
	UpsertIPs(stats []*WebsiteStatIP) error
	TopIPs(start, end string, sites []string, page, limit uint) ([]*WebsiteStatIPRank, uint, error)
	TopGeos(start, end string, sites []string, groupBy string, country string, limit uint) ([]*WebsiteStatGeoRank, error)
	ClearIPsBefore(date string) error

	// URI 统计
	UpsertURIs(stats []*WebsiteStatURI) error
	TopURIs(start, end string, sites []string, page, limit uint) ([]*WebsiteStatURIRank, uint, error)
	TopSlowURIs(start, end string, sites []string, threshold, page, limit uint) ([]*WebsiteStatURIRank, uint, error)
	ClearURIsBefore(date string) error

	// 错误日志查询
	ListErrors(start, end string, sites []string, status int, page, limit uint) ([]*WebsiteErrorLog, uint, error)

	// 网站维度汇总
	ListSiteStats(start, end string, sites []string) ([]*WebsiteStatSiteItem, error)
}

type WebsiteStatUsecase struct {
	t    *gotext.Locale
	log  *slog.Logger
	repo WebsiteStatRepo
}

func NewWebsiteStatUsecase(t *gotext.Locale, log *slog.Logger, repo WebsiteStatRepo) *WebsiteStatUsecase {
	return &WebsiteStatUsecase{t: t, log: log, repo: repo}
}

func (uc *WebsiteStatUsecase) Upsert(stats []*WebsiteStat) error {
	return uc.repo.Upsert(stats)
}

func (uc *WebsiteStatUsecase) ListByDateRange(start, end string, sites []string) ([]*WebsiteStat, error) {
	return uc.repo.ListByDateRange(start, end, sites)
}

func (uc *WebsiteStatUsecase) DailySeries(start, end string, sites []string) ([]*WebsiteStatSeries, error) {
	return uc.repo.DailySeries(start, end, sites)
}

func (uc *WebsiteStatUsecase) HourlySeries(date string, sites []string) ([]*WebsiteStatSeries, error) {
	return uc.repo.HourlySeries(date, sites)
}

func (uc *WebsiteStatUsecase) ClearBefore(date string) error {
	return uc.repo.ClearBefore(date)
}

func (uc *WebsiteStatUsecase) InsertErrors(errors []*WebsiteErrorLog) error {
	return uc.repo.InsertErrors(errors)
}

func (uc *WebsiteStatUsecase) ClearErrorsBefore(t time.Time) error {
	return uc.repo.ClearErrorsBefore(t)
}

func (uc *WebsiteStatUsecase) Clear(ctx context.Context) error {
	if err := uc.repo.Clear(); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("website stats cleared"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)))

	return nil
}

func (uc *WebsiteStatUsecase) DeleteBySite(site string) error {
	return uc.repo.DeleteBySite(site)
}

func (uc *WebsiteStatUsecase) VacuumDB() error {
	return uc.repo.VacuumDB()
}

func (uc *WebsiteStatUsecase) UpsertSpiders(stats []*WebsiteStatSpider) error {
	return uc.repo.UpsertSpiders(stats)
}

func (uc *WebsiteStatUsecase) TopSpiders(start, end string, sites []string, limit uint) ([]*WebsiteStatSpiderRank, error) {
	return uc.repo.TopSpiders(start, end, sites, limit)
}

func (uc *WebsiteStatUsecase) ClearSpidersBefore(date string) error {
	return uc.repo.ClearSpidersBefore(date)
}

func (uc *WebsiteStatUsecase) UpsertClients(stats []*WebsiteStatClient) error {
	return uc.repo.UpsertClients(stats)
}

func (uc *WebsiteStatUsecase) TopClients(start, end string, sites []string, limit uint) ([]*WebsiteStatClientRank, error) {
	return uc.repo.TopClients(start, end, sites, limit)
}

func (uc *WebsiteStatUsecase) ClearClientsBefore(date string) error {
	return uc.repo.ClearClientsBefore(date)
}

func (uc *WebsiteStatUsecase) UpsertIPs(stats []*WebsiteStatIP) error {
	return uc.repo.UpsertIPs(stats)
}

func (uc *WebsiteStatUsecase) TopIPs(start, end string, sites []string, page, limit uint) ([]*WebsiteStatIPRank, uint, error) {
	return uc.repo.TopIPs(start, end, sites, page, limit)
}

func (uc *WebsiteStatUsecase) TopGeos(start, end string, sites []string, groupBy string, country string, limit uint) ([]*WebsiteStatGeoRank, error) {
	return uc.repo.TopGeos(start, end, sites, groupBy, country, limit)
}

func (uc *WebsiteStatUsecase) ClearIPsBefore(date string) error {
	return uc.repo.ClearIPsBefore(date)
}

func (uc *WebsiteStatUsecase) UpsertURIs(stats []*WebsiteStatURI) error {
	return uc.repo.UpsertURIs(stats)
}

func (uc *WebsiteStatUsecase) TopURIs(start, end string, sites []string, page, limit uint) ([]*WebsiteStatURIRank, uint, error) {
	return uc.repo.TopURIs(start, end, sites, page, limit)
}

func (uc *WebsiteStatUsecase) TopSlowURIs(start, end string, sites []string, threshold, page, limit uint) ([]*WebsiteStatURIRank, uint, error) {
	return uc.repo.TopSlowURIs(start, end, sites, threshold, page, limit)
}

func (uc *WebsiteStatUsecase) ClearURIsBefore(date string) error {
	return uc.repo.ClearURIsBefore(date)
}

func (uc *WebsiteStatUsecase) ListErrors(start, end string, sites []string, status int, page, limit uint) ([]*WebsiteErrorLog, uint, error) {
	return uc.repo.ListErrors(start, end, sites, status, page, limit)
}

func (uc *WebsiteStatUsecase) ListSiteStats(start, end string, sites []string) ([]*WebsiteStatSiteItem, error) {
	return uc.repo.ListSiteStats(start, end, sites)
}

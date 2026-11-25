package data

import (
	"slices"
	"strings"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/migration"
)

type scanEventRepo struct {
	db *gorm.DB
}

// NewScanEventRepo 创建扫描事件数据访问实例
func NewScanEventRepo() (biz.ScanEventRepo, error) {
	scanDB, err := openSharedDB("scan")
	if err != nil {
		return nil, err
	}

	if err = gormigrate.New(scanDB, nil, migration.ScanMigrations).Migrate(); err != nil {
		return nil, err
	}

	return &scanEventRepo{
		db: scanDB,
	}, nil
}

func (r *scanEventRepo) Upsert(events []*biz.ScanEvent) error {
	if len(events) == 0 {
		return nil
	}

	sourceMap := make(map[string]*biz.ScanSource, len(events))
	for _, event := range events {
		sourceMap[event.SourceIP] = &biz.ScanSource{
			SourceIP: event.SourceIP,
			Country:  event.Country,
			Region:   event.Region,
			City:     event.City,
			ISP:      event.ISP,
		}
	}

	sources := make([]*biz.ScanSource, 0, len(sourceMap))
	for _, source := range sourceMap {
		sources = append(sources, source)
	}
	if err := r.db.Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_ip"}},
			DoUpdates: clause.AssignmentColumns([]string{"country", "region", "city", "isp"}),
		},
		clause.Returning{Columns: []clause.Column{{Name: "id"}, {Name: "source_ip"}}},
	).CreateInBatches(&sources, upsertBatchSize).Error; err != nil {
		return err
	}

	sourceMap = make(map[string]*biz.ScanSource, len(sources))
	for _, source := range sources {
		sourceMap[source.SourceIP] = source
	}
	for _, event := range events {
		event.SourceID = sourceMap[event.SourceIP].ID
	}

	return batchUpsert(r.db, events, clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}, {Name: "source_id"}, {Name: "port"}, {Name: "protocol"}},
		DoUpdates: clause.Assignments(map[string]any{
			"count":     gorm.Expr("count + ?", gorm.Expr("excluded.count")),
			"last_seen": gorm.Expr("excluded.last_seen"),
		}),
	})
}

func (r *scanEventRepo) List(
	start, end, sourceIP string,
	port uint,
	location string,
	page, limit uint,
) ([]*biz.ScanEvent, uint, error) {
	var total int64
	var items []*biz.ScanEvent

	tx := r.db.Table("scan_events").
		Joins("JOIN scan_sources ON scan_sources.id = scan_events.source_id").
		Where("scan_events.date BETWEEN ? AND ?", start, end)
	if sourceIP != "" {
		tx = tx.Where("scan_sources.source_ip LIKE ?", "%"+sourceIP+"%")
	}
	if port > 0 {
		tx = tx.Where("scan_events.port = ?", port)
	}
	if location != "" {
		like := "%" + location + "%"
		tx = tx.Where("scan_sources.country LIKE ? OR scan_sources.region LIKE ? OR scan_sources.city LIKE ? OR scan_sources.isp LIKE ?", like, like, like, like)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := tx.Select("scan_events.*, scan_sources.source_ip, scan_sources.country, scan_sources.region, scan_sources.city, scan_sources.isp").
		Order("scan_events.last_seen DESC").Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, uint(total), nil //nolint:gosec
}

func (r *scanEventRepo) Summary(start, end string) (*biz.ScanSummary, error) {
	var summary biz.ScanSummary
	err := r.db.Model(&biz.ScanEvent{}).
		Where("date BETWEEN ? AND ?", start, end).
		Select("COALESCE(SUM(count), 0) as total_count, COUNT(DISTINCT source_id) as unique_ips, COUNT(DISTINCT port || '-' || protocol) as unique_ports").
		Scan(&summary).Error
	return &summary, err
}

func (r *scanEventRepo) Trend(start, end string) ([]*biz.ScanDayTrend, error) {
	var trends []*biz.ScanDayTrend
	err := r.db.Model(&biz.ScanEvent{}).
		Where("date BETWEEN ? AND ?", start, end).
		Select("date, COALESCE(SUM(count), 0) as total_count, COUNT(DISTINCT source_id) as unique_ips").
		Group("date").
		Order("date ASC").
		Scan(&trends).Error
	return trends, err
}

func (r *scanEventRepo) TopSourceIPs(start, end string, limit uint) ([]*biz.ScanSourceRank, error) {
	var ranks []*biz.ScanSourceRank
	err := r.db.Table("scan_events").
		Joins("JOIN scan_sources ON scan_sources.id = scan_events.source_id").
		Where("scan_events.date BETWEEN ? AND ?", start, end).
		Select("MAX(scan_sources.source_ip) as source_ip, COALESCE(SUM(scan_events.count), 0) as total_count, COUNT(DISTINCT scan_events.port || '-' || scan_events.protocol) as port_count, MAX(scan_events.last_seen) as last_seen, MAX(scan_sources.country) as country, MAX(scan_sources.region) as region, MAX(scan_sources.city) as city, MAX(scan_sources.isp) as isp").
		Group("scan_events.source_id").
		Order("total_count DESC").
		Limit(int(limit)).
		Scan(&ranks).Error
	for _, rank := range ranks {
		rank.LastSeen = r.parseTimeStr(rank.LastSeen)
	}
	return ranks, err
}

// parseTimeStr 解析 Go time.String() 格式并转为 RFC3339
func (r *scanEventRepo) parseTimeStr(s string) string {
	if idx := strings.Index(s, " m="); idx > 0 {
		s = s[:idx]
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", s); err == nil {
		return t.Format(time.RFC3339)
	}
	return s
}

func (r *scanEventRepo) TopPorts(start, end string, limit uint) ([]*biz.ScanPortRank, error) {
	var ranks []*biz.ScanPortRank
	err := r.db.Model(&biz.ScanEvent{}).
		Where("date BETWEEN ? AND ?", start, end).
		Select("port, protocol, COALESCE(SUM(count), 0) as total_count, COUNT(DISTINCT source_id) as ip_count").
		Group("port, protocol").
		Order("total_count DESC").
		Limit(int(limit)).
		Scan(&ranks).Error
	return ranks, err
}

func (r *scanEventRepo) ClearBefore(date string) error {
	return r.db.Where("date < ?", date).Delete(&biz.ScanEvent{}).Error
}

func (r *scanEventRepo) Clear() error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&biz.ScanEvent{}).Error; err != nil {
			return err
		}
		return tx.Where("1 = 1").Delete(&biz.ScanSource{}).Error
	})
}

func (r *scanEventRepo) VacuumDB() error {
	if err := r.db.Exec("DELETE FROM scan_sources WHERE id NOT IN (SELECT source_id FROM scan_events)").Error; err != nil {
		return err
	}
	return vacuumDB(r.db)
}

func (r *scanEventRepo) AddBlocks(blocks []*biz.ScanBlock) error {
	return batchUpsert(r.db, blocks, clause.OnConflict{
		Columns:   []clause.Column{{Name: "ip"}},
		DoUpdates: clause.AssignmentColumns([]string{"expires_at", "outbound", "created_at"}),
	})
}

func (r *scanEventRepo) ListBlocks(ip string, now time.Time, page, limit uint) ([]*biz.ScanBlock, uint, error) {
	var total int64
	items := make([]*biz.ScanBlock, 0)

	tx := r.db.Model(&biz.ScanBlock{}).Where("expires_at = 0 OR expires_at > ?", now.Unix())
	if ip != "" {
		tx = tx.Where("ip LIKE ?", "%"+ip+"%")
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("created_at DESC").Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, uint(total), nil //nolint:gosec
}

func (r *scanEventRepo) ActiveBlocks(now time.Time) ([]*biz.ScanBlock, error) {
	blocks := make([]*biz.ScanBlock, 0)
	err := r.db.Where("expires_at = 0 OR expires_at > ?", now.Unix()).Order("ip ASC").Find(&blocks).Error
	return blocks, err
}

func (r *scanEventRepo) ExpiredBlocks(now time.Time) ([]*biz.ScanBlock, error) {
	blocks := make([]*biz.ScanBlock, 0)
	err := r.db.Where("expires_at > 0 AND expires_at <= ?", now.Unix()).Find(&blocks).Error
	return blocks, err
}

func (r *scanEventRepo) DeleteBlocks(ips []string) error {
	for chunk := range slices.Chunk(ips, upsertBatchSize) {
		if err := r.db.Where("ip IN ?", chunk).Delete(&biz.ScanBlock{}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *scanEventRepo) ClearBlocks() error {
	return r.db.Where("1 = 1").Delete(&biz.ScanBlock{}).Error
}

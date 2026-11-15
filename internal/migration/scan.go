package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

var ScanMigrations = []*gormigrate.Migration{
	{
		ID: "20260218-init-scan-events",
		Migrate: func(tx *gorm.DB) error {
			if tx.Migrator().HasTable("scan_events") {
				return nil
			}
			return tx.AutoMigrate(&biz.ScanSource{}, &biz.ScanEvent{})
		},
	},
	{
		ID: "20260814-normalize-scan-events",
		Migrate: func(tx *gorm.DB) error {
			legacy := tx.Migrator().HasTable("scan_events_legacy")
			if !legacy && !tx.Migrator().HasColumn("scan_events", "source_ip") {
				return nil
			}

			if !legacy {
				if err := tx.Transaction(func(tx *gorm.DB) error {
					if err := tx.Migrator().RenameTable("scan_events", "scan_events_legacy"); err != nil {
						return err
					}
					if err := tx.Exec("DROP INDEX IF EXISTS idx_scan_unique").Error; err != nil {
						return err
					}
					if err := tx.Exec("DROP INDEX IF EXISTS idx_scan_date").Error; err != nil {
						return err
					}
					return tx.AutoMigrate(&biz.ScanSource{}, &biz.ScanEvent{})
				}); err != nil {
					return err
				}
			}

			// 同一 IP 的归属信息取任意一条，扫描来源数量远小于事件数量
			if err := tx.Exec(`
				INSERT INTO scan_sources (source_ip, country, region, city, isp)
				SELECT source_ip, MAX(country), MAX(region), MAX(city), MAX(isp)
				FROM scan_events_legacy
				GROUP BY source_ip
				ON CONFLICT DO NOTHING
			`).Error; err != nil {
				return err
			}

			// 按主键分批提交
			var cursor uint
			if err := tx.Raw("SELECT COALESCE(MAX(id), 0) FROM scan_events").Scan(&cursor).Error; err != nil {
				return err
			}
			for {
				var next uint
				if err := tx.Raw(
					"SELECT COALESCE(MAX(id), 0) FROM (SELECT id FROM scan_events_legacy WHERE id > ? ORDER BY id LIMIT ?)",
					cursor, batchSize,
				).Scan(&next).Error; err != nil {
					return err
				}
				if next == 0 {
					break
				}
				if err := tx.Exec(`
					INSERT INTO scan_events (id, source_id, port, protocol, date, count, first_seen, last_seen)
					SELECT events.id, sources.id, events.port, events.protocol, events.date,
						events.count, events.first_seen, events.last_seen
					FROM scan_events_legacy AS events
					JOIN scan_sources AS sources ON sources.source_ip = events.source_ip
					WHERE events.id > ? AND events.id <= ?
					ON CONFLICT DO NOTHING
				`, cursor, next).Error; err != nil {
					return err
				}
				cursor = next
			}

			if err := tx.Migrator().DropTable("scan_events_legacy"); err != nil {
				return err
			}

			return vacuumDB(tx)
		},
	},
	{
		ID: "20261115-add-scan-blocks",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.ScanBlock{})
		},
	},
}

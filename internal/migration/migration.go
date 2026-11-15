package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const batchSize = 1000

var Migrations []*gormigrate.Migration

func vacuumDB(db *gorm.DB) error {
	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		return err
	}
	if err := db.Exec("VACUUM").Error; err != nil {
		return err
	}
	// 写回 VACUUM 结果并截断文件
	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		return err
	}
	return db.Exec("PRAGMA optimize").Error
}

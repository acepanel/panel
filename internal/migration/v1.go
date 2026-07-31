package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

func init() {
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260101-init",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&biz.App{},
				&biz.Cache{},
				&biz.Cert{},
				&biz.CertAccount{},
				&biz.CertDNS{},
				&biz.Cron{},
				&biz.DatabaseServer{},
				&biz.DatabaseUser{},
				&biz.Monitor{},
				&biz.Setting{},
				&biz.SSH{},
				&biz.Task{},
				&biz.User{},
				&biz.UserToken{},
				&biz.WebHook{},
				&biz.Website{},
			)
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(
				&biz.App{},
				&biz.Cert{},
				&biz.CertAccount{},
				&biz.CertDNS{},
				&biz.Cron{},
				&biz.DatabaseServer{},
				&biz.DatabaseUser{},
				&biz.Monitor{},
				&biz.Setting{},
				&biz.SSH{},
				&biz.Task{},
				&biz.User{},
				&biz.UserToken{},
				&biz.WebHook{},
				&biz.Website{},
			)
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260110-add-project",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.Project{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.Project{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260120-add-backup",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.BackupStorage{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.BackupStorage{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260216-add-cron-config",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.Cron{})
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260218-add-scan-events",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.ScanEvent{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.ScanEvent{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260226-add-user-passkeys",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.UserPasskey{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.UserPasskey{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260228-update-user-passkeys",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.UserPasskey{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.UserPasskey{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260406-add-cert-alias",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.Cert{})
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260416-add-website-expire-at",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.Website{})
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260610-add-waf",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.WafBinding{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.WafBinding{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260720-add-tamper-rules",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.TamperRule{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.TamperRule{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260720-add-file-share",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.FileShare{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.FileShare{})
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260720-update-task-fields",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.Task{})
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260725-add-notify-and-alert",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.NotifyChannel{}, &biz.AlertRule{}, &biz.Alert{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.NotifyChannel{}, &biz.AlertRule{}, &biz.Alert{})
		},
	})
}

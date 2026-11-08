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
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260814-move-monitor",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("monitors") {
				return nil
			}
			if err := tx.Migrator().DropTable("monitors"); err != nil {
				return err
			}
			return vacuumDB(tx)
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260814-remove-main-scan-events",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("scan_events") {
				return nil
			}
			return tx.Migrator().DropTable("scan_events")
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20260815-cert-multi-websites",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&biz.Cert{}, &biz.Website{}); err != nil {
				return err
			}

			// 证书与网站的关系反转到网站表，取最早的一张
			if tx.Migrator().HasColumn(&biz.Cert{}, "website_id") {
				if err := tx.Exec(`UPDATE websites SET cert_id = (
					SELECT MIN(certs.id) FROM certs WHERE certs.website_id = websites.id
				) WHERE EXISTS (SELECT 1 FROM certs WHERE certs.website_id = websites.id)`).Error; err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&biz.Cert{}, "website_id"); err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropColumn(&biz.Website{}, "cert_id")
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20261025-add-website-project",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&biz.Website{}); err != nil {
				return err
			}

			// 之前项目按同名约定创建反代网站，没有关联字段，按名称补上绑定
			return tx.Exec(`UPDATE websites SET project_id = (
				SELECT projects.id FROM projects WHERE projects.name = websites.name
			) WHERE type = 'proxy' AND EXISTS (SELECT 1 FROM projects WHERE projects.name = websites.name)`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropColumn(&biz.Website{}, "project_id")
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20261028-user-hard-delete",
		Migrate: func(tx *gorm.DB) error {
			// 用户和通行密钥改为硬删除，软删除残留的行会占住唯一的用户名
			for _, table := range []string{"users", "user_passkeys"} {
				if !tx.Migrator().HasColumn(table, "deleted_at") {
					continue
				}
				if err := tx.Exec("DELETE FROM `" + table + "` WHERE deleted_at IS NOT NULL").Error; err != nil {
					return err
				}
				if err := tx.Exec("DROP INDEX IF EXISTS `idx_" + table + "_deleted_at`").Error; err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(table, "deleted_at"); err != nil {
					return err
				}
			}

			// 之前删除用户没有真正删掉令牌和通行密钥
			for _, model := range []any{&biz.UserToken{}, &biz.UserPasskey{}} {
				if err := tx.Where("user_id NOT IN (SELECT id FROM users)").Delete(model).Error; err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20261108-add-file-favorite",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&biz.FileFavorite{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&biz.FileFavorite{})
		},
	})

}

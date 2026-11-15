package bootstrap

import (
	"errors"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/libtnb/logrotate"
	"github.com/libtnb/sqlite"
	sloggorm "github.com/orandin/slog-gorm"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/migration"
	"github.com/acepanel/panel/v3/pkg/config"
)

// NewDB 打开面板数据库，返回的 cleanup 在进程退出时关闭连接
func NewDB(conf *config.Config) (*gorm.DB, func() error, error) {
	// db 日志写入轮转文件
	w, err := logrotate.New(filepath.Join(app.Root, "panel/storage/logs/db.log"),
		logrotate.WithMaxSize(10*logrotate.MB),
		logrotate.WithMaxAge(30*logrotate.Day),
		logrotate.WithRotateAt("00:00"),
		logrotate.WithFileMode(0o600),
		logrotate.WithLocation(time.Local),
	)
	if err != nil {
		return nil, nil, err
	}

	handler := slog.New(slog.NewJSONHandler(w, nil)).Handler()
	options := []sloggorm.Option{sloggorm.WithHandler(handler)}
	if conf.Database.Debug {
		options = append(options, sloggorm.WithTraceAll())
	}

	db, err := gorm.Open(
		sqlite.Open("file:"+filepath.Join(app.Root, "panel/storage/panel.db")+
			"?_txlock=immediate&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"),
		&gorm.Config{
			Logger:                                   sloggorm.New(options...),
			SkipDefaultTransaction:                   true,
			DisableForeignKeyConstraintWhenMigrating: true,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	return db, func() error { return errors.Join(sqlDB.Close(), w.Close()) }, nil
}

func NewMigrate(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, nil, migration.Migrations)
}

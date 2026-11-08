package data

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/libtnb/sqlite"
	"github.com/moby/moby/client"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/acepanel/panel/v3/internal/app"
)

// upsert 分批大小
const upsertBatchSize = 100

var (
	sharedDBMu sync.Mutex
	sharedDBs  = make(map[string]*gorm.DB)
)

func getDockerClient(sock string) (*client.Client, error) {
	apiClient, err := client.New(client.WithHost(sock))
	if err != nil {
		return nil, err
	}

	return apiClient, nil
}

// openSharedDB 打开长期持有的辅助数据库
func openSharedDB(name string) (*gorm.DB, error) {
	sharedDBMu.Lock()
	defer sharedDBMu.Unlock()

	if db, ok := sharedDBs[name]; ok {
		return db, nil
	}

	db, err := openDB(name)
	if err != nil {
		return nil, err
	}
	sharedDBs[name] = db

	return db, nil
}

// openDB 打开数据库
func openDB(name string) (*gorm.DB, error) {
	if err := registerZstdSerializer(); err != nil {
		return nil, err
	}

	dsn := "file:" + filepath.Join(app.Root, fmt.Sprintf("panel/storage/%s.db", name)) +
		"?_txlock=immediate&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return db, nil
}

// quickCheck 用 PRAGMA quick_check 检测 SQLite 数据库是否损坏
func quickCheck(db *gorm.DB) bool {
	var result string
	if err := db.Raw("PRAGMA quick_check").Row().Scan(&result); err != nil {
		return false
	}
	return result == "ok"
}

// batchUpsert 通用分批 upsert 辅助函数
func batchUpsert[T any](db *gorm.DB, items []T, conflict clause.OnConflict) error {
	for i := 0; i < len(items); i += upsertBatchSize {
		end := min(i+upsertBatchSize, len(items))
		if err := db.Clauses(conflict).Create(items[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}

// vacuumDB 清理数据后回收文件空间
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

package data

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/migration"
)

func newScanEventRepoForTest(t *testing.T) *scanEventRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	must.NoError(t, err)
	// file::memory: 每连接独立库
	sqlDB, err := db.DB()
	must.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	must.NoError(t, gormigrate.New(db, nil, migration.ScanMigrations).Migrate())
	return &scanEventRepo{db: db}
}

func TestScanBlocks(t *testing.T) {
	repo := newScanEventRepoForTest(t)
	now := time.Now()

	count := func() int {
		var total int64
		must.NoError(t, repo.db.Model(&biz.ScanBlock{}).Count(&total).Error)
		return int(total)
	}

	must.NoError(t, repo.AddBlocks([]*biz.ScanBlock{
		{IP: "1.1.1.1", CreatedAt: now.Add(-time.Hour)},
		{IP: "2.2.2.2", ExpiresAt: now.Add(time.Hour).Unix()},
		{IP: "2001:db8::1"},
	}))
	// 已有记录的按这次的重新计时
	must.NoError(t, repo.AddBlocks([]*biz.ScanBlock{{IP: "1.1.1.1", ExpiresAt: now.Add(-time.Minute).Unix(), Outbound: true}}))
	check.Equal(t, count(), 3)

	expired, err := repo.ExpiredBlocks(now)
	must.NoError(t, err)
	must.Len(t, expired, 1)
	check.Equal(t, expired[0].IP, "1.1.1.1")
	check.True(t, expired[0].Outbound)
	check.True(t, expired[0].CreatedAt.After(now.Add(-time.Minute)))

	// 永久屏蔽不会到期
	expired, err = repo.ExpiredBlocks(now.AddDate(100, 0, 0))
	must.NoError(t, err)
	check.Len(t, expired, 2)

	must.NoError(t, repo.DeleteBlocks([]string{"1.1.1.1", "3.3.3.3"}))
	expired, err = repo.ExpiredBlocks(now)
	must.NoError(t, err)
	check.Len(t, expired, 0)
	check.Equal(t, count(), 2)

	active, err := repo.ActiveBlocks(now)
	must.NoError(t, err)
	check.Len(t, active, 2)

	listed, total, err := repo.ListBlocks("2.2", now, 1, 10)
	must.NoError(t, err)
	check.Equal(t, total, uint(1))
	must.Len(t, listed, 1)
	check.Equal(t, listed[0].IP, "2.2.2.2")

	// 超过单批上限时分批处理
	ips := make([]string, 0, 2*upsertBatchSize+5)
	blocks := make([]*biz.ScanBlock, 0, cap(ips))
	for i := range cap(ips) {
		ips = append(ips, fmt.Sprintf("10.0.%d.%d", i/250, i%250))
		blocks = append(blocks, &biz.ScanBlock{IP: ips[i], ExpiresAt: now.Unix()})
	}
	must.NoError(t, repo.AddBlocks(blocks))
	check.Equal(t, count(), 2+len(ips))
	must.NoError(t, repo.DeleteBlocks(ips))
	check.Equal(t, count(), 2)

	must.NoError(t, repo.ClearBlocks())
	check.Equal(t, count(), 0)
}

package data

import (
	"fmt"
	"slices"
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

	must.NoError(t, repo.AddBlocks([]*biz.ScanBlock{
		{IP: "1.1.1.1", ExpiresAt: now.Add(-time.Minute).Unix(), Outbound: true},
		{IP: "2.2.2.2", ExpiresAt: now.Add(time.Hour).Unix()},
		{IP: "2001:db8::1"},
	}))
	// 已有记录的保持原到期时间
	must.NoError(t, repo.AddBlocks([]*biz.ScanBlock{{IP: "1.1.1.1", ExpiresAt: now.Add(time.Hour).Unix()}}))

	blocked, err := repo.BlockedIPs([]string{"1.1.1.1", "3.3.3.3", "2001:db8::1"})
	must.NoError(t, err)
	slices.Sort(blocked)
	check.DeepEqual(t, blocked, []string{"1.1.1.1", "2001:db8::1"})

	expired, err := repo.ExpiredBlocks(now)
	must.NoError(t, err)
	must.Len(t, expired, 1)
	check.Equal(t, expired[0].IP, "1.1.1.1")
	check.True(t, expired[0].Outbound)
	check.False(t, expired[0].CreatedAt.IsZero())

	// 永久屏蔽不会到期
	expired, err = repo.ExpiredBlocks(now.AddDate(100, 0, 0))
	must.NoError(t, err)
	check.Len(t, expired, 2)

	must.NoError(t, repo.DeleteBlocks([]string{"1.1.1.1", "3.3.3.3"}))
	expired, err = repo.ExpiredBlocks(now)
	must.NoError(t, err)
	check.Len(t, expired, 0)
	blocked, err = repo.BlockedIPs([]string{"1.1.1.1", "2.2.2.2"})
	must.NoError(t, err)
	check.DeepEqual(t, blocked, []string{"2.2.2.2"})

	// 超过单批上限时分批处理
	ips := make([]string, 0, 2*upsertBatchSize+5)
	blocks := make([]*biz.ScanBlock, 0, cap(ips))
	for i := range cap(ips) {
		ips = append(ips, fmt.Sprintf("10.0.%d.%d", i/250, i%250))
		blocks = append(blocks, &biz.ScanBlock{IP: ips[i], ExpiresAt: now.Unix()})
	}
	must.NoError(t, repo.AddBlocks(blocks))
	blocked, err = repo.BlockedIPs(ips)
	must.NoError(t, err)
	check.Len(t, blocked, len(ips))
	must.NoError(t, repo.DeleteBlocks(ips))
	blocked, err = repo.BlockedIPs(ips)
	must.NoError(t, err)
	check.Len(t, blocked, 0)
}

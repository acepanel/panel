package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/migration"
)

func newWebsiteStatRepoForTest(t *testing.T) *websiteStatRepo {
	t.Helper()
	must.NoError(t, registerZstdSerializer())
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	must.NoError(t, err)
	// file::memory: 每连接独立库
	sqlDB, err := db.DB()
	must.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	must.NoError(t, gormigrate.New(db, nil, migration.WebsiteStatMigrations).Migrate())
	return &websiteStatRepo{db: db}
}

// statRow 第 i 个计数列的值为 base+i，每列取值不同才能发现列错位
func statRow(site, date string, hour int, base uint64) *biz.WebsiteStat {
	return &biz.WebsiteStat{
		Site: site, Date: date, Hour: hour,
		PV: base + 1, UV: base + 2, IP: base + 3, Bandwidth: base + 4, BandwidthIn: base + 5,
		Requests: base + 6, Errors: base + 7, Spiders: base + 8, RequestTimeSum: base + 9, RequestTimeCount: base + 10,
		Status2xx: base + 11, Status3xx: base + 12, Status4xx: base + 13, Status5xx: base + 14,
	}
}

// statJSON 第 i 个计数键的值为 base+step*i
func statJSON(head string, base, step uint64) string {
	keys := []string{"pv", "uv", "ip", "bandwidth", "bandwidth_in", "requests", "errors", "spiders", "request_time_sum", "request_time_count", "status_2xx", "status_3xx", "status_4xx", "status_5xx"}
	parts := []string{head}
	for i, key := range keys {
		parts = append(parts, fmt.Sprintf("%q:%d", key, base+step*uint64(i+1)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func mustJSON(t *testing.T, got any, want ...string) {
	t.Helper()
	raw, err := json.Marshal(got)
	must.NoError(t, err)
	must.Equal(t, string(raw), "["+strings.Join(want, ",")+"]")
}

func TestWebsiteStatAggregate(t *testing.T) {
	repo := newWebsiteStatRepoForTest(t)

	must.NoError(t, repo.Upsert([]*biz.WebsiteStat{
		statRow("a", "2026-02-18", -1, 100),
		statRow("a", "2026-02-18", 3, 10),
		statRow("a", "2026-02-18", 5, 20),
		statRow("a", "2026-02-19", -1, 200),
		statRow("b", "2026-02-18", -1, 1000),
		statRow("b", "2026-02-18", 3, 30),
	}))
	// 同键再写一次走累加
	must.NoError(t, repo.Upsert([]*biz.WebsiteStat{statRow("a", "2026-02-18", 3, 10)}))

	sites, err := repo.ListSiteStats("2026-02-18", "2026-02-19", nil)
	must.NoError(t, err)
	mustJSON(t, sites, statJSON(`"site":"b"`, 1000, 1), statJSON(`"site":"a"`, 300, 2))

	sites, err = repo.ListSiteStats("2026-02-19", "2026-02-19", []string{"a", "b"})
	must.NoError(t, err)
	mustJSON(t, sites, statJSON(`"site":"a"`, 200, 1))

	daily, err := repo.DailySeries("2026-02-18", "2026-02-19", nil)
	must.NoError(t, err)
	mustJSON(t, daily, statJSON(`"key":"2026-02-18"`, 1100, 2), statJSON(`"key":"2026-02-19"`, 200, 1))

	daily, err = repo.DailySeries("2026-02-18", "2026-02-19", []string{"b"})
	must.NoError(t, err)
	mustJSON(t, daily, statJSON(`"key":"2026-02-18"`, 1000, 1))

	hourly, err := repo.HourlySeries("2026-02-18", nil)
	must.NoError(t, err)
	mustJSON(t, hourly, statJSON(`"key":"3"`, 50, 3), statJSON(`"key":"5"`, 20, 1))

	hourly, err = repo.HourlySeries("2026-02-18", []string{"a"})
	must.NoError(t, err)
	mustJSON(t, hourly, statJSON(`"key":"3"`, 20, 2), statJSON(`"key":"5"`, 20, 1))

	hourly, err = repo.HourlySeries("2026-02-20", nil)
	must.NoError(t, err)
	must.Len(t, hourly, 0)
}

func TestWebsiteStatDelete(t *testing.T) {
	repo := newWebsiteStatRepoForTest(t)
	models := []any{&biz.WebsiteStat{}, &biz.WebsiteErrorLog{}, &biz.WebsiteStatSpider{}, &biz.WebsiteStatClient{}, &biz.WebsiteStatIP{}, &biz.WebsiteStatURI{}}
	counts := func() []int64 {
		result := make([]int64, len(models))
		for i, model := range models {
			must.NoError(t, repo.db.Model(model).Count(&result[i]).Error)
		}
		return result
	}

	// 超过一批，分批写入不能丢
	errors := make([]*biz.WebsiteErrorLog, 0, 2*upsertBatchSize+50)
	for i := range cap(errors) {
		errors = append(errors, &biz.WebsiteErrorLog{Site: []string{"a", "b"}[i%2], URI: "/", Method: "GET", Status: 500})
	}
	must.NoError(t, repo.InsertErrors(errors))
	must.NoError(t, repo.InsertErrors(nil))
	for _, site := range []string{"a", "b"} {
		must.NoError(t, repo.Upsert([]*biz.WebsiteStat{statRow(site, "2026-02-18", -1, 0)}))
		must.NoError(t, repo.UpsertSpiders([]*biz.WebsiteStatSpider{{Site: site, Date: "2026-02-18", Spider: "Googlebot", Requests: 1}}))
		must.NoError(t, repo.UpsertClients([]*biz.WebsiteStatClient{{Site: site, Date: "2026-02-18", Browser: "Chrome", OS: "Linux", Requests: 1}}))
		must.NoError(t, repo.UpsertIPs([]*biz.WebsiteStatIP{{Site: site, Date: "2026-02-18", IP: "1.2.3.4", Requests: 1}}))
		must.NoError(t, repo.UpsertURIs([]*biz.WebsiteStatURI{{Site: site, Date: "2026-02-18", URI: "/", Requests: 1}}))
	}
	must.DeepEqual(t, counts(), []int64{2, 250, 2, 2, 2, 2})

	must.NoError(t, repo.DeleteBySite("a"))
	must.DeepEqual(t, counts(), []int64{1, 125, 1, 1, 1, 1})

	must.NoError(t, repo.Clear())
	must.DeepEqual(t, counts(), []int64{0, 0, 0, 0, 0, 0})
}

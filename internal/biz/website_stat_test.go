package biz_test

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/pkg/websitestat"
)

const countersJSON = `"pv":1,"uv":2,"ip":3,"bandwidth":4,"bandwidth_in":5,"requests":6,"errors":7,"spiders":8,"request_time_sum":9,"request_time_count":10,"status_2xx":11,"status_3xx":12,"status_4xx":13,"status_5xx":14`

// 键名与顺序是前端契约：解码不许有未知键，再编码必须逐字节还原
func mustKeepJSON[T any](t *testing.T, want string) {
	t.Helper()
	var v T
	dec := json.NewDecoder(strings.NewReader(want))
	dec.DisallowUnknownFields()
	must.NoError(t, dec.Decode(&v))
	got, err := json.Marshal(v)
	must.NoError(t, err)
	must.Equal(t, string(got), want)
}

func TestWebsiteStatJSON(t *testing.T) {
	mustKeepJSON[biz.WebsiteStatSeries](t, `{"key":"2026-02-18",`+countersJSON+"}")
	mustKeepJSON[biz.WebsiteStatSiteItem](t, `{"site":"demo",`+countersJSON+"}")
	mustKeepJSON[biz.WebsiteStatSpiderRank](t, `{"spider":"Googlebot","requests":1,"percent":2.5}`)
	mustKeepJSON[biz.WebsiteStatClientRank](t, `{"browser":"Chrome 120","os":"Windows","requests":1}`)
	mustKeepJSON[biz.WebsiteStatIPRank](t, `{"ip":"1.2.3.4","country":"CN","region":"GD","city":"SZ","isp":"CT","requests":1,"bandwidth":2}`)
	mustKeepJSON[biz.WebsiteStatGeoRank](t, `{"country":"CN","region":"GD","city":"SZ","requests":1,"bandwidth":2}`)
	mustKeepJSON[biz.WebsiteStatURIRank](t, `{"uri":"/a","requests":1,"bandwidth":2,"errors":3,"request_time_sum":4,"request_time_count":5}`)
}

func newWebsiteStatUsecaseForTest(repo biz.WebsiteStatRepo, aggregator *websitestat.Aggregator) *biz.WebsiteStatUsecase {
	return biz.NewWebsiteStatUsecase(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), repo, aggregator)
}

// newAggregatorForTest 内存里留着站点 a、b 各一条未落库的今日请求
func newAggregatorForTest() *websitestat.Aggregator {
	aggregator := websitestat.NewAggregator()
	aggregator.Record(&websitestat.LogEntry{Site: "a", URI: "/", Status: 200, Bytes: 100, IP: "1.1.1.1"})
	aggregator.Record(&websitestat.LogEntry{Site: "b", URI: "/", Status: 500, Bytes: 7, IP: "2.2.2.2"})
	return aggregator
}

func dayForTest(offset int) string {
	return time.Now().AddDate(0, 0, offset).Format(time.DateOnly)
}

func TestWebsiteStatOverviewHourly(t *testing.T) {
	aggregator := newAggregatorForTest()
	live := aggregator.SiteStats()
	stored := map[int]uint64{3: 5, 20: 6}
	repo := &mocks.WebsiteStatRepo{
		ListSiteStatsFunc: func(string, string, []string) ([]*biz.WebsiteStatSiteItem, error) {
			return []*biz.WebsiteStatSiteItem{
				{Site: "a", Requests: 10, Bandwidth: 1000},
				{Site: "b", Requests: 20, Bandwidth: 2000},
			}, nil
		},
		HourlySeriesFunc: func(string, []string) ([]*biz.WebsiteStatSeries, error) {
			return []*biz.WebsiteStatSeries{
				{Key: "3", Requests: stored[3]},
				{Key: "20", Requests: stored[20]},
			}, nil
		},
	}

	overview, err := newWebsiteStatUsecaseForTest(repo, aggregator).Overview(dayForTest(0), dayForTest(0), []string{"a"})
	must.NoError(t, err)

	// 只叠加所选站点的内存增量，上一周期不含今日
	must.DeepEqual(t, overview.Current, websitestat.Counters{PV: 1, UV: 1, IP: 1, Bandwidth: 3100, Requests: 31, Status2xx: 1})
	must.DeepEqual(t, overview.Previous, websitestat.Counters{Bandwidth: 3000, Requests: 30})
	must.Equal(t, repo.ListSiteStatsCalls()[1].Start, dayForTest(-1))
	must.Equal(t, repo.ListSiteStatsCalls()[1].End, dayForTest(-1))
	must.Equal(t, repo.HourlySeriesCalls()[1].Date, dayForTest(-1))

	must.Len(t, overview.Series, 24)
	must.Len(t, overview.PreviousSeries, 24)
	var total uint64
	for h, point := range overview.Series {
		want := stored[h]
		if hour := live["a"].Hours[h]; hour != nil {
			want += hour.Requests
		}
		must.Equal(t, point.Key, strconv.Itoa(h))
		must.Equal(t, point.Requests, want)
		must.Equal(t, overview.PreviousSeries[h].Key, strconv.Itoa(h))
		must.Equal(t, overview.PreviousSeries[h].Requests, stored[h])
		total += point.Requests
	}
	must.Equal(t, total, uint64(12))
}

func TestWebsiteStatOverviewDaily(t *testing.T) {
	repo := &mocks.WebsiteStatRepo{
		ListSiteStatsFunc: func(string, string, []string) ([]*biz.WebsiteStatSiteItem, error) {
			return nil, nil
		},
		DailySeriesFunc: func(start, _ string, _ []string) ([]*biz.WebsiteStatSeries, error) {
			return []*biz.WebsiteStatSeries{{Key: start, Requests: 7}}, nil
		},
	}

	overview, err := newWebsiteStatUsecaseForTest(repo, newAggregatorForTest()).Overview(dayForTest(-2), dayForTest(0), nil)
	must.NoError(t, err)

	must.DeepEqual(t, overview.Current, websitestat.Counters{PV: 2, UV: 2, IP: 2, Bandwidth: 107, Requests: 2, Errors: 1, Status2xx: 1, Status5xx: 1})
	must.DeepEqual(t, overview.Previous, websitestat.Counters{})
	must.Equal(t, repo.DailySeriesCalls()[1].Start, dayForTest(-5))
	must.Equal(t, repo.DailySeriesCalls()[1].End, dayForTest(-3))

	// 缺数据的日期补零，内存增量只记在今日
	must.DeepEqual(t, overview.Series, []*biz.WebsiteStatSeries{
		{Key: dayForTest(-2), Requests: 7},
		{Key: dayForTest(-1)},
		{Key: dayForTest(0), Counters: overview.Current},
	})
	must.DeepEqual(t, overview.PreviousSeries, []*biz.WebsiteStatSeries{
		{Key: dayForTest(-5), Requests: 7},
		{Key: dayForTest(-4)},
		{Key: dayForTest(-3)},
	})
}

func TestWebsiteStatListSiteStats(t *testing.T) {
	var rows []*biz.WebsiteStatSiteItem
	repo := &mocks.WebsiteStatRepo{
		ListSiteStatsFunc: func(string, string, []string) ([]*biz.WebsiteStatSiteItem, error) {
			return rows, nil
		},
	}
	sites := func(uc *biz.WebsiteStatUsecase, start, end int, sites []string) string {
		items, err := uc.ListSiteStats(dayForTest(start), dayForTest(end), sites)
		must.NoError(t, err)
		raw, err := json.Marshal(items)
		must.NoError(t, err)
		return string(raw)
	}

	// 没有任何数据时也是空数组
	must.Equal(t, sites(newWebsiteStatUsecaseForTest(repo, websitestat.NewAggregator()), 0, 0, nil), "[]")

	aggregator := newAggregatorForTest()
	aggregator.Record(&websitestat.LogEntry{Site: "b", URI: "/x", Status: 200, Bytes: 1, IP: "2.2.2.2"})
	aggregator.Record(&websitestat.LogEntry{Site: "c", URI: "/", Status: 200, Bytes: 1, IP: "3.3.3.3"})
	uc := newWebsiteStatUsecaseForTest(repo, aggregator)
	names := func(start, end int, filter []string) []string {
		rows = []*biz.WebsiteStatSiteItem{
			{Site: "a", Requests: 2},
			{Site: "b", Requests: 1},
		}
		items, err := uc.ListSiteStats(dayForTest(start), dayForTest(end), filter)
		must.NoError(t, err)
		result := make([]string, len(items))
		for i, item := range items {
			result[i] = item.Site + "=" + strconv.FormatUint(item.Requests, 10)
		}
		return result
	}

	// 叠加内存增量后重新按请求数排序，只在内存里有数据的站点也要列出
	must.DeepEqual(t, names(-1, 0, nil), []string{"a=3", "b=3", "c=1"})
	must.DeepEqual(t, names(-1, 0, []string{"b", "c"}), []string{"b=3", "a=2", "c=1"})
	must.DeepEqual(t, names(-3, -1, nil), []string{"a=2", "b=1"})
}

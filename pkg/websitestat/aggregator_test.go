package websitestat

import (
	"testing"

	"github.com/libtnb/assert/must"
)

func TestAggregatorDedup(t *testing.T) {
	a := NewAggregator()
	for _, entry := range []*LogEntry{
		{Site: "s", IP: "1.1.1.1", UA: "a"},
		{Site: "s", IP: "1.1.1.1", UA: "a"},
		{Site: "s", IP: "1.1.1.1", UA: "b"},
		{Site: "s", IP: "2.2.2.2", UA: "a"},
		// IP 与 UA 的边界参与去重，拼起来相同也算不同访客
		{Site: "s", IP: "2.2.2.2a", UA: ""},
	} {
		a.Record(entry)
	}

	snap := a.SiteStats()["s"]
	must.Equal(t, snap.UV, uint64(4))
	must.Equal(t, snap.IP, uint64(3))
}

func TestAggregatorDedupMaxKeys(t *testing.T) {
	a := NewAggregator()
	a.UVMaxKeys, a.IPMaxKeys = 2, 1
	for _, ip := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		a.Record(&LogEntry{Site: "s", IP: ip})
	}

	snap := a.SiteStats()["s"]
	must.Equal(t, snap.UV, uint64(2))
	must.Equal(t, snap.IP, uint64(1))
}

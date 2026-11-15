package websitestat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/libtnb/assert/must"
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

func TestSnapshotJSON(t *testing.T) {
	mustKeepJSON[Counters](t, "{"+countersJSON+"}")
	mustKeepJSON[HourSnapshot](t, "{"+countersJSON+"}")
	mustKeepJSON[SiteSnapshot](t, "{"+countersJSON+"}")
	mustKeepJSON[RealtimeStats](t, `{"bandwidth":1.5,"bandwidth_in":2.5,"rps":3.5}`)
}

func TestCountersAdd(t *testing.T) {
	var c Counters
	must.NoError(t, json.Unmarshal([]byte("{"+countersJSON+"}"), &c))
	c.Add(c)
	got, err := json.Marshal(c)
	must.NoError(t, err)
	must.Equal(t, string(got), `{"pv":2,"uv":4,"ip":6,"bandwidth":8,"bandwidth_in":10,"requests":12,"errors":14,"spiders":16,"request_time_sum":18,"request_time_count":20,"status_2xx":22,"status_3xx":24,"status_4xx":26,"status_5xx":28}`)
}

package db

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

// mongosh 把 64 位整数输出成 {high, low} 两个有符号 32 位数，low 的最高位为 1 时是负数
func TestMongoDBLongToInt64(t *testing.T) {
	var r *MongoDB

	check.Equal(t, r.longToInt64(float64(8192)), 8192)
	check.Equal(t, r.longToInt64(map[string]any{"high": float64(0), "low": float64(8192)}), 8192)
	// 3GB：low 是 0xC0000000，按有符号读是 -1073741824
	check.Equal(t, r.longToInt64(map[string]any{"high": float64(0), "low": float64(-1073741824)}), 3<<30)
	// 5GB：high 为 1
	check.Equal(t, r.longToInt64(map[string]any{"high": float64(1), "low": float64(1073741824)}), 5<<30)
	check.Equal(t, r.longToInt64("x"), 0)
}

func TestMongoDBScript(t *testing.T) {
	var r *MongoDB

	script, err := r.script(`db.getSiblingDB('admin').changeUserPassword(%s,%s)`, "app", `a'b"c\d$e`)
	must.NoError(t, err)
	check.Equal(t, script, `db.getSiblingDB('admin').changeUserPassword("app","a'b\"c\\d$e")`)

	script, err = r.script(`db.runCommand({ping:1})`)
	must.NoError(t, err)
	check.Equal(t, script, `db.runCommand({ping:1})`)
}

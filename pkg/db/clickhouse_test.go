package db

import (
	"testing"

	"github.com/libtnb/assert/check"
)

// 经 Operator 接口调用 QueryRow 后直接 Scan 不能空指针
func TestClickHouseQueryRowScanReturnsError(t *testing.T) {
	var operator Operator = &ClickHouse{}

	var version string
	err := operator.QueryRow(t.Context(), "SELECT version()").Scan(&version)
	check.ErrorContains(t, err, "does not support sql.Row")
}

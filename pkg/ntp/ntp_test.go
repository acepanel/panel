package ntp

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/utils/env"
)

func TestNowWithDefaultAddresses(t *testing.T) {
	now, err := Now()
	if err != nil {
		t.Logf("内置 NTP 服务器不可达，回落本地时间: %v", err)
	}
	check.DeepEqual(t, now, time.Now(), cmpopts.EquateApproxTime(time.Minute))
}

func TestNowWithCustomAddress(t *testing.T) {
	now, err := Now("time.windows.com")
	check.NoError(t, err)
	check.DeepEqual(t, now, time.Now(), cmpopts.EquateApproxTime(time.Minute))
}

func TestNowWithInvalidAddress(t *testing.T) {
	now, err := Now("invalid.address")
	check.ErrorIs(t, err, errNotReachable)
	// 失败时回落到本地时间，不是零值
	check.DeepEqual(t, now, time.Now(), cmpopts.EquateApproxTime(time.Minute))
}

func TestUpdateSystemTime(t *testing.T) {
	if env.IsWindows() {
		t.Skip("Skipping on Windows")
	}
	err := UpdateSystemTime(t.Context(), time.Now())
	check.NoError(t, err)
}

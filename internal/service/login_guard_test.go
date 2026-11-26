package service

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestLoginGuardCount(t *testing.T) {
	guard := newLoginGuard()
	check.Equal(t, guard.Count("203.0.113.7"), uint(0))

	guard.Fail("203.0.113.7")
	guard.Fail("203.0.113.7")
	check.Equal(t, guard.Count("203.0.113.7"), uint(2))
	check.Equal(t, guard.Count("203.0.113.8"), uint(0))

	guard.Reset("203.0.113.7")
	check.Equal(t, guard.Count("203.0.113.7"), uint(0))
}

func TestLoginGuardMergesIPv6Prefix(t *testing.T) {
	guard := newLoginGuard()

	guard.Fail("2001:db8:1:2::1")
	guard.Fail("2001:db8:1:2::2")
	check.Equal(t, guard.Count("2001:db8:1:2:ffff::3"), uint(2))
	check.Equal(t, guard.Count("2001:db8:1:3::1"), uint(0))
}

package os

import (
	"testing"
	"time"

	"github.com/libtnb/assert/check"
)

func TestIsDebian(t *testing.T) {
	check.True(t, IsDebian(), check.Msgf("os-release: %v", readOSRelease()))
}

func TestIsRHEL(t *testing.T) {
	check.False(t, IsRHEL(), check.Msgf("os-release: %v", readOSRelease()))
}

func TestIsEOL(t *testing.T) {
	ubuntu := map[string]string{"ID": "ubuntu", "ID_LIKE": "debian", "VERSION_ID": "22.04"}
	check.False(t, isEOL(ubuntu, time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)))
	check.True(t, isEOL(ubuntu, time.Date(2027, 5, 1, 0, 0, 0, 0, time.UTC)))

	rocky := map[string]string{"ID": "rocky", "ID_LIKE": "rhel centos fedora", "VERSION_ID": "9.6"}
	check.False(t, isEOL(rocky, time.Date(2032, 5, 1, 0, 0, 0, 0, time.UTC)))
	check.True(t, isEOL(rocky, time.Date(2032, 7, 1, 0, 0, 0, 0, time.UTC)))

	// 衍生版的 VERSION_ID 不含点，不能按下标取前两段
	mint := map[string]string{"ID": "linuxmint", "ID_LIKE": "ubuntu debian", "VERSION_ID": "22"}
	check.False(t, isEOL(mint, time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)))
	check.False(t, isEOL(nil, time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)))
}

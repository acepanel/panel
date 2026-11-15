package tamper

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestExcludeMatches(t *testing.T) {
	tests := []struct {
		ex, path string
		want     bool
	}{
		{"", "/www/site/cache", false},
		{"/www/site/cache", "/www/site/cache", true},
		{"/www/site/cache/", "/www/site/cache/a.php", true},
		{"/www/site/cache", "/www/site/cache2/a.php", false},
		// 相对名匹配任意位置上的路径段
		{"cache", "/www/site/cache/a.php", true},
		{"cache", "/www/site/cache", true},
		{"cache", "/www/site/mycache/a.php", false},
		{"cache", "/www/site/cache.php", false},
		// 多段的相对路径要连续命中
		{"cache/tmp", "/www/site/cache/tmp/a.php", true},
		{"cache/tmp/", "/www/site/cache/tmp", true},
		{"cache/tmp", "/www/site/cache/x/tmp/a.php", false},
		{"cache/tmp", "/www/site/mycache/tmp/a.php", false},
	}
	for _, tt := range tests {
		check.Equal(t, ExcludeMatches(tt.ex, tt.path), tt.want, check.Msgf("ex=%q path=%q", tt.ex, tt.path))
	}
}

package db

import (
	"testing"

	"github.com/lib/pq"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

// 用户名、密码、库名里的反斜杠和单引号要原样到达驱动
func TestPostgresDSNEscape(t *testing.T) {
	cases := []struct{ username, password, dbname string }{
		{`postgres`, `plain`, `postgres`},
		{`o'brien`, `it's`, `my'db`},
		{`dom\user`, `tail\`, `a\b`},
		{`a\'b`, `\\'\'`, `'\`},
	}
	for _, c := range cases {
		cfg, err := pq.NewConfig(postgresDSN(c.username, c.password, "127.0.0.1", 5432, c.dbname))
		must.NoError(t, err)
		check.Equal(t, cfg.User, c.username)
		check.Equal(t, cfg.Password, c.password)
		check.Equal(t, cfg.Database, c.dbname)
		check.Equal(t, cfg.Host, "127.0.0.1")
	}
}

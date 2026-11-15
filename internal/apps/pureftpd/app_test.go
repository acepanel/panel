package pureftpd

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestPort(t *testing.T) {
	s := new(App)
	// 面板改端口时写入的是带引号的形式
	check.Equal(t, s.Port("Daemonize yes\nBind \"0.0.0.0,2121\"\n"), 2121)
	check.Equal(t, s.Port("Bind 127.0.0.1,2100\n"), 2100)
	// 未配置、被注释或只写了地址时为默认端口
	check.Equal(t, s.Port("Daemonize yes\n"), 21)
	check.Equal(t, s.Port("# Bind 127.0.0.1,2100\n"), 21)
	check.Equal(t, s.Port("Bind 127.0.0.1\n"), 21)
}

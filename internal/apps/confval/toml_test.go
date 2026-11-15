package confval

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestGetDottedKey(t *testing.T) {
	conf := "bindPort = 7000\nauth.token = \"12345678\"\n# maxPortsPerClient = 0\n"

	check.Equal(t, getTOML(conf, "bindPort"), "7000")
	check.Equal(t, getTOML(conf, "auth.token"), "12345678")
	// 注释掉的项视为未设置
	check.Empty(t, getTOML(conf, "maxPortsPerClient"))
	check.Empty(t, getTOML(conf, "absent"))
}

func TestGetSectionKey(t *testing.T) {
	conf := "bindPort = 7000\n\n[auth]\nmethod = \"token\"\ntoken = \"abc\"\n"

	check.Equal(t, getTOML(conf, "auth.method"), "token")
	check.Equal(t, getTOML(conf, "auth.token"), "abc")
}

func TestGetIgnoresProxyBlock(t *testing.T) {
	conf := "serverAddr = \"1.1.1.1\"\n\n[[proxies]]\nname = \"ssh\"\nserverAddr = \"2.2.2.2\"\n"

	check.Equal(t, getTOML(conf, "serverAddr"), "1.1.1.1")
}

func TestSetReplacesInPlace(t *testing.T) {
	conf := "# 监听端口\nbindPort = 7000 # 默认\nauth.token = \"old\"\n"

	check.Equal(t, setTOML(conf, "bindPort", 7001), "# 监听端口\nbindPort = 7001 # 默认\nauth.token = \"old\"\n")
	check.Equal(t, setTOML(conf, "auth.token", "new"), "# 监听端口\nbindPort = 7000 # 默认\nauth.token = \"new\"\n")
}

func TestSetReplacesInSection(t *testing.T) {
	conf := "bindPort = 7000\n\n[auth]\ntoken = \"old\"\n"

	check.Equal(t, setTOML(conf, "auth.token", "new"), "bindPort = 7000\n\n[auth]\ntoken = \"new\"\n")
}

func TestSetInsertsBeforeFirstTable(t *testing.T) {
	conf := "bindPort = 7000\n\n[[proxies]]\nname = \"ssh\"\n"

	check.Equal(t, setTOML(conf, "auth.token", "abc"), "bindPort = 7000\n\nauth.token = \"abc\"\n[[proxies]]\nname = \"ssh\"\n")
}

func TestSetAppendsWhenNoTable(t *testing.T) {
	check.Equal(t, setTOML("bindPort = 7000", "auth.token", "abc"), "bindPort = 7000\nauth.token = \"abc\"")
}

func TestSetCommentsOutEmptyValue(t *testing.T) {
	check.Equal(t, setTOML("bindPort = 7000", "bindPort", ""), "# bindPort = 7000")
	check.Equal(t, setTOML("# bindPort = 7000", "bindPort", ""), "# bindPort = 7000")
	// 键不存在时不应插入空值
	check.Equal(t, setTOML("bindPort = 7000", "auth.token", ""), "bindPort = 7000")
}

func TestSetReenablesCommented(t *testing.T) {
	check.Equal(t, setTOML("# maxPortsPerClient = 0", "maxPortsPerClient", 5), "maxPortsPerClient = 5")
}

func TestSetBool(t *testing.T) {
	check.Equal(t, setTOML("bindPort = 7000", "transport.tls.force", true), "bindPort = 7000\ntransport.tls.force = true")
}

func TestSetDoesNotTouchProxyBlock(t *testing.T) {
	conf := "serverAddr = \"1.1.1.1\"\n\n[[proxies]]\n# 手写的隧道\nname = \"ssh\"\nserverPort = 22\n"

	check.Equal(t, setTOML(conf, "serverAddr", "2.2.2.2"), "serverAddr = \"2.2.2.2\"\n\n[[proxies]]\n# 手写的隧道\nname = \"ssh\"\nserverPort = 22\n")
	// serverPort 只存在于 proxies 表内，不应被改写，而是插到首个表头前
	check.Equal(t, setTOML(conf, "serverPort", 7000), "serverAddr = \"1.1.1.1\"\n\nserverPort = 7000\n[[proxies]]\n# 手写的隧道\nname = \"ssh\"\nserverPort = 22\n")
}

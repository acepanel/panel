package confval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestFieldsReadAndWrite(t *testing.T) {
	conf := "port 6379\n# maxmemory 100mb\n"

	var port, maxmemory, timeout string
	fields := []Field{
		{Key: "port", Value: &port},
		{Key: "maxmemory", Value: &maxmemory},
		{Key: "timeout", Value: &timeout},
	}
	Directive.Read(conf, fields)
	check.Equal(t, port, "6379")
	check.Empty(t, maxmemory)
	check.Empty(t, timeout)

	// 原样写回不改动内容
	check.Equal(t, Directive.Write(conf, fields), conf)

	port, maxmemory, timeout = "", "200mb", "30"
	check.Equal(t, Directive.Write(conf, fields), "# port 6379\nmaxmemory 200mb\n\ntimeout 30")
}

func TestFieldsHonorSection(t *testing.T) {
	conf := "[client]\nport = 3306\n\n[mysqld]\nport = 3306\n\n[mysqldump]\nmax_allowed_packet = 1G\n"

	var port, packet string
	fields := []Field{
		{Section: "mysqld", Key: "port", Value: &port},
		{Section: "mysqld", Key: "max_allowed_packet", Value: &packet},
	}
	SectionINI.Read(conf, fields)
	check.Equal(t, port, "3306")
	// 其它分组的同名项不算
	check.Empty(t, packet)

	port, packet = "3307", "256M"
	check.Equal(t, SectionINI.Write(conf, fields), "[client]\nport = 3306\n\n[mysqld]\nport = 3307\nmax_allowed_packet = 256M\n\n[mysqldump]\nmax_allowed_packet = 1G\n")
}

func TestTOMLFieldsKeepLiteralKind(t *testing.T) {
	conf := "bindPort = 7000\nauth.token = \"old\"\n"

	var port, token, mux, user string
	fields := []Field{
		{Key: "bindPort", Value: &port, Kind: Int},
		{Key: "auth.token", Value: &token},
		{Key: "transport.tcpMux", Value: &mux, Kind: Bool},
		{Key: "user", Value: &user},
	}
	ReadTOML(conf, fields)
	check.Equal(t, port, "7000")
	check.Equal(t, token, "old")
	check.Empty(t, mux)

	port, token, mux = "7001", "new", "true"
	// 整数与布尔不加引号，空值的新项不写入
	check.Equal(t, WriteTOML(conf, fields), "bindPort = 7001\nauth.token = \"new\"\n\ntransport.tcpMux = true")
}

func TestLoadYAMLAlwaysReturnsMap(t *testing.T) {
	dir := t.TempDir()

	// 文件不存在、内容为空、只有注释、显式 null，更新路径随后都要往 map 里写
	cfg, err := LoadYAML(filepath.Join(dir, "absent.yml"))
	must.NoError(t, err)
	must.NotNil(t, cfg)

	for name, content := range map[string]string{"empty.yml": "", "comment.yml": "# only comments\n", "null.yml": "~\n"} {
		path := filepath.Join(dir, name)
		must.NoError(t, os.WriteFile(path, []byte(content), 0600))

		cfg, err = LoadYAML(path)
		must.NoError(t, err)
		must.NotNil(t, cfg)
		cfg["key"] = "value"
	}

	path := filepath.Join(dir, "config.yml")
	must.NoError(t, os.WriteFile(path, []byte("http.port: 9200\nnet:\n  port: 27017\n"), 0600))
	cfg, err = LoadYAML(path)
	must.NoError(t, err)
	check.Equal(t, GetYAML(cfg, "http.port"), "9200")
	check.Equal(t, GetYAML(cfg, "net.port"), "27017")

	// 格式错误时报错而不是当成空配置，否则保存会把原文件整个覆盖掉
	must.NoError(t, os.WriteFile(path, []byte("net:\n  port: [\n"), 0600))
	cfg, err = LoadYAML(path)
	check.Error(t, err)
	check.NotNil(t, cfg)
}

func TestSetYAMLNestedCreatesParents(t *testing.T) {
	cfg := map[string]any{"net": map[string]any{"bindIp": "127.0.0.1"}}

	SetYAMLNested(cfg, "net.port", "27018")
	SetYAMLNested(cfg, "storage.wiredTiger.engineConfig.cacheSizeGB", "2")
	SetYAMLNested(cfg, "path", "/data")
	SetYAMLNested(cfg, "security.authorization", "")

	check.DeepEqual(t, cfg, map[string]any{
		"net":     map[string]any{"bindIp": "127.0.0.1", "port": "27018"},
		"storage": map[string]any{"wiredTiger": map[string]any{"engineConfig": map[string]any{"cacheSizeGB": "2"}}},
		"path":    "/data",
	})
	check.Equal(t, GetYAML(cfg, "storage.wiredTiger.engineConfig.cacheSizeGB"), "2")
}

func TestSetYAMLNestedClearsFlatKey(t *testing.T) {
	cfg := map[string]any{"logger.level": "trace"}
	SetYAMLNested(cfg, "logger.level", "warning")

	// 平铺键不清掉的话 GetYAML 读到的还是旧值
	check.DeepEqual(t, cfg, map[string]any{"logger": map[string]any{"level": "warning"}})
	check.Equal(t, GetYAML(cfg, "logger.level"), "warning")
}

func TestJVMHeap(t *testing.T) {
	// jvm.options 里注释掉的示例行不算
	initSize, maxSize := JVMHeap("## -Xms4g\n## -Xmx4g\n-Xms1g\n-Xmx2g\n")
	check.Equal(t, initSize, "1g")
	check.Equal(t, maxSize, "2g")

	// 环境变量取值形式，两项在同一行，后面可以带别的参数
	initSize, maxSize = JVMHeap("-Xms512m -Xmx1g -Dtls.server.mode=disabled")
	check.Equal(t, initSize, "512m")
	check.Equal(t, maxSize, "1g")

	initSize, maxSize = JVMHeap("## -Xms4g\n")
	check.Empty(t, initSize)
	check.Empty(t, maxSize)
}

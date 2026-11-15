package confval

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestDirectiveGet(t *testing.T) {
	conf := "# comment\nbind 127.0.0.1 ::1\nport 6379\n\n# maxmemory 100mb\n"

	check.Equal(t, Directive.Get(conf, "bind"), "127.0.0.1 ::1")
	check.Equal(t, Directive.Get(conf, "port"), "6379")
	// 注释掉的项不应被读出
	check.Empty(t, Directive.Get(conf, "maxmemory"))
	check.Empty(t, Directive.Get(conf, "absent"))
}

func TestDirectiveSetReplaces(t *testing.T) {
	check.Equal(t, Directive.Set("port 6379", "port", "6380"), "port 6380")
}

func TestDirectiveSetAppendsWhenMissing(t *testing.T) {
	check.Equal(t, Directive.Set("port 6379", "timeout", "30"), "port 6379\ntimeout 30")
}

func TestDirectiveSetCommentsOutEmptyValue(t *testing.T) {
	check.Equal(t, Directive.Set("port 6379", "port", ""), "# port 6379")
	// 已注释的保持原样，不重复加注释符
	check.Equal(t, Directive.Set("# port 6379", "port", ""), "# port 6379")
}

func TestDirectiveSetReenablesCommented(t *testing.T) {
	check.Equal(t, Directive.Set("# maxmemory 100mb", "maxmemory", "200mb"), "maxmemory 200mb")
}

func TestDirectiveSetOnlyKeepsFirstMatch(t *testing.T) {
	check.Equal(t, Directive.Set("port 6379\nport 7000", "port", "6380"), "port 6380")
}

func TestSetStripsNewlinesFromValue(t *testing.T) {
	check.Equal(t, Directive.Set("port 1", "port", "63\nopq\r79"), "port 63opq79")
}

func TestFTPRequiresValueToMatch(t *testing.T) {
	// 裸键行不算配置项，避免把开关误判为键值对
	check.Empty(t, FTP.Get("NoAnonymous\n", "NoAnonymous"))
	check.Equal(t, FTP.Get("NoAnonymous yes\n", "NoAnonymous"), "yes")
}

func TestPropertiesRoundTrip(t *testing.T) {
	conf := "# broker\nnum.network.threads=3\n"

	check.Equal(t, Properties.Get(conf, "num.network.threads"), "3")
	check.Equal(t, Properties.Set(conf, "num.network.threads", "5"), "# broker\nnum.network.threads=5\n")
}

func TestNginxTerminatorAndIndent(t *testing.T) {
	conf := "http {\n    keepalive_timeout  60;\n}"

	check.Equal(t, Nginx.Get(conf, "keepalive_timeout"), "60")
	// 缩进保留，行尾分号补回
	check.Equal(t, Nginx.Set(conf, "keepalive_timeout", "75"), "http {\n    keepalive_timeout 75;\n}")
}

func TestNginxIgnoresLinesWithoutTerminator(t *testing.T) {
	check.Empty(t, Nginx.Get("http {\n    keepalive_timeout 60\n}", "keepalive_timeout"))
}

func TestNginxKeepsTrailingComment(t *testing.T) {
	conf := "http {\n    keepalive_timeout  60; # 保持连接\n}"

	// 注释在分号之后，读取时不能混进值里
	check.Equal(t, Nginx.Get(conf, "keepalive_timeout"), "60")
	check.Equal(t, Nginx.Set(conf, "keepalive_timeout", "75"), "http {\n    keepalive_timeout 75; # 保持连接\n}")
}

func TestCommentOutKeepsWholeLine(t *testing.T) {
	// 注释掉时整行保留，行尾注释不丢
	check.Equal(t, Postgres.Set("max_connections = 100 # note", "max_connections", ""), "# max_connections = 100 # note")
}

func TestNginxAppendsWithIndent(t *testing.T) {
	check.Equal(t, Nginx.Set("a b;", "c", "d"), "a b;\n    c d;")
}

func TestPostgresQuotesAndInlineComment(t *testing.T) {
	conf := "max_connections = 100 # note\nlisten_addresses = '*'\n"

	check.Equal(t, Postgres.Get(conf, "max_connections"), "100")
	check.Equal(t, Postgres.Get(conf, "listen_addresses"), "*")
	// 只替换值，行尾注释保留
	check.Equal(t, Postgres.Set(conf, "max_connections", "200"), "max_connections = '200' # note\nlisten_addresses = '*'\n")
}

func TestInlineCommentNotAppliedWhereUnsupported(t *testing.T) {
	// redis.conf 不支持行尾注释，密码里的 # 是数据不能当注释切掉
	check.Equal(t, Directive.Get("requirepass p@ss#word\n", "requirepass"), "p@ss#word")
	check.Equal(t, Directive.Set("requirepass p@ss#word", "requirepass", "a#b"), "requirepass a#b")
	// java properties 同理
	check.Equal(t, Properties.Get("pass=a#b\n", "pass"), "a#b")
}

func TestQuotedCommentCharIsData(t *testing.T) {
	check.Equal(t, Postgres.Get("password = 'a#b' # real note\n", "password"), "a#b")
	check.Equal(t, Postgres.Set("password = 'a#b' # real note", "password", "x#y"), "password = 'x#y' # real note")
}

func TestFlatINIIgnoresSectionHeaders(t *testing.T) {
	conf := "[mysqld]\nport = 3306\n; skip-name-resolve = 1\n"

	check.Equal(t, PHPINI.Get(conf, "port"), "3306")
	check.Empty(t, PHPINI.Get(conf, "mysqld"))
	// 分号注释同样能被重新启用
	check.Equal(t, PHPINI.Set(conf, "skip-name-resolve", "2"), "[mysqld]\nport = 3306\nskip-name-resolve = 2\n")
}

func TestSectionINIMatchesOnlyTargetSection(t *testing.T) {
	conf := "[server]\nhttp_port = 3000\n\n[database]\nhttp_port = 9999\n"

	check.Equal(t, SectionINI.GetIn(conf, "server", "http_port"), "3000")
	check.Equal(t, SectionINI.GetIn(conf, "database", "http_port"), "9999")
	check.Empty(t, SectionINI.GetIn(conf, "absent", "http_port"))

	got := SectionINI.SetIn(conf, "database", "http_port", "8888")
	check.Equal(t, SectionINI.GetIn(got, "server", "http_port"), "3000")
	check.Equal(t, SectionINI.GetIn(got, "database", "http_port"), "8888")
}

func TestSectionINIInsertsInsideSection(t *testing.T) {
	conf := "[server]\nhttp_port = 3000\n\n[database]\ntype = sqlite3\n"

	got := SectionINI.SetIn(conf, "server", "domain", "example.com")
	check.Equal(t, SectionINI.GetIn(got, "server", "domain"), "example.com")
	// 新项必须落在 server 段内，不能漏进 database 段
	check.Empty(t, SectionINI.GetIn(got, "database", "domain"))
}

func TestSectionINICreatesMissingSection(t *testing.T) {
	got := SectionINI.SetIn("[server]\nhttp_port = 3000\n", "smtp", "host", "localhost:25")

	check.Equal(t, SectionINI.GetIn(got, "smtp", "host"), "localhost:25")
	check.Equal(t, SectionINI.GetIn(got, "server", "http_port"), "3000")
}

func TestPHPINICommentsWithSemicolon(t *testing.T) {
	check.Equal(t, PHPINI.Set("memory_limit = 128M", "memory_limit", ""), "; memory_limit = 128M")
}

func TestGetYAMLFlatAndNested(t *testing.T) {
	check.Equal(t, GetYAML(map[string]any{"discovery.type": "single-node"}, "discovery.type"), "single-node")
	check.Equal(t, GetYAML(map[string]any{"discovery": map[string]any{"type": "single-node"}}, "discovery.type"), "single-node")
	check.Empty(t, GetYAML(map[string]any{}, "discovery.type"))
}

func TestSetYAMLFlattensAndClearsNested(t *testing.T) {
	cfg := map[string]any{"discovery": map[string]any{"type": "old"}}
	SetYAML(cfg, "discovery.type", "single-node")

	// 扁平键写入，嵌套键清空后整个父键一并移除
	check.DeepEqual(t, cfg, map[string]any{"discovery.type": "single-node"})
}

func TestSetYAMLIgnoresEmptyValue(t *testing.T) {
	cfg := map[string]any{}
	SetYAML(cfg, "discovery.type", "")

	check.Empty(t, cfg)
}

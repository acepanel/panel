package mysql

import (
	"strings"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/pkg/types"
)

// 安装脚本生成的 my.cnf 样式：[client] 与 [mysqld] 都有 port，[mysqld] 与 [mysqldump] 都有 max_allowed_packet
const installedConf = `[client]
port = 3306
socket = /tmp/mysql.sock

[mysqld]
port = 3306
socket = /tmp/mysql.sock
default_storage_engine = InnoDB
key_buffer_size = 8M
max_allowed_packet = 256M
max_connections = 100
slow_query_log = 1
long_query_time = 3

innodb_buffer_pool_size = 128M
innodb_flush_log_at_trx_commit = 1

[mysqldump]
quick
max_allowed_packet = 1G
`

func TestTuneReadsMysqldSection(t *testing.T) {
	conf := strings.Replace(installedConf, "[mysqld]\nport = 3306", "[mysqld]\nport = 3307", 1)

	s := new(App)
	tune := new(ConfigTune)
	confval.SectionINI.Read(conf, s.tuneFields(tune))

	check.Equal(t, tune.Port, "3307")
	check.Equal(t, tune.MaxAllowedPacket, "256M")
	check.Equal(t, tune.InnodbBufferPoolSize, "128M")
	// 模板里没有的项读出空串
	check.Empty(t, tune.ThreadStack)
}

func TestTuneRoundTripKeepsConfigIntact(t *testing.T) {
	s := new(App)
	tune := new(ConfigTune)
	confval.SectionINI.Read(installedConf, s.tuneFields(tune))

	check.Equal(t, confval.SectionINI.Write(installedConf, s.tuneFields(tune)), installedConf)
}

func TestTuneWritesOnlyMysqldSection(t *testing.T) {
	s := new(App)
	tune := new(ConfigTune)
	confval.SectionINI.Read(installedConf, s.tuneFields(tune))
	tune.Port = "3307"
	tune.MaxAllowedPacket = "512M"
	tune.ThreadStack = "512K"

	got := confval.SectionINI.Write(installedConf, s.tuneFields(tune))

	check.Equal(t, confval.SectionINI.GetIn(got, "mysqld", "port"), "3307")
	check.Equal(t, confval.SectionINI.GetIn(got, "mysqld", "max_allowed_packet"), "512M")
	check.Equal(t, confval.SectionINI.GetIn(got, "mysqld", "thread_stack"), "512K")
	// 其它段的同名项原样保留，新增项不能漏进文件末尾的 [mysqldump] 段
	check.Equal(t, confval.SectionINI.GetIn(got, "client", "port"), "3306")
	check.Equal(t, confval.SectionINI.GetIn(got, "mysqldump", "max_allowed_packet"), "1G")
	check.Empty(t, confval.SectionINI.GetIn(got, "mysqldump", "thread_stack"))
	check.Equal(t, strings.Count(got, "port = "), 2)
	check.Equal(t, strings.Count(got, "max_allowed_packet = "), 2)
}

const extendedStatus = `+-----------------------------------+----------+
| Variable_name                     | Value    |
+-----------------------------------+----------+
| Bytes_received                    | 2048     |
| Bytes_sent                        | 1048576  |
| Innodb_buffer_pool_read_requests  | 1000     |
| Innodb_buffer_pool_reads          | 50       |
| Key_read_requests                 | 200      |
| Key_reads                         | 20       |
| Open_tables                       | 64       |
| Uptime                            | 3600     |
+-----------------------------------+----------+`

func loadMap(items []types.NV) map[string]string {
	m := make(map[string]string, len(items))
	for _, item := range items {
		m[item.Name] = item.Value
	}
	return m
}

func TestLoadItemsComputesHitRatesByName(t *testing.T) {
	s := &App{t: gotext.NewLocale("", "en")}

	status := s.parseStatus(extendedStatus)
	must.Equal(t, status["Innodb_buffer_pool_reads"], "50")

	load := loadMap(s.loadItems(status))
	check.Equal(t, load["Uptime"], "3600")
	check.Equal(t, load["Bytes Sent"], "1.00 MB")
	check.Equal(t, load["Index Hit Rate"], "90.00%")
	check.Equal(t, load["Innodb Index Hit Rate"], "95.00%")
	check.Equal(t, load["Open Tables"], "64")
	// 实例没有查询缓存时不展示相关项
	_, ok := load["Query Cache Hit Rate"]
	check.False(t, ok)
}

func TestLoadItemsToleratesMissingVariables(t *testing.T) {
	s := &App{t: gotext.NewLocale("", "en")}

	load := loadMap(s.loadItems(s.parseStatus("")))
	check.Equal(t, load["Index Hit Rate"], "100.00%")
	check.Equal(t, load["Innodb Index Hit Rate"], "100.00%")
	check.Equal(t, len(load), 2)
}

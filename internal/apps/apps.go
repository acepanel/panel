package apps

import (
	"github.com/google/wire"

	"github.com/acepanel/panel/v3/internal/apps/acewaf"
	"github.com/acepanel/panel/v3/internal/apps/apache"
	"github.com/acepanel/panel/v3/internal/apps/clickhouse"
	"github.com/acepanel/panel/v3/internal/apps/codeserver"
	"github.com/acepanel/panel/v3/internal/apps/docker"
	"github.com/acepanel/panel/v3/internal/apps/elasticsearch"
	"github.com/acepanel/panel/v3/internal/apps/fail2ban"
	"github.com/acepanel/panel/v3/internal/apps/frp"
	"github.com/acepanel/panel/v3/internal/apps/gitea"
	"github.com/acepanel/panel/v3/internal/apps/grafana"
	"github.com/acepanel/panel/v3/internal/apps/kafka"
	"github.com/acepanel/panel/v3/internal/apps/mariadb"
	"github.com/acepanel/panel/v3/internal/apps/memcached"
	"github.com/acepanel/panel/v3/internal/apps/minio"
	"github.com/acepanel/panel/v3/internal/apps/mongodb"
	"github.com/acepanel/panel/v3/internal/apps/mysql"
	"github.com/acepanel/panel/v3/internal/apps/nginx"
	"github.com/acepanel/panel/v3/internal/apps/openresty"
	"github.com/acepanel/panel/v3/internal/apps/opensearch"
	"github.com/acepanel/panel/v3/internal/apps/percona"
	"github.com/acepanel/panel/v3/internal/apps/pgadmin"
	"github.com/acepanel/panel/v3/internal/apps/phpmyadmin"
	"github.com/acepanel/panel/v3/internal/apps/podman"
	"github.com/acepanel/panel/v3/internal/apps/postgresql"
	"github.com/acepanel/panel/v3/internal/apps/prometheus"
	"github.com/acepanel/panel/v3/internal/apps/pureftpd"
	"github.com/acepanel/panel/v3/internal/apps/redis"
	"github.com/acepanel/panel/v3/internal/apps/rocketmq"
	"github.com/acepanel/panel/v3/internal/apps/rsync"
	"github.com/acepanel/panel/v3/internal/apps/s3fs"
	"github.com/acepanel/panel/v3/internal/apps/supervisor"
	"github.com/acepanel/panel/v3/internal/apps/valkey"
)

var ProviderSet = wire.NewSet(
	apache.NewApp, clickhouse.NewApp, codeserver.NewApp,
	docker.NewApp, elasticsearch.NewApp, fail2ban.NewApp,
	frp.NewApp, gitea.NewApp, grafana.NewApp,
	kafka.NewApp, mariadb.NewApp, memcached.NewApp,
	minio.NewApp, mongodb.NewApp, mysql.NewApp,
	nginx.NewApp, openresty.NewApp, opensearch.NewApp,
	percona.NewApp, pgadmin.NewApp, phpmyadmin.NewApp, podman.NewApp,
	postgresql.NewApp, prometheus.NewApp, pureftpd.NewApp,
	redis.NewApp, rocketmq.NewApp, rsync.NewApp,
	s3fs.NewApp, supervisor.NewApp, valkey.NewApp,
	acewaf.NewApp,
)

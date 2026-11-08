package bootstrap

import (
	"github.com/acepanel/panel/v3/internal/apps/apache"
	"github.com/acepanel/panel/v3/internal/apps/caddy"
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
	"github.com/acepanel/panel/v3/internal/apps/openlitespeed"
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
	"github.com/acepanel/panel/v3/internal/apps/stalwart"
	"github.com/acepanel/panel/v3/internal/apps/supervisor"
	"github.com/acepanel/panel/v3/internal/apps/valkey"
	"github.com/acepanel/panel/v3/pkg/apploader"
)

func NewLoader(apacheApp *apache.App, caddyApp *caddy.App, clickhouseApp *clickhouse.App, codeserverApp *codeserver.App, dockerApp *docker.App, elasticsearchApp *elasticsearch.App, fail2banApp *fail2ban.App, frpApp *frp.App, giteaApp *gitea.App, grafanaApp *grafana.App, kafkaApp *kafka.App, mariadbApp *mariadb.App, memcachedApp *memcached.App, minioApp *minio.App, mongodbApp *mongodb.App, mysqlApp *mysql.App, nginxApp *nginx.App, openlitespeedApp *openlitespeed.App, openrestyApp *openresty.App, opensearchApp *opensearch.App, perconaApp *percona.App, pgadminApp *pgadmin.App, phpmyadminApp *phpmyadmin.App, podmanApp *podman.App, postgresqlApp *postgresql.App, prometheusApp *prometheus.App, pureftpdApp *pureftpd.App, redisApp *redis.App, rocketmqApp *rocketmq.App, rsyncApp *rsync.App, s3fsApp *s3fs.App, stalwartApp *stalwart.App, supervisorApp *supervisor.App, valkeyApp *valkey.App) *apploader.Loader {
	loader := new(apploader.Loader)
	loader.Add(apacheApp, caddyApp, clickhouseApp, codeserverApp, dockerApp, elasticsearchApp, fail2banApp, frpApp, giteaApp, grafanaApp, kafkaApp, mariadbApp, memcachedApp, minioApp, mongodbApp, mysqlApp, nginxApp, openlitespeedApp, openrestyApp, opensearchApp, perconaApp, pgadminApp, phpmyadminApp, podmanApp, postgresqlApp, prometheusApp, pureftpdApp, redisApp, rocketmqApp, rsyncApp, s3fsApp, stalwartApp, supervisorApp, valkeyApp)
	return loader
}

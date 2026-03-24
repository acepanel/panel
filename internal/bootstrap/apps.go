package bootstrap

import (
	"github.com/acepanel/panel/v3/internal/apps/apache"
	"github.com/acepanel/panel/v3/internal/apps/codeserver"
	"github.com/acepanel/panel/v3/internal/apps/docker"
	"github.com/acepanel/panel/v3/internal/apps/fail2ban"
	"github.com/acepanel/panel/v3/internal/apps/frp"
	"github.com/acepanel/panel/v3/internal/apps/gitea"
	"github.com/acepanel/panel/v3/internal/apps/grafana"
	"github.com/acepanel/panel/v3/internal/apps/mariadb"
	"github.com/acepanel/panel/v3/internal/apps/memcached"
	"github.com/acepanel/panel/v3/internal/apps/minio"
	"github.com/acepanel/panel/v3/internal/apps/mysql"
	"github.com/acepanel/panel/v3/internal/apps/nginx"
	"github.com/acepanel/panel/v3/internal/apps/openresty"
	"github.com/acepanel/panel/v3/internal/apps/percona"
	"github.com/acepanel/panel/v3/internal/apps/phpmyadmin"
	"github.com/acepanel/panel/v3/internal/apps/podman"
	"github.com/acepanel/panel/v3/internal/apps/postgresql"
	"github.com/acepanel/panel/v3/internal/apps/prometheus"
	"github.com/acepanel/panel/v3/internal/apps/pureftpd"
	"github.com/acepanel/panel/v3/internal/apps/redis"
	"github.com/acepanel/panel/v3/internal/apps/rsync"
	"github.com/acepanel/panel/v3/internal/apps/s3fs"
	"github.com/acepanel/panel/v3/internal/apps/supervisor"
	"github.com/acepanel/panel/v3/pkg/apploader"
)

func NewLoader(
	apache *apache.App,
	codeserver *codeserver.App,
	docker *docker.App,
	fail2ban *fail2ban.App,
	frp *frp.App,
	gitea *gitea.App,
	grafana *grafana.App,
	mariadb *mariadb.App,
	memcached *memcached.App,
	minio *minio.App,
	mysql *mysql.App,
	nginx *nginx.App,
	openresty *openresty.App,
	percona *percona.App,
	phpmyadmin *phpmyadmin.App,
	podman *podman.App,
	postgresql *postgresql.App,
	prometheus *prometheus.App,
	pureftpd *pureftpd.App,
	redis *redis.App,
	rsync *rsync.App,
	s3fs *s3fs.App,
	supervisor *supervisor.App,
) *apploader.Loader {
	loader := new(apploader.Loader)
	loader.Add(apache, codeserver, docker, fail2ban, frp, gitea, grafana, mariadb, memcached, minio, mysql, nginx, openresty, percona, phpmyadmin, podman, postgresql, prometheus, pureftpd, redis, rsync, s3fs, supervisor)
	return loader
}

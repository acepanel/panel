package bootstrap

import (
	"github.com/tnb-labs/panel/internal/apps/benchmark"
	"github.com/tnb-labs/panel/internal/apps/codeserver"
	"github.com/tnb-labs/panel/internal/apps/docker"
	"github.com/tnb-labs/panel/internal/apps/fail2ban"
	"github.com/tnb-labs/panel/internal/apps/frp"
	"github.com/tnb-labs/panel/internal/apps/gitea"
	"github.com/tnb-labs/panel/internal/apps/memcached"
	"github.com/tnb-labs/panel/internal/apps/minio"
	"github.com/tnb-labs/panel/internal/apps/mysql"
	"github.com/tnb-labs/panel/internal/apps/nginx"
	"github.com/tnb-labs/panel/internal/apps/php74"
	"github.com/tnb-labs/panel/internal/apps/php80"
	"github.com/tnb-labs/panel/internal/apps/php81"
	"github.com/tnb-labs/panel/internal/apps/php82"
	"github.com/tnb-labs/panel/internal/apps/php83"
	"github.com/tnb-labs/panel/internal/apps/php84"
	"github.com/tnb-labs/panel/internal/apps/phpmyadmin"
	"github.com/tnb-labs/panel/internal/apps/podman"
	"github.com/tnb-labs/panel/internal/apps/postgresql"
	"github.com/tnb-labs/panel/internal/apps/pureftpd"
	"github.com/tnb-labs/panel/internal/apps/redis"
	"github.com/tnb-labs/panel/internal/apps/rsync"
	"github.com/tnb-labs/panel/internal/apps/s3fs"
	"github.com/tnb-labs/panel/internal/apps/supervisor"
	"github.com/tnb-labs/panel/internal/apps/toolbox"
	"github.com/tnb-labs/panel/pkg/apploader"
)

func NewLoader(
	benchmark *benchmark.App,
	codeserver *codeserver.App,
	docker *docker.App,
	fail2ban *fail2ban.App,
	frp *frp.App,
	gitea *gitea.App,
	memcached *memcached.App,
	minio *minio.App,
	mysql *mysql.App,
	nginx *nginx.App,
	php74 *php74.App,
	php80 *php80.App,
	php81 *php81.App,
	php82 *php82.App,
	php83 *php83.App,
	php84 *php84.App,
	phpmyadmin *phpmyadmin.App,
	podman *podman.App,
	postgresql *postgresql.App,
	pureftpd *pureftpd.App,
	redis *redis.App,
	rsync *rsync.App,
	s3fs *s3fs.App,
	supervisor *supervisor.App,
	toolbox *toolbox.App,
) *apploader.Loader {
	loader := new(apploader.Loader)
	loader.Add(benchmark, codeserver, docker, fail2ban, frp, gitea, memcached, minio, mysql, nginx, php74, php80, php81, php82, php83, php84, phpmyadmin, podman, postgresql, pureftpd, redis, rsync, s3fs, supervisor, toolbox)
	return loader
}

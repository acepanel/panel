package apps

import (
	"github.com/google/wire"

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
)

var ProviderSet = wire.NewSet(
	benchmark.NewApp,
	codeserver.NewApp,
	docker.NewApp,
	fail2ban.NewApp,
	frp.NewApp,
	gitea.NewApp,
	memcached.NewApp,
	minio.NewApp,
	mysql.NewApp,
	nginx.NewApp,
	php74.NewApp,
	php80.NewApp,
	php81.NewApp,
	php82.NewApp,
	php83.NewApp,
	php84.NewApp,
	phpmyadmin.NewApp,
	podman.NewApp,
	postgresql.NewApp,
	pureftpd.NewApp,
	redis.NewApp,
	rsync.NewApp,
	s3fs.NewApp,
	supervisor.NewApp,
	toolbox.NewApp,
)

module github.com/acepanel/panel

go 1.25

require (
	github.com/DeRuina/timberjack v1.3.9
	github.com/bddjr/hlfhr v1.4.0
	github.com/beevik/ntp v1.5.0
	github.com/coder/websocket v1.8.14
	github.com/creack/pty v1.1.24
	github.com/dchest/captcha v1.1.0
	github.com/expr-lang/expr v1.17.7
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-chi/httplog/v3 v3.3.0
	github.com/go-gormigrate/gormigrate/v2 v2.1.5
	github.com/go-resty/resty/v2 v2.17.1
	github.com/go-sql-driver/mysql v1.9.3
	github.com/gomodule/redigo v1.9.3
	github.com/google/wire v0.7.0
	github.com/gookit/color v1.6.0
	github.com/gookit/validate v1.5.6
	github.com/hashicorp/go-version v1.8.0
	github.com/klauspost/compress v1.18.2
	github.com/leonelquinteros/gotext v1.7.2
	github.com/lib/pq v1.10.9
	github.com/libdns/alidns v1.0.6-beta.3
	github.com/libdns/cloudflare v0.2.2
	github.com/libdns/cloudns v1.1.0
	github.com/libdns/gcore v0.0.0-20250427050847-9964da923833
	github.com/libdns/huaweicloud v1.0.0
	github.com/libdns/libdns v1.1.1
	github.com/libdns/namesilo v1.0.0
	github.com/libdns/porkbun v1.1.0
	github.com/libdns/tencentcloud v1.4.3
	github.com/libdns/westcn v1.0.2
	github.com/libtnb/chix v1.3.2
	github.com/libtnb/gormstore v1.1.1
	github.com/libtnb/sessions v1.2.2
	github.com/libtnb/utils v1.2.1
	github.com/mholt/acmez/v3 v3.1.4
	github.com/moby/moby/api v1.53.0-rc.1
	github.com/moby/moby/client v0.2.1
	github.com/ncruces/go-sqlite3 v0.30.4
	github.com/ncruces/go-sqlite3/gormlite v0.30.2
	github.com/orandin/slog-gorm v1.4.0
	github.com/pquerna/otp v1.5.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/samber/lo v1.52.0
	github.com/sethvargo/go-limiter v1.1.0
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/spf13/cast v1.10.0
	github.com/stretchr/testify v1.11.1
	github.com/tufanbarisyildirim/gonginx v0.0.0-20250620092546-c3e307e36701
	github.com/urfave/cli/v3 v3.6.1
	go.yaml.in/yaml/v4 v4.0.0-rc.3
	golang.org/x/crypto v0.46.0
	golang.org/x/net v0.48.0
	gorm.io/gorm v1.31.1
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/G-Core/gcore-dns-sdk-go v0.3.3 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/boombuler/barcode v1.1.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gofiber/schema v1.6.0 // indirect
	github.com/gookit/filter v1.2.3 // indirect
	github.com/gookit/goutil v0.7.3 // indirect
	github.com/imega/luaformatter v0.0.0-20211025140405-86b0a68d6bef // indirect
	github.com/jaevor/go-nanoid v1.4.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/libtnb/securecookie v1.2.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	github.com/timtadh/data-structures v0.6.2 // indirect
	github.com/timtadh/lexmachine v0.2.3 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/exp v0.0.0-20251219203646-944ab1f22d93 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

replace (
	github.com/mholt/acmez/v3 => github.com/libtnb/acmez/v3 v3.0.0-20260103184942-a835890fc93e
	github.com/moby/moby/client => github.com/libtnb/moby/client v0.0.0-20260103192150-39cfd5376055
	github.com/stretchr/testify => github.com/libtnb/testify v0.0.0-20260103194301-c7a63ea79696
)

tool github.com/google/wire

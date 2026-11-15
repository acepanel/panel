package caddy

const ServerRoot = "/opt/ace/server/caddy"

const MainConf = ServerRoot + "/Caddyfile"

const htmlDir = ServerRoot + "/html"

// acmeDir HTTP-01 验证目录，站点与兜底站点以它为根提供 token
const acmeDir = ServerRoot + "/acme"

// ErrorLogPath 全局错误日志，Caddy 没有站点级错误日志
const ErrorLogPath = ServerRoot + "/logs/error.log"

// panelACMEConf 面板验证写入的 token 文件名，供清理
const panelACMEConf = ServerRoot + "/conf/acme-tokens"

const confName = "caddy.conf"

const acmeURI = "/.well-known/acme-challenge/"

const phpCacheConf = `# browser cache
@ace_static path *.bmp *.jpg *.jpeg *.png *.gif *.svg *.ico *.tiff *.webp *.avif *.heif *.heic *.jxl
header @ace_static Cache-Control max-age=2592000
log_skip @ace_static
@ace_assets path *.js *.css *.ttf *.otf *.woff *.woff2 *.eot
header @ace_assets Cache-Control max-age=21600
log_skip @ace_assets
# deny sensitive files
@ace_sensitive path_regexp ^/(\.user\.ini|\.htaccess|\.git|\.svn|\.env)
error @ace_sensitive 404
`

const spaConf = `# single-page application route fallback, remove if not needed
try_files {path} {path}/ /index.html
`

const errorPageConf = `handle_errors 404 {
	rewrite * /404.html
	file_server
}
`

// statConf 访问统计日志片段，%s 为站点名
const statConf = `log ace_stat {
	output net unixgram//tmp/ace_stats.sock {
		soft_start
	}
	format append {
		fields {
			site %s
		}
	}
}
`

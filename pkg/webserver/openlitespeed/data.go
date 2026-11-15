package openlitespeed

const ServerRoot = "/opt/ace/server/openlitespeed"

const htmlDir = ServerRoot + "/html"

// panelConfDir 面板托管的主配置片段目录，由主配置 include
const panelConfDir = ServerRoot + "/conf/panel"

const listenersConf = panelConfDir + "/listeners.conf"

const vhostsConf = panelConfDir + "/vhosts.conf"

// vhostStubDir configFile 桩目录，让 OLS 的 .txt 转储留在面板目录内
const vhostStubDir = panelConfDir + "/vhosts"

const sitesPath = "/opt/ace/sites"

// 站点配置目录下的文件名
const (
	vhostConfName  = "openlitespeed.conf"        // 站点 vhconf，由面板生成
	listenConfName = "openlitespeed.listen.conf" // 面板记录的监听地址与域名
)

// stopURI 站点停用时请求重写到的路径
const stopURI = "/ace-stop/"

// accessLogFormat OLS 不剥离值两端的引号，外层加引号会原样写进每行日志
const accessLogFormat = `%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"`

const phpCacheConf = `expires {
  enableExpires           1
  expiresByType           image/*=A2592000,text/css=A21600,application/javascript=A21600,application/x-javascript=A21600,font/*=A21600,application/font-woff=A21600,application/font-woff2=A21600,application/vnd.ms-fontobject=A21600
}
# deny sensitive files
context exp:^/\.(user\.ini|htaccess|git|svn|env) {
  location                $DOC_ROOT/
  allowBrowse             0
}
`

const spaConf = `# single-page application route fallback, remove if not needed
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule ^ /index.html [L]
`

const errorPageConf = `errorpage 404 {
  url                     /404.html
}
`

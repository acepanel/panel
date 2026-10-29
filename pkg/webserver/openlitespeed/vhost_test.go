package openlitespeed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// newConfigDir 创建带 site/shared 子目录的临时配置目录，父目录即站点目录
func newConfigDir(t *testing.T) string {
	t.Helper()
	configDir := filepath.Join(t.TempDir(), "config")
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared"), 0755))
	return configDir
}

func readConf(t *testing.T, configDir string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(configDir, VhostConfName))
	must.NoError(t, err)
	return string(content)
}

func TestDefaults(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.True(t, vhost.Enable())
	check.Equal(t, vhost.Root(), filepath.Join(filepath.Dir(configDir), "public"))
	check.DeepEqual(t, vhost.Index(), []string{"index.html"})
	check.DeepEqual(t, vhost.Listen(), []types.Listen{{Address: "80", Args: []string{}}})
	check.DeepEqual(t, vhost.ServerName(), []string{"localhost"})
	check.False(t, vhost.SSL())
	check.Nil(t, vhost.SSLConfig())
	check.Equal(t, vhost.PHP(), uint(0))
}

func TestBasicRoundTrip(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl", "quic"}}}))
	check.NoError(t, vhost.SetServerName([]string{"example.com", "www.example.com"}))
	check.NoError(t, vhost.SetRoot("/var/www/html"))
	check.NoError(t, vhost.SetIndex([]string{"index.php", "index.html"}))
	check.NoError(t, vhost.SetAccessLog("/var/log/access.log"))
	check.NoError(t, vhost.SetErrorLog("/var/log/error.log"))
	check.NoError(t, vhost.SetPHP(84))
	check.NoError(t, vhost.SetIncludes([]types.IncludeFile{{Path: "/etc/custom.conf"}}))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "docRoot                  /var/www/html")
	check.Contains(t, conf, "indexFiles               index.php, index.html")
	check.Contains(t, conf, "accesslog /var/log/access.log {")
	check.Contains(t, conf, "errorlog /var/log/error.log {")
	check.Contains(t, conf, "include                  "+phpHandlerFile(84))
	check.Contains(t, conf, "include                  /etc/custom.conf")

	reloaded, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Listen(), []types.Listen{{Address: "80", Args: []string{}}, {Address: "443", Args: []string{"ssl", "quic"}}})
	check.DeepEqual(t, reloaded.ServerName(), []string{"example.com", "www.example.com"})
	check.Equal(t, reloaded.Root(), "/var/www/html")
	check.DeepEqual(t, reloaded.Index(), []string{"index.php", "index.html"})
	check.Equal(t, reloaded.AccessLog(), "/var/log/access.log")
	check.Equal(t, reloaded.ErrorLog(), "/var/log/error.log")
	check.Equal(t, reloaded.PHP(), uint(84))
	check.DeepEqual(t, reloaded.Includes(), []types.IncludeFile{{Path: "/etc/custom.conf"}})

	// 监听记录
	listen, err := os.ReadFile(filepath.Join(configDir, ListenConfName))
	must.NoError(t, err)
	check.Contains(t, string(listen), "listen                   443 ssl quic")
	check.Contains(t, string(listen), "domain                   example.com www.example.com")
}

func TestEnable(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetEnable(false))
	check.False(t, vhost.Enable())
	check.NoError(t, vhost.Save())
	conf := readConf(t, configDir)
	check.Contains(t, conf, "context "+stopURI+" {")
	check.Contains(t, conf, "RewriteRule              ^ "+stopURI+"stop.html [L]")

	check.NoError(t, vhost.SetEnable(true))
	check.NoError(t, vhost.Save())
	check.NotContains(t, readConf(t, configDir), stopURI)
}

func TestSSL(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl", "quic"}}}))
	ssl := &types.SSLConfig{
		Cert:         "/path/fullchain.pem",
		Key:          "/path/private.key",
		Protocols:    []string{"TLSv1.2", "TLSv1.3"},
		HSTS:         true,
		OCSP:         true,
		HTTPRedirect: true,
	}
	check.NoError(t, vhost.SetSSLConfig(ssl))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "sslProtocol              24")
	check.Contains(t, conf, "enableQuic               1")
	check.Contains(t, conf, "enableStapling           1")
	check.Contains(t, conf, "Strict-Transport-Security")
	check.Contains(t, conf, "https://%{HTTP_HOST}%{REQUEST_URI} [R=301,L]")

	reloaded, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.True(t, reloaded.SSL())
	check.DeepEqual(t, reloaded.SSLConfig(), ssl)

	check.NoError(t, reloaded.ClearSSL())
	check.NoError(t, reloaded.Save())
	check.NotContains(t, readConf(t, configDir), "vhssl")
	check.NotContains(t, readConf(t, configDir), "https://%{HTTP_HOST}")
}

func TestProxy(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	upstreams := []types.Upstream{{
		Name:      "backend",
		Servers:   map[string]string{"10.0.0.1:8080": "weight=5", "10.0.0.2:8080": ""},
		Algo:      "least_conn",
		Keepalive: 32,
	}}
	check.NoError(t, vhost.SetUpstreams(upstreams))
	check.NoError(t, vhost.SetProxies([]types.Proxy{
		{
			Location:  "^~ /",
			Pass:      "http://backend",
			Host:      "example.com",
			Buffering: true,
		},
		{
			Location:        "~ ^/api/v[0-9]+/",
			Pass:            "https://api.example.com",
			Headers:         map[string]string{"X-Custom": "1"},
			ResponseHeaders: &types.ResponseHeaderConfig{Add: map[string]string{"X-Cache": "HIT"}, Hide: []string{"X-Powered-By"}},
			AccessControl:   &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"*"}},
		},
		{
			Location: "/ws",
			Pass:     "http://127.0.0.1:3000",
		},
	}))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "type                     loadbalancer")
	check.Contains(t, conf, "workers                  proxy::")
	check.Contains(t, conf, "address                  https://api.example.com:443")
	check.Contains(t, conf, "context exp:^/api/v[0-9]+/ {")
	check.Contains(t, conf, "RequestHeader set Host example.com")
	check.Contains(t, conf, "Header unset X-Powered-By")
	check.Contains(t, conf, "allow                    10.0.0.0/8")
	check.Contains(t, conf, "websocket /ws/ {")
	check.Contains(t, conf, "address                  127.0.0.1:3000")

	reloaded, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Upstreams(), upstreams, cmpopts.EquateEmpty())

	// OLS 没有缓冲开关，Buffering 回读时丢失；未设置的 Host 补成 nginx 的默认变量，其余字段原样保留
	check.DeepEqual(t, reloaded.Proxies(), []types.Proxy{
		{Location: "^~ /", Pass: "http://backend", Host: "example.com"},
		{
			Location:        "~ ^/api/v[0-9]+/",
			Pass:            "https://api.example.com",
			Host:            "$proxy_host",
			Headers:         map[string]string{"X-Custom": "1"},
			ResponseHeaders: &types.ResponseHeaderConfig{Add: map[string]string{"X-Cache": "HIT"}, Hide: []string{"X-Powered-By"}},
			AccessControl:   &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"*"}},
		},
		{Location: "/ws", Pass: "http://127.0.0.1:3000", Host: "$proxy_host"},
	}, cmpopts.EquateEmpty())
}

func TestRedirects(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	redirects := []types.Redirect{
		{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 301},
		{Type: types.RedirectTypeURL, From: "/docs", To: "https://docs.example.com", KeepURI: true, StatusCode: 302},
		{Type: types.RedirectTypeHost, From: "old.example.com", To: "https://example.com", KeepURI: true, StatusCode: 301},
		{Type: types.RedirectType404, To: "/404.html", StatusCode: 308},
	}
	check.NoError(t, vhost.SetRedirects(redirects))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "context exp:^/old$ {")
	check.Contains(t, conf, "context exp:^/docs(.*)$ {")
	check.Contains(t, conf, "location                 https://docs.example.com$1")
	check.Contains(t, conf, `RewriteCond              %{HTTP_HOST} ^old\.example\.com$ [NC]`)
	check.Contains(t, conf, "RewriteRule              ^(.*)$ https://example.com$1 [R=301,L]")
	check.Contains(t, conf, "errorpage 404 {")

	reloaded, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	// 回读顺序按配置块排布：URL 与 404 走 context/errorpage，host 重写排在最后
	check.DeepEqual(t, reloaded.Redirects(), []types.Redirect{
		redirects[0], redirects[1], redirects[3], redirects[2],
	})
}

func TestBasicAuth(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "^~ /", Pass: "http://127.0.0.1:3000"}}))
	auths := []types.BasicAuth{
		{Path: "/", UserFile: "/opt/ace/sites/x/htpasswd_0"},
		{Path: "/admin", UserFile: "/opt/ace/sites/x/htpasswd_1"},
	}
	check.NoError(t, vhost.SetBasicAuth(auths))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "realm "+vhost.realmName(0)+" {")
	check.Contains(t, conf, "location                 /opt/ace/sites/x/htpasswd_1")
	check.Contains(t, conf, "context /admin/ {")
	// 整站认证合并进代理上下文，不再生成静态根上下文
	check.Equal(t, strings.Count(conf, "context / {"), 1)
	check.Contains(t, conf, "realm                    "+vhost.realmName(0)+"\n")

	reloaded, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.BasicAuth(), auths,
		cmpopts.SortSlices(func(a, b types.BasicAuth) bool { return a.Path < b.Path }))

	check.NoError(t, reloaded.ClearBasicAuth())
	check.NoError(t, reloaded.Save())
	check.NotContains(t, readConf(t, configDir), "realm")
}

func TestRootContext(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetSSLConfig(&types.SSLConfig{Cert: "/c", Key: "/k", HSTS: true}))
	check.NoError(t, vhost.SetBasicAuth([]types.BasicAuth{{Path: "/", UserFile: "/opt/ace/sites/x/htpasswd_0"}}))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Equal(t, strings.Count(conf, "context / {"), 1)
	check.Contains(t, conf, "location                 $DOC_ROOT/")
	check.Contains(t, conf, "autoLoadHtaccess         1")
	check.Contains(t, conf, "Header set Strict-Transport-Security max-age=31536000")
	check.NotContains(t, conf, "\nextraHeaders")

	reloaded, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	sslCfg := reloaded.SSLConfig()
	must.NotNil(t, sslCfg)
	check.True(t, sslCfg.HSTS)
	check.DeepEqual(t, reloaded.BasicAuth(), []types.BasicAuth{{Path: "/", UserFile: "/opt/ace/sites/x/htpasswd_0"}})
}

func TestFragments(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetRawConfig("010-rewrite.conf", types.ScopeSite, "RewriteCond %{REQUEST_FILENAME} !-f\nRewriteRule ^ /index.php [L]\n"))
	check.NoError(t, vhost.SetConfig("010-cache.conf", types.ScopeSite, Dialect{}.PHPCacheConf()))
	check.NoError(t, vhost.SetConfig("001-acme.conf", types.ScopeSite, ""))
	check.NoError(t, vhost.SetRawConfig("800-custom.conf", types.ScopeShared, "enableIpGeo 1\n"))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	must.Contains(t, conf, "\nrewrite {")
	rewriteIdx := strings.Index(conf, "\nrewrite {")
	body := conf[rewriteIdx:] //nolint:gocritic
	check.Contains(t, body, "include                  "+filepath.Join(configDir, "site", "010-rewrite.conf"))
	check.NotContains(t, body, "010-cache.conf")
	head := conf[:rewriteIdx]
	check.Contains(t, head, "include                  "+filepath.Join(configDir, "site", "010-cache.conf"))
	check.Contains(t, head, "include                  "+filepath.Join(configDir, "shared", "800-custom.conf"))
	check.NotContains(t, head, "001-acme.conf")
	check.Equal(t, vhost.Config("010-rewrite.conf", types.ScopeSite), "RewriteCond %{REQUEST_FILENAME} !-f\nRewriteRule ^ /index.php [L]")
}

func TestReset(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetPHP(83))
	check.NoError(t, vhost.SetRoot("/tmp/x"))
	check.NoError(t, vhost.Reset())
	check.Equal(t, vhost.PHP(), uint(0))
	check.Equal(t, vhost.Root(), filepath.Join(filepath.Dir(configDir), "public"))
}

func TestLocationToURI(t *testing.T) {
	cases := map[string]string{
		"^~ /":         "/",
		"/":            "/",
		"/api":         "/api/",
		"^~ /static/":  "/static/",
		"~ ^/v1/":      "exp:^/v1/",
		"~* \\.(jpg)$": `exp:(?i)\.(jpg)$`,
		"= /exact":     "/exact",
		"~ ^/a.b/":     "exp:^/a.b/",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			check.Equal(t, locationToURI(in), want)
		})
	}
}

func TestListenerNaming(t *testing.T) {
	check.Equal(t, listenerAddress("80"), "*:80")
	check.Equal(t, listenerAddress("0.0.0.0:8080"), "*:8080")
	check.Equal(t, listenerName("[::]:443"), "ip6_443")
	check.Equal(t, listenerName("*:80"), "any_80")
	check.Equal(t, listenerName("127.0.0.1:8899"), "127_0_0_1_8899")
}

// 代理站点上的子路径认证必须仍走代理，静态上下文会让该路径去磁盘找文件而 404
func TestProxyAuthContext(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/", Pass: "http://127.0.0.1:8080"}}))
	must.NoError(t, vhost.SetBasicAuth([]types.BasicAuth{{Path: "/secret", UserFile: filepath.Join(configDir, "htpasswd_0")}}))
	must.NoError(t, vhost.Save())

	content := readConf(t, configDir)
	_, secret, ok := strings.Cut(content, "context /secret/ {")
	must.True(t, ok)
	secret, _, _ = strings.Cut(secret, "\n}")
	check.Contains(t, secret, "type                     proxy")
	check.Contains(t, secret, "realm                    ")
	check.NotContains(t, secret, "$DOC_ROOT")

	check.DeepEqual(t, vhost.BasicAuth(), []types.BasicAuth{{Path: "/secret", UserFile: filepath.Join(configDir, "htpasswd_0")}}, cmpopts.EquateEmpty())
}

// autoLoadHtaccess 只在 vhost 级生效，写进 context 会被 OLS 静默忽略
func TestHtaccessAtVhostLevel(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.Save())

	cfg, err := Parse(readConf(t, configDir))
	must.NoError(t, err)
	check.Equal(t, cfg.GetBlock("rewrite").Value("autoLoadHtaccess"), "1")
	check.Empty(t, cfg.GetBlock("context", "/").GetBlock("rewrite").Value("autoLoadHtaccess"))
}

func TestProxyCustom(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	custom := "addDefaultCharset        off\n# 注释保留\nenableIpGeo              1"
	check.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://127.0.0.1:8080", Custom: custom}}))
	check.NoError(t, vhost.Save())
	// 要落在该规则的 context 里才只对该规则生效
	check.Contains(t, readConf(t, configDir), "  # ace:custom\n  addDefaultCharset        off\n")

	reloaded, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	got := reloaded.Proxies()
	must.Len(t, got, 1)
	check.Equal(t, got[0].Custom, custom)

	check.Error(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://127.0.0.1:8080", Custom: "rewrite {"}}))
}

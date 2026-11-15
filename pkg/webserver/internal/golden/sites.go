package golden

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// Site 一个已保存的全功能站点，New 用同一份配置目录重新加载
type Site struct {
	Name      string
	ConfigDir string
	New       func() (types.Vhost, error)
}

// Run 搭出全功能站点并断言：整份配置与基准逐字节一致；新建对象重新加载后再保存不变；读到的设置原样写回再保存也不变
func Run(t *testing.T, d types.Dialect, custom string) {
	t.Helper()
	root := t.TempDir()
	cert, key := filepath.Join(root, "cert.pem"), filepath.Join(root, "key.pem")
	for _, site := range Build(t, d, root, "", cert, key, custom) {
		t.Run(site.Name, func(t *testing.T) {
			first := snapshot(t, site.ConfigDir, root)
			assert(t, site.Name, first)

			v, err := site.New()
			must.NoError(t, err)
			must.NoError(t, v.Save())
			must.DeepEqual(t, snapshot(t, site.ConfigDir, root), first)

			v, err = site.New()
			must.NoError(t, err)
			reapply(t, v, cert, key)
			must.DeepEqual(t, snapshot(t, site.ConfigDir, root), first)
		})
	}
}

// Build 在 root/sites 下搭出覆盖全部功能的三个站点：PHP、反向代理、停用的静态站点。
// 四个方言共用同一份站点模型，custom 是该方言原生语法的代理自定义指令；
// 站点名决定片段名，与别的站点同处一个服务器实例时用 prefix 错开
func Build(t *testing.T, d types.Dialect, root, prefix, cert, key, custom string) []Site {
	t.Helper()
	https := d.HTTPSListenArgs()
	var altSvc string
	if slices.Contains(https, "quic") {
		altSvc = `h3=":$server_port"; ma=2592000`
	}
	newSite := func(name string) (string, string) {
		siteDir := filepath.Join(root, "sites", prefix+name)
		for _, dir := range []string{"config/site", "config/shared", "public", "log"} {
			must.NoError(t, os.MkdirAll(filepath.Join(siteDir, dir), 0755))
		}
		return siteDir, filepath.Join(siteDir, "config")
	}
	htpasswd := func(siteDir, name string) string {
		path := filepath.Join(siteDir, name)
		must.NoError(t, os.WriteFile(path, []byte(d.HTPasswdLine("admin", "secret")+"\n"), 0600))
		return path
	}

	phpDir, phpConfig := newSite("php")
	php, err := d.NewPHPVhost(phpConfig)
	must.NoError(t, err)
	must.NoError(t, php.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: https}}))
	must.NoError(t, php.SetServerName([]string{"php.golden.test", "www.php.golden.test"}))
	must.NoError(t, php.SetRoot(filepath.Join(phpDir, "public")))
	must.NoError(t, php.SetIndex([]string{"index.php", "index.html"}))
	must.NoError(t, php.SetAccessLog(filepath.Join(phpDir, "log", "access.log")))
	must.NoError(t, php.SetErrorLog(filepath.Join(phpDir, "log", "error.log")))
	must.NoError(t, php.SetPHP(84))
	must.NoError(t, php.SetSSLConfig(&types.SSLConfig{
		Cert: cert, Key: key, Protocols: []string{"TLSv1.2", "TLSv1.3"},
		HSTS: true, OCSP: true, HTTPRedirect: true, AltSvc: altSvc,
	}))
	must.NoError(t, php.SetRedirects([]types.Redirect{
		{Type: types.RedirectTypeHost, From: "old.golden.test", To: "https://php.golden.test", KeepURI: true, StatusCode: 301},
		{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 302},
		{Type: types.RedirectType404, To: "/", StatusCode: 308},
		{Type: types.RedirectTypeURL, From: "/docs", To: "https://docs.golden.test", KeepURI: true, StatusCode: 308},
	}))
	must.NoError(t, php.SetRateLimit(&types.RateLimit{PerServer: 300, PerIP: 25, Rate: 512}))
	must.NoError(t, php.SetRealIP(&types.RealIP{From: []string{"10.0.0.0/8", "192.168.1.1"}, Header: "X-Forwarded-For", Recursive: true}))
	must.NoError(t, php.SetBasicAuth([]types.BasicAuth{
		{Path: "/", UserFile: htpasswd(phpDir, "htpasswd_0")},
		{Path: "/admin", UserFile: htpasswd(phpDir, "htpasswd_1")},
	}))
	must.NoError(t, php.SetConfig("001-acme.conf", types.ScopeSite, ""))
	must.NoError(t, php.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	must.NoError(t, php.SetConfig("010-cache.conf", types.ScopeSite, d.PHPCacheConf()))
	must.NoError(t, php.SetRawConfig("010-rewrite.conf", types.ScopeSite, ""))
	if shared, site := d.StatConf(prefix + "php"); site != "" {
		if shared != "" {
			must.NoError(t, php.SetConfig("010-stat-format.conf", types.ScopeShared, shared))
		}
		must.NoError(t, php.SetConfig("021-stats-log.conf", types.ScopeSite, site))
	}
	if lscache := d.LSCacheConf(prefix + "php"); lscache != "" {
		must.NoError(t, php.SetConfig("020-lscache.conf", types.ScopeSite, lscache))
	}
	must.NoError(t, php.Save())

	proxyDir, proxyConfig := newSite("proxy")
	proxy, err := d.NewProxyVhost(proxyConfig)
	must.NoError(t, err)
	must.NoError(t, proxy.SetListen([]types.Listen{{Address: "80"}, {Address: "8443", Args: https}}))
	must.NoError(t, proxy.SetServerName([]string{"proxy.golden.test"}))
	must.NoError(t, proxy.SetRoot(filepath.Join(proxyDir, "public")))
	must.NoError(t, proxy.SetAccessLog(filepath.Join(proxyDir, "log", "access.log")))
	must.NoError(t, proxy.SetErrorLog(filepath.Join(proxyDir, "log", "error.log")))
	must.NoError(t, proxy.SetSSLConfig(&types.SSLConfig{Cert: cert, Key: key, Protocols: []string{"TLSv1.3"}}))
	// 用户的共享级自定义配置，保存站点设置不能动它
	must.NoError(t, proxy.SetRawConfig("800-custom.conf", types.ScopeShared, "# custom shared config\n"))
	must.NoError(t, proxy.SetUpstreams([]types.Upstream{
		{Name: "backend", Servers: map[string]string{"127.0.0.1:3001": "weight=5", "127.0.0.1:3002": ""}, Keepalive: 32},
		{
			Name: "pool", Servers: map[string]string{"unix:/tmp/api.sock": "", "10.0.0.2:80": "backup"}, Algo: types.AlgoLeastConn,
			Resolver: []string{"8.8.8.8", "ipv6=off"}, ResolverTimeout: 5 * time.Second,
		},
	}))
	must.NoError(t, proxy.SetProxies([]types.Proxy{
		{
			Location: "/", Pass: "http://backend", Buffering: true,
			// 与内置头同名：改了值的要生效且读得回来，值与内置相同的不重复写
			Headers: map[string]string{
				"X-Forwarded-Host": "$host", "X-Request-Start": "t=$msec",
				"X-Forwarded-Proto": "https", "X-Real-IP": "$remote_addr",
			},
			Replaces: map[string]string{"http://old": "https://new"},
			Cache: &types.CacheConfig{
				Valid: map[string]string{"200 302": "10m"}, NoCacheConditions: []string{"$cookie_nocache", "$arg_nocache"},
				UseStale: []string{"error", "timeout"}, BackgroundUpdate: true, Lock: true, MinUses: 2,
				Methods: []string{"GET", "HEAD"}, Key: "$scheme$host$request_uri",
			},
		},
		{
			Location: "^~ /api/", Pass: "https://10.0.0.5/v2/", Host: "api.golden.test", SNI: "api.internal", HTTPVersion: "2",
			Resolver: []string{"8.8.8.8"}, ResolverTimeout: 5 * time.Second,
			Timeout:           &types.TimeoutConfig{Connect: 5 * time.Second, Read: 90 * time.Second, Send: 30 * time.Second},
			Retry:             &types.RetryConfig{Conditions: []string{"error", "timeout"}, Tries: 3, Timeout: 10 * time.Second},
			ClientMaxBodySize: 10485760,
			SSLBackend:        &types.SSLBackendConfig{Verify: true, TrustedCertificate: cert, VerifyDepth: 2},
			ResponseHeaders:   &types.ResponseHeaderConfig{Hide: []string{"Server"}, Add: map[string]string{"X-Proxy": "ace", "X-Scheme": "$scheme"}},
			// 只填允许列表即白名单
			AccessControl: &types.AccessControlConfig{Allow: []string{"10.0.0.0/8", "192.168.0.0/16"}},
		},
		{
			Location: "~* \\.(jpg|png)$", Pass: "http://pool", Buffering: true, ClientMaxBodySize: 1048576,
			Headers:         map[string]string{"X-Static": "1"},
			ResponseHeaders: &types.ResponseHeaderConfig{Add: map[string]string{"X-Static": "1"}},
		},
		{
			Location: "= /health", Pass: "http://127.0.0.1:9000", Host: "$host", Buffering: true,
			AccessControl: &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
		},
		{Location: "/ws", Pass: "https://backend.golden.test", AccessControl: &types.AccessControlConfig{Deny: []string{"10.0.0.99"}}},
		{
			Location: "/internal", Pass: "http://127.0.0.1:9001", Custom: custom,
			AccessControl: &types.AccessControlConfig{Deny: []string{"all"}},
		},
	}))
	must.NoError(t, proxy.SetBasicAuth([]types.BasicAuth{{Path: "/secret", UserFile: htpasswd(proxyDir, "htpasswd_0")}}))
	must.NoError(t, proxy.SetConfig("001-acme.conf", types.ScopeSite, ""))
	must.NoError(t, proxy.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	must.NoError(t, proxy.Save())

	staticDir, staticConfig := newSite("static")
	static, err := d.NewStaticVhost(staticConfig)
	must.NoError(t, err)
	must.NoError(t, static.SetListen([]types.Listen{{Address: "127.0.0.1:80"}, {Address: "127.0.0.1:443", Args: https}}))
	must.NoError(t, static.SetServerName([]string{"static.golden.test"}))
	must.NoError(t, static.SetRoot(filepath.Join(staticDir, "public")))
	must.NoError(t, static.SetIndex([]string{"index.html", "index.htm"}))
	must.NoError(t, static.SetSSLConfig(&types.SSLConfig{Cert: cert, Key: key}))
	must.NoError(t, static.SetProxies([]types.Proxy{{Location: "^~ /api", Pass: "http://127.0.0.1:3000", Host: "$host", Buffering: true}}))
	must.NoError(t, static.SetRedirects([]types.Redirect{{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 302}}))
	must.NoError(t, static.SetBasicAuth([]types.BasicAuth{{Path: "/private", UserFile: htpasswd(staticDir, "htpasswd_0")}}))
	must.NoError(t, static.SetConfig("001-acme.conf", types.ScopeSite, ""))
	must.NoError(t, static.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	must.NoError(t, static.SetRawConfig("800-spa.conf", types.ScopeSite, d.SPAConf()))
	must.NoError(t, static.SetEnable(false))
	must.NoError(t, static.Save())

	return []Site{
		{Name: prefix + "php", ConfigDir: phpConfig, New: func() (types.Vhost, error) { return d.NewPHPVhost(phpConfig) }},
		{Name: prefix + "proxy", ConfigDir: proxyConfig, New: func() (types.Vhost, error) { return d.NewProxyVhost(proxyConfig) }},
		{Name: prefix + "static", ConfigDir: staticConfig, New: func() (types.Vhost, error) { return d.NewStaticVhost(staticConfig) }},
	}
}

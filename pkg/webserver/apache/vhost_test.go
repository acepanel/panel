package apache

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

func setupPHPVhost(t *testing.T) (*phpVhost, string) {
	t.Helper()
	configDir := t.TempDir()
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))

	vhost, err := newPHPVhost(configDir)
	must.NoError(t, err)
	must.NotNil(t, vhost)
	return vhost, configDir
}

func confFiles(t *testing.T, configDir, scope, prefix, suffix string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(configDir, scope))
	must.NoError(t, err)

	var out []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), suffix) {
			out = append(out, entry.Name())
		}
	}
	return out
}

func setupProxyVhost(t *testing.T) (*proxyVhost, string) {
	t.Helper()
	configDir := t.TempDir()
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared"), 0755))

	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	return vhost, configDir
}

// withProxyDefaults 回读会把没写进配置的 Host 与 HTTP 版本按 mod_proxy 的实际行为补齐
func withProxyDefaults(proxies []types.Proxy) []types.Proxy {
	want := make([]types.Proxy, len(proxies))
	for i, p := range proxies {
		if p.Host == "" {
			p.Host = "$proxy_host"
		}
		if p.HTTPVersion == "" {
			p.HTTPVersion = "1.1"
		}
		want[i] = p
	}
	return want
}

func TestPHPVhostNew(t *testing.T) {
	vhost, configDir := setupPHPVhost(t)

	check.Equal(t, vhost.ConfigDir, configDir)
	check.NotNil(t, vhost.config)
	check.NotNil(t, vhost.vhost)
}

func TestPHPVhostEnable(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	// 默认应该是启用状态
	check.True(t, vhost.Enable())

	// 禁用网站，证书验证路径不能被改写到停用页；停用页不要认证
	check.NoError(t, vhost.SetEnable(false))
	check.False(t, vhost.Enable())
	disable := vhost.Config("00-disable.conf", types.ScopeSite)
	check.Contains(t, disable, "RewriteCond %{REQUEST_URI} !^/\\.well-known/acme-challenge/\nRewriteRule ^ "+disablePage+" [L]")
	check.Contains(t, disable, "<Directory "+htmlDir+">\n    <If \"true\">\n        Require all granted\n    </If>\n</Directory>")

	// 重新启用
	check.NoError(t, vhost.SetEnable(true))
	check.True(t, vhost.Enable())
}

func TestPHPVhostServerName(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	names := []string{"example.com", "www.example.com", "api.example.com"}
	check.NoError(t, vhost.SetServerName(names))
	check.DeepEqual(t, vhost.ServerName(), names)
}

func TestPHPVhostServerNameEmpty(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	// 空输入是空操作，原有域名保持不变
	before := vhost.ServerName()
	check.NoError(t, vhost.SetServerName([]string{}))
	check.DeepEqual(t, vhost.ServerName(), before)
}

func TestPHPVhostRoot(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	root := "/var/www/html"
	check.NoError(t, vhost.SetRoot(root))
	check.Equal(t, vhost.Root(), root)
}

func TestPHPVhostRewrite(t *testing.T) {
	vhost, _ := setupPHPVhost(t)
	root := t.TempDir()
	must.NoError(t, vhost.SetRoot(root))

	rules := "RewriteEngine On\nRewriteRule ^ /index.php [L]\n"
	must.NoError(t, vhost.SetRewrite(rules))
	content, err := os.ReadFile(filepath.Join(root, ".htaccess"))
	must.NoError(t, err)
	check.Equal(t, string(content), rules)
	check.Equal(t, vhost.Rewrite(), rules)
	// 规则跟着根目录走，不落进站点配置
	check.Equal(t, vhost.Config("010-rewrite.conf", types.ScopeSite), "")

	must.NoError(t, vhost.SetRoot(t.TempDir()))
	check.Equal(t, vhost.Rewrite(), "")
}

func TestPHPVhostIndex(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	index := []string{"index.html", "index.php", "default.html"}
	check.NoError(t, vhost.SetIndex(index))
	check.DeepEqual(t, vhost.Index(), index)
}

func TestPHPVhostIndexEmpty(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.NoError(t, vhost.SetIndex([]string{}))
	check.Empty(t, vhost.Index())
}

func TestPHPVhostListen(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	listens := []types.Listen{
		{Address: "*:80"},
		{Address: "*:443"},
	}
	check.NoError(t, vhost.SetListen(listens))
	check.DeepEqual(t, vhost.Listen(), listens, cmpopts.EquateEmpty())
}

func TestPHPVhostSSL(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.False(t, vhost.SSL())
	check.Nil(t, vhost.SSLConfig())
}

func TestPHPVhostSetSSLConfig(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	sslConfig := &types.SSLConfig{
		Cert:      "/etc/ssl/cert.pem",
		Key:       "/etc/ssl/key.pem",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
		HSTS:      true,
		OCSP:      true,
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	check.True(t, vhost.SSL())
	check.Contains(t, Export(vhost.config), "SSLUseStapling on")
	check.DeepEqual(t, vhost.SSLConfig(), sslConfig)
}

func TestPHPVhostSetSSLConfigNil(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.Error(t, vhost.SetSSLConfig(nil))
}

func TestPHPVhostClearSSL(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	sslConfig := &types.SSLConfig{
		Cert: "/etc/ssl/cert.pem",
		Key:  "/etc/ssl/key.pem",
		HSTS: true,
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))
	check.True(t, vhost.SSL())

	check.NoError(t, vhost.ClearSSL())
	check.False(t, vhost.SSL())
}

func TestPHPVhostClearHTTPSPreservesOtherHeaders(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	// 添加一个非 HSTS 的 Header
	vhost.vhost.Add("Header", "set", "X-Custom-Header", "value")

	// 设置 SSL 和 HSTS
	sslConfig := &types.SSLConfig{
		Cert: "/etc/ssl/cert.pem",
		Key:  "/etc/ssl/key.pem",
		HSTS: true,
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	// 清除 HTTPS 只带走 HSTS，自定义 Header 原样保留
	check.NoError(t, vhost.ClearSSL())

	out := Export(vhost.config)
	check.Contains(t, out, "X-Custom-Header")
	check.NotContains(t, out, "Strict-Transport-Security")
}

func TestPHPVhostPHP(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.Equal(t, vhost.PHP(), uint(0))

	check.NoError(t, vhost.SetPHP(84))
	check.Equal(t, vhost.PHP(), uint(84))

	check.NoError(t, vhost.SetPHP(0))
	check.Equal(t, vhost.PHP(), uint(0))
}

func TestPHPVhostAccessLog(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	accessLog := "/var/log/apache/access.log"
	check.NoError(t, vhost.SetAccessLog(accessLog))
	check.Equal(t, vhost.AccessLog(), accessLog)
}

func TestPHPVhostErrorLog(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	errorLog := "/var/log/apache/error.log"
	check.NoError(t, vhost.SetErrorLog(errorLog))
	check.Equal(t, vhost.ErrorLog(), errorLog)
}

func TestPHPVhostBasicAuth(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.Nil(t, vhost.BasicAuth())

	auths := []types.BasicAuth{
		{Path: "/", UserFile: "/etc/htpasswd_0"},
		{Path: "/admin", UserFile: "/etc/htpasswd_1"},
	}
	check.NoError(t, vhost.SetBasicAuth(auths))
	check.DeepEqual(t, vhost.BasicAuth(), auths)
	// 整站认证要放过证书验证路径
	check.Contains(t, Export(vhost.config), "<LocationMatch ^/(?!\\.well-known/acme-challenge/)>\n        AuthType Basic\n")
	check.Contains(t, Export(vhost.config), "<Location /admin>\n        AuthType Basic\n")

	check.NoError(t, vhost.SetBasicAuth(nil))
	check.Nil(t, vhost.BasicAuth())
	check.NotContains(t, Export(vhost.config), "AuthType")

	// 旧版整站认证写成 <Location />
	vhost.vhost.AddBlock("Location", "/").Add("AuthType", "Basic")
	check.DeepEqual(t, vhost.BasicAuth(), []types.BasicAuth{{Path: "/"}})
}

func TestPHPVhostBasicAuthOrder(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	// Location 后声明者覆盖先声明者，整站规则应排在目录规则之前声明
	check.NoError(t, vhost.SetBasicAuth([]types.BasicAuth{
		{Path: "/admin", UserFile: "/etc/htpasswd_0"},
		{Path: "/", UserFile: "/etc/htpasswd_1"},
	}))
	check.DeepEqual(t, vhost.BasicAuth(), []types.BasicAuth{
		{Path: "/", UserFile: "/etc/htpasswd_1"},
		{Path: "/admin", UserFile: "/etc/htpasswd_0"},
	})
}

func TestPHPVhostBasicAuthLegacy(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	// 旧版 vhost 级整站配置应能读出并被清除
	vhost.vhost.Set("AuthType", "Basic")
	vhost.vhost.Set("AuthName", "Restricted")
	vhost.vhost.Set("AuthUserFile", "/etc/htpasswd")
	vhost.vhost.Set("Require", "valid-user")

	check.DeepEqual(t, vhost.BasicAuth(), []types.BasicAuth{{Path: "/", UserFile: "/etc/htpasswd"}})

	check.NoError(t, vhost.SetBasicAuth(nil))
	check.Nil(t, vhost.BasicAuth())
}

func TestPHPVhostRateLimit(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.Nil(t, vhost.RateLimit())

	limit := &types.RateLimit{
		Rate: 512,
	}
	check.NoError(t, vhost.SetRateLimit(limit))
	check.DeepEqual(t, vhost.RateLimit(), limit)

	check.NoError(t, vhost.ClearRateLimit())
	check.Nil(t, vhost.RateLimit())
}

func TestPHPVhostSave(t *testing.T) {
	vhost, configDir := setupPHPVhost(t)

	check.NoError(t, vhost.SetServerName([]string{"save-test.com"}))
	check.NoError(t, vhost.Save())

	// 验证配置文件已保存
	configFile := filepath.Join(configDir, "apache.conf")
	content, err := os.ReadFile(configFile)
	must.NoError(t, err)
	check.Contains(t, string(content), "save-test.com")
}

func TestPHPVhostExport(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.NoError(t, vhost.SetServerName([]string{"export-test.com"}))
	check.NoError(t, vhost.SetRoot("/var/www/export-test"))

	content := Export(vhost.config)
	check.Contains(t, content, "export-test.com")
	check.Contains(t, content, "/var/www/export-test")
	check.Contains(t, content, "<VirtualHost")
	check.Contains(t, content, "</VirtualHost>")
}

func TestPHPVhostExportWithSSL(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	sslConfig := &types.SSLConfig{
		Cert:      "/etc/ssl/cert.pem",
		Key:       "/etc/ssl/key.pem",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	content := Export(vhost.config)
	check.Contains(t, content, "SSLEngine on")
	check.Contains(t, content, "SSLCertificateFile")
	check.Contains(t, content, "SSLCertificateKeyFile")
}

func TestPHPVhostListenProtocolDetection(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	listens := []types.Listen{
		{Address: "*:443", Args: []string{"ssl"}},
	}
	check.NoError(t, vhost.SetListen(listens))

	sslConfig := &types.SSLConfig{
		Cert: "/etc/ssl/cert.pem",
		Key:  "/etc/ssl/key.pem",
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	// 开启 SSL 不应改写监听项
	check.DeepEqual(t, vhost.Listen(), listens, cmpopts.EquateEmpty())
}

func TestPHPVhostDirectoryBlock(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	root := "/var/www/test-dir"
	check.NoError(t, vhost.SetRoot(root))

	content := Export(vhost.config)
	check.Contains(t, content, "<Directory "+root+">")
	check.Contains(t, content, "</Directory>")
}

func TestPHPVhostFilesMatchBlock(t *testing.T) {
	vhost, _ := setupPHPVhost(t)

	check.NoError(t, vhost.SetPHP(84))

	content := vhost.Config("010-php.conf", "site")
	check.Contains(t, content, "proxy:unix:/tmp/php-cgi-84.sock|fcgi://localhost/")
}

func TestDefaultVhostConfIncludesServerD(t *testing.T) {
	// 验证默认配置包含 site 的 include
	check.Contains(t, defaultVhostConf, "site")
	check.Contains(t, defaultVhostConf, "IncludeOptional")
}

func TestPHPVhostRedirects(t *testing.T) {
	vhost, configDir := setupPHPVhost(t)

	// 初始应该没有重定向
	check.Empty(t, vhost.Redirects())

	// 设置重定向
	redirects := []types.Redirect{
		{
			Type:       types.RedirectTypeURL,
			From:       "/old",
			To:         "/new",
			StatusCode: 301,
		},
		{
			Type:       types.RedirectTypeHost,
			From:       "old.example.com",
			To:         "https://new.example.com",
			KeepURI:    true,
			StatusCode: 308,
		},
		{Type: types.RedirectTypeURL, From: "/docs/a.b", To: "https://docs.example.com", KeepURI: true, StatusCode: 302},
		{Type: types.RedirectTypeHost, From: "www.example.com", To: "https://example.com/", StatusCode: 301},
	}
	check.NoError(t, vhost.SetRedirects(redirects))

	// 验证重定向文件已创建
	check.Len(t, confFiles(t, configDir, "site", "1", "-redirect.conf"), 4)

	check.DeepEqual(t, vhost.Redirects(), redirects)
}

func TestPHPVhostRedirectURL(t *testing.T) {
	vhost, configDir := setupPHPVhost(t)

	redirects := []types.Redirect{
		{
			Type:       types.RedirectTypeURL,
			From:       "/old-page",
			To:         "/new-page",
			StatusCode: 301,
		},
	}
	check.NoError(t, vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	must.NoError(t, err)

	// 只匹配这一个路径，不保留 URI 时连查询串一起丢掉
	check.Contains(t, string(content), "RewriteEngine on\nRewriteRule ^/old-page$ /new-page [R=301,L,QSD]\n")

	// 保留 URI 即接上原始路径，查询串由 mod_rewrite 自动带上
	redirects[0].KeepURI = true
	check.NoError(t, vhost.SetRedirects(redirects))
	check.Contains(t, readFile(t, filepath.Join(siteDir, "100-redirect.conf")), "RewriteRule ^/old-page$ /new-page%{REQUEST_URI} [R=301,L]\n")
	check.DeepEqual(t, vhost.Redirects(), redirects)

	// 旧版用 mod_alias 写的也读得回来
	must.NoError(t, os.WriteFile(filepath.Join(siteDir, "100-redirect.conf"), []byte("RedirectMatch 302 ^/old(.*)$ /new$1\n"), 0600))
	check.DeepEqual(t, vhost.Redirects(), []types.Redirect{{Type: types.RedirectTypeURL, From: "/old", To: "/new", KeepURI: true, StatusCode: 302}})
}

func TestPHPVhostRedirectHost(t *testing.T) {
	vhost, configDir := setupPHPVhost(t)

	redirects := []types.Redirect{
		{
			Type:       types.RedirectTypeHost,
			From:       "old.example.com",
			To:         "https://new.example.com",
			KeepURI:    true,
			StatusCode: 308,
		},
	}
	check.NoError(t, vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	must.NoError(t, err)

	check.Contains(t, string(content), "RewriteEngine")
	check.Contains(t, string(content), "RewriteCond")
	check.Contains(t, string(content), "old.example.com")
	check.Contains(t, string(content), "R=308")
}

func TestPHPVhostRedirect404(t *testing.T) {
	vhost, configDir := setupPHPVhost(t)
	file := filepath.Join(configDir, "site", "100-redirect.conf")

	// 未指定状态码按 308，不保留 URI
	must.NoError(t, vhost.SetRedirects([]types.Redirect{{Type: types.RedirectType404, To: "/custom-404.html"}}))
	check.Contains(t, readFile(t, file), `ErrorDocument 404 /.ace-404
RewriteEngine on
RewriteRule ^/\.ace-404$ /custom-404.html [R=308,L,QSD]
<FilesMatch \.php$>
    ProxyErrorOverride On 404
</FilesMatch>
`)
	check.DeepEqual(t, vhost.Redirects(), []types.Redirect{{Type: types.RedirectType404, To: "/custom-404.html", StatusCode: 308}})

	// 保留 URI 即接上原始请求的路径与查询串
	redirects := []types.Redirect{{Type: types.RedirectType404, To: "https://new.example.com", KeepURI: true, StatusCode: 302}}
	must.NoError(t, vhost.SetRedirects(redirects))
	check.Contains(t, readFile(t, file), `ErrorDocument 404 /.ace-404
RewriteEngine on
RewriteCond %{THE_REQUEST} ^\S+\s([^?\s]*)\??(\S*)
RewriteRule ^/\.ace-404$ https://new.example.com%1?%2 [R=302,L,NE]
`)
	check.DeepEqual(t, vhost.Redirects(), redirects)

	// 旧版把目标当错误页，读得回来，重新保存后换成真重定向
	must.NoError(t, os.WriteFile(file, []byte("ErrorDocument 404 /\n"), 0600))
	check.DeepEqual(t, vhost.Redirects(), []types.Redirect{{Type: types.RedirectType404, To: "/", StatusCode: 308}})
}

func TestProxyVhostProxies(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)

	// 初始应该没有代理配置
	check.Empty(t, vhost.Proxies())

	// 设置代理配置
	proxies := []types.Proxy{
		{
			Location: "/",
			Pass:     "http://backend:8080/",
			Host:     "example.com",
		},
		{
			Location:  "/api",
			Pass:      "http://api-backend:8080/",
			Buffering: true,
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 全部规则写在一个文件里
	check.Len(t, confFiles(t, configDir, "site", "2", "-proxy.conf"), 1)

	check.DeepEqual(t, vhost.Proxies(), withProxyDefaults(proxies), cmpopts.EquateEmpty())
}

func TestProxyVhostProxyConfig(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)

	proxies := []types.Proxy{
		{
			Location:  "/",
			Pass:      "http://backend:8080/",
			Host:      "example.com",
			Buffering: true,
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 读取配置文件内容
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	must.NoError(t, err)

	check.Contains(t, string(content), "ProxyPassMatch ^/(.*)$ http://backend:8080/$1 upgrade=websocket\n")
	check.Contains(t, string(content), "ProxyPassReverse / http://backend:8080/\n")
	check.Contains(t, string(content), "RequestHeader set Host example.com\n")
}

func TestProxyVhostClearProxies(t *testing.T) {
	vhost, _ := setupProxyVhost(t)

	proxies := []types.Proxy{
		{Location: "/", Pass: "http://backend/"},
	}
	check.NoError(t, vhost.SetProxies(proxies))
	check.Len(t, vhost.Proxies(), 1)

	check.NoError(t, vhost.SetProxies(nil))
	check.Empty(t, vhost.Proxies())
}

func TestProxyVhostUpstreams(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)

	// 初始应该没有上游服务器配置
	check.Empty(t, vhost.Upstreams())

	// 设置上游服务器（Apache 使用 balancer）
	upstreams := []types.Upstream{
		{
			Name: "backend",
			Servers: map[string]string{
				"127.0.0.1:8080": "weight=5",
				"127.0.0.1:8081": "weight=3",
			},
			Algo: "least_conn",
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))

	// 验证 balancer 文件已创建
	check.NotEmpty(t, confFiles(t, configDir, "shared", "", ""))

	check.DeepEqual(t, vhost.Upstreams(), upstreams, cmpopts.EquateEmpty())
}

func TestProxyVhostBalancerConfig(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)

	upstreams := []types.Upstream{
		{
			Name: "mybackend",
			Servers: map[string]string{
				"127.0.0.1:8080": "weight=5",
			},
			Algo:      "least_conn",
			Keepalive: 16,
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))

	// 读取配置文件内容
	files := confFiles(t, configDir, "shared", "", "")
	must.NotEmpty(t, files)
	content, err := os.ReadFile(filepath.Join(configDir, "shared", files[0]))
	must.NoError(t, err)

	check.Contains(t, string(content), "balancer://mybackend")
	check.Contains(t, string(content), "BalancerMember")
	check.Contains(t, string(content), "http://127.0.0.1:8080")
	check.Contains(t, string(content), "lbmethod=bybusyness")
}

func TestProxyVhostClearUpstreams(t *testing.T) {
	vhost, _ := setupProxyVhost(t)

	upstreams := []types.Upstream{
		{
			Name:    "backend",
			Servers: map[string]string{"http://127.0.0.1:8080": ""},
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))
	check.Len(t, vhost.Upstreams(), 1)

	check.NoError(t, vhost.SetUpstreams(nil))
	check.Empty(t, vhost.Upstreams())
}

func TestProxyVhostSubstitute(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)

	// 测试内容替换的写入和解析
	proxies := []types.Proxy{
		{
			Location: "/",
			Pass:     "http://backend:8080/",
			Replaces: map[string]string{
				"http://old.example.com": "https://new.example.com",
				"foo":                    "bar",
			},
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 读取配置文件内容，验证 Substitute 已写入
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	must.NoError(t, err)

	check.Contains(t, string(content), "mod_substitute")
	check.Contains(t, string(content), "Substitute")

	check.DeepEqual(t, vhost.Proxies(), withProxyDefaults(proxies), cmpopts.EquateEmpty())
}

func TestProxyVhostSubstituteWithSlash(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)

	// 测试包含 / 的内容替换
	proxies := []types.Proxy{
		{
			Location: "/",
			Pass:     "http://backend:8080/",
			Replaces: map[string]string{
				"http://old.example.com/path/to/resource": "https://new.example.com/new/path",
			},
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 读取配置文件内容
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	must.NoError(t, err)

	// 验证使用 | 作为分隔符
	check.Contains(t, string(content), "Substitute \"s|http://old.example.com/path/to/resource|https://new.example.com/new/path|n\"")

	check.DeepEqual(t, vhost.Proxies(), withProxyDefaults(proxies), cmpopts.EquateEmpty())
}

func TestProxyVhostUpstreamMultipleServers(t *testing.T) {
	vhost, _ := setupProxyVhost(t)

	// 测试多个 BalancerMember 的解析
	upstreams := []types.Upstream{
		{
			Name: "test_upstream",
			Servers: map[string]string{
				"127.0.0.1:8080": "",
				"127.0.0.1:8081": "",
				"127.0.0.1:8082": "weight=5",
			},
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))

	check.DeepEqual(t, vhost.Upstreams(), upstreams, cmpopts.EquateEmpty())
}

func TestSyncListen(t *testing.T) {
	root := t.TempDir()
	confDir := filepath.Join(root, "conf")
	must.NoError(t, os.MkdirAll(filepath.Join(confDir, "extra"), 0755))
	mainConf := filepath.Join(confDir, "httpd.conf")
	must.NoError(t, os.WriteFile(mainConf, []byte("Listen 80\nListen 443\n"), 0644))
	listenConf := filepath.Join(confDir, "extra", listenConfName)

	newSite := func(name string, addrs ...string) {
		dir := filepath.Join(root, "sites", name, "config")
		must.NoError(t, os.MkdirAll(dir, 0755))
		content := "<VirtualHost " + strings.Join(addrs, " ") + ">\n    ServerName " + name + "\n</VirtualHost>\n"
		must.NoError(t, os.WriteFile(filepath.Join(dir, configName), []byte(content), 0600))
	}
	sites := filepath.Join(root, "sites")

	// 只有标准端口时不写监听项，但引用行要补上
	newSite("a", "*:80", "*:443")
	must.NoError(t, syncListen(sites, confDir))
	check.NotContains(t, readFile(t, listenConf), "Listen")
	check.Contains(t, readFile(t, mainConf), "IncludeOptional conf/extra/"+listenConfName)

	// 非标端口去重后补齐
	newSite("b", "*:8080", "[::]:8443")
	newSite("c", "127.0.0.1:8080")
	must.NoError(t, syncListen(sites, confDir))
	check.Contains(t, readFile(t, listenConf), "Listen 8080\nListen 8443\n")

	// 重复执行不累加
	before := readFile(t, mainConf)
	must.NoError(t, syncListen(sites, confDir))
	check.Equal(t, readFile(t, mainConf), before)

	// 端口不再使用时随之移除
	must.NoError(t, os.RemoveAll(filepath.Join(sites, "b")))
	must.NoError(t, os.RemoveAll(filepath.Join(sites, "c")))
	must.NoError(t, syncListen(sites, confDir))
	check.NotContains(t, readFile(t, listenConf), "Listen")
}

func TestProxyVhostCustom(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)
	custom := "Header always set X-Custom value\n# 注释保留\nRequestHeader unset Accept-Encoding"
	must.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://backend:8080", Custom: custom}}))
	cfg, err := ParseFile(filepath.Join(configDir, "site", "200-proxy.conf"))
	must.NoError(t, err)
	// 要落在这条规则自己那一节里才只对它生效
	section := cfg.FindOne("IfModule.ElseIf")
	must.NotNil(t, section)
	check.Equal(t, section.Arg(0), "%{REQUEST_URI} =~ m#^/api#")
	check.Len(t, section.GetAll("Header"), 1)

	got := vhost.Proxies()
	must.Len(t, got, 1)
	check.Equal(t, got[0].Custom, custom)
	// 自定义指令里的响应头不回读成面板字段
	check.Nil(t, got[0].ResponseHeaders)

	// 未闭合的标签会破坏外层结构，直接报错，已有文件保持原样
	check.Error(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://backend:8080", Custom: "<Location /x>"}}))
	check.Error(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://backend:8080", Custom: "</Location>"}}))
	check.Equal(t, vhost.Proxies()[0].Custom, custom)
}

// 静态与 PHP 站点也能把子路径交给后端
func TestStaticVhostProxies(t *testing.T) {
	configDir := t.TempDir()
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared"), 0755))
	vhost, err := newStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetUpstreams([]types.Upstream{{Name: "api", Servers: map[string]string{"127.0.0.1:3000": ""}}}))
	check.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "^~ /api", Pass: "http://api"}}))

	content, err := os.ReadFile(filepath.Join(configDir, "site", "200-proxy.conf"))
	must.NoError(t, err)
	check.Contains(t, string(content), "ProxyPassMatch ^/api balancer://api\n")
	check.Len(t, vhost.Upstreams(), 1)
	check.Len(t, vhost.Proxies(), 1)
}

// 手写的主文件保存后不能被打散或改写：RewriteCond 紧跟自己的 RewriteRule，If 条件链保持先后，
// 限速只动自己那两条，双引号里的反斜杠不随保存次数减半
func TestSaveKeepsHandwritten(t *testing.T) {
	configDir := t.TempDir()
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	handwritten := `<VirtualHost *:80>
    ServerName example.com
    DocumentRoot /var/www/html
    SetEnv APP_ENV production
    SetOutputFilter DEFLATE
    RewriteEngine On
    RewriteCond %{REQUEST_URI} ^/a
    RewriteRule ^ /x [L]
    RewriteCond %{REQUEST_URI} ^/b
    RewriteRule ^ /y [L]
    <If "%{HTTP_HOST} == 'a.example.com'">
        Header set X-Site a
    </If>
    <ElseIf "%{HTTP_HOST} == 'b.example.com'">
        Header set X-Site b
    </ElseIf>
    <Else>
        Header set X-Site other
    </Else>
    LogFormat "%h \"%r\" C:\\dir" custom
</VirtualHost>
`
	configFile := filepath.Join(configDir, configName)
	must.NoError(t, os.WriteFile(configFile, []byte(handwritten), 0600))

	vhost, err := newStaticVhost(configDir)
	must.NoError(t, err)
	check.Nil(t, vhost.RateLimit())
	must.NoError(t, vhost.SetRateLimit(&types.RateLimit{Rate: 512}))
	must.NoError(t, vhost.Save())
	check.Contains(t, readFile(t, configFile), "    SetEnv APP_ENV production\n    SetEnv rate-limit 512\n    SetOutputFilter DEFLATE\n    SetOutputFilter RATE_LIMIT\n")

	vhost, err = newStaticVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, vhost.RateLimit(), &types.RateLimit{Rate: 512})
	must.NoError(t, vhost.ClearRateLimit())
	must.NoError(t, vhost.Save())
	check.Equal(t, readFile(t, configFile), handwritten)

	for range 3 {
		vhost, err = newStaticVhost(configDir)
		must.NoError(t, err)
		must.NoError(t, vhost.Save())
	}
	check.Equal(t, readFile(t, configFile), handwritten)
}

// 超限要在动现有文件之前报出来
func TestPHPVhostRedirectLimit(t *testing.T) {
	vhost, _ := setupPHPVhost(t)
	redirects := []types.Redirect{{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 301}}
	must.NoError(t, vhost.SetRedirects(redirects))

	tooMany := make([]types.Redirect, redirectEndNum-redirectStartNum+2)
	for i := range tooMany {
		tooMany[i] = types.Redirect{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 301}
	}
	check.Error(t, vhost.SetRedirects(tooMany))
	check.DeepEqual(t, vhost.Redirects(), redirects)
}

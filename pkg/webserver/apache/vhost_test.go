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

func newPHPVhost(t *testing.T) (*PHPVhost, string) {
	t.Helper()
	configDir := t.TempDir()
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))

	vhost, err := NewPHPVhost(configDir)
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

func newProxyVhost(t *testing.T) (*ProxyVhost, string) {
	t.Helper()
	configDir := t.TempDir()
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared"), 0755))

	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	return vhost, configDir
}

func TestPHPVhostNew(t *testing.T) {
	vhost, configDir := newPHPVhost(t)

	check.Equal(t, vhost.configDir, configDir)
	check.NotNil(t, vhost.config)
	check.NotNil(t, vhost.vhost)
}

func TestPHPVhostEnable(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	// 默认应该是启用状态
	check.True(t, vhost.Enable())

	// 禁用网站
	check.NoError(t, vhost.SetEnable(false))
	check.False(t, vhost.Enable())

	// 重新启用
	check.NoError(t, vhost.SetEnable(true))
	check.True(t, vhost.Enable())
}

func TestPHPVhostServerName(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	names := []string{"example.com", "www.example.com", "api.example.com"}
	check.NoError(t, vhost.SetServerName(names))
	check.DeepEqual(t, vhost.ServerName(), names)
}

func TestPHPVhostServerNameEmpty(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	// 空输入是空操作，原有域名保持不变
	before := vhost.ServerName()
	check.NoError(t, vhost.SetServerName([]string{}))
	check.DeepEqual(t, vhost.ServerName(), before)
}

func TestPHPVhostRoot(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	root := "/var/www/html"
	check.NoError(t, vhost.SetRoot(root))
	check.Equal(t, vhost.Root(), root)
}

func TestPHPVhostIndex(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	index := []string{"index.html", "index.php", "default.html"}
	check.NoError(t, vhost.SetIndex(index))
	check.DeepEqual(t, vhost.Index(), index)
}

func TestPHPVhostIndexEmpty(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.NoError(t, vhost.SetIndex([]string{}))
	check.Empty(t, vhost.Index())
}

func TestPHPVhostListen(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	listens := []types.Listen{
		{Address: "*:80"},
		{Address: "*:443"},
	}
	check.NoError(t, vhost.SetListen(listens))
	check.DeepEqual(t, vhost.Listen(), listens, cmpopts.EquateEmpty())
}

func TestPHPVhostSSL(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.False(t, vhost.SSL())
	check.Nil(t, vhost.SSLConfig())
}

func TestPHPVhostSetSSLConfig(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	sslConfig := &types.SSLConfig{
		Cert:      "/etc/ssl/cert.pem",
		Key:       "/etc/ssl/key.pem",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
		HSTS:      true,
		OCSP:      true,
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	check.True(t, vhost.SSL())
	check.DeepEqual(t, vhost.SSLConfig(), sslConfig)
}

func TestPHPVhostSetSSLConfigNil(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.Error(t, vhost.SetSSLConfig(nil))
}

func TestPHPVhostClearSSL(t *testing.T) {
	vhost, _ := newPHPVhost(t)

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
	vhost, _ := newPHPVhost(t)

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
	vhost, _ := newPHPVhost(t)

	check.Equal(t, vhost.PHP(), uint(0))

	check.NoError(t, vhost.SetPHP(84))
	check.Equal(t, vhost.PHP(), uint(84))

	check.NoError(t, vhost.SetPHP(0))
	check.Equal(t, vhost.PHP(), uint(0))
}

func TestPHPVhostAccessLog(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	accessLog := "/var/log/apache/access.log"
	check.NoError(t, vhost.SetAccessLog(accessLog))
	check.Equal(t, vhost.AccessLog(), accessLog)
}

func TestPHPVhostErrorLog(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	errorLog := "/var/log/apache/error.log"
	check.NoError(t, vhost.SetErrorLog(errorLog))
	check.Equal(t, vhost.ErrorLog(), errorLog)
}

func TestPHPVhostIncludes(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	includes := []types.IncludeFile{
		{Path: "/etc/apache/conf.d/ssl.conf"},
		{Path: "/etc/apache/conf.d/php.conf"},
	}
	check.NoError(t, vhost.SetIncludes(includes))
	check.DeepEqual(t, vhost.Includes(), includes)
}

func TestPHPVhostBasicAuth(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.Nil(t, vhost.BasicAuth())

	auths := []types.BasicAuth{
		{Path: "/", UserFile: "/etc/htpasswd_0"},
		{Path: "/admin", UserFile: "/etc/htpasswd_1"},
	}
	check.NoError(t, vhost.SetBasicAuth(auths))
	check.DeepEqual(t, vhost.BasicAuth(), auths)

	check.NoError(t, vhost.ClearBasicAuth())
	check.Nil(t, vhost.BasicAuth())
}

func TestPHPVhostBasicAuthOrder(t *testing.T) {
	vhost, _ := newPHPVhost(t)

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
	vhost, _ := newPHPVhost(t)

	// 旧版 vhost 级整站配置应能读出并被清除
	vhost.vhost.Set("AuthType", "Basic")
	vhost.vhost.Set("AuthName", "Restricted")
	vhost.vhost.Set("AuthUserFile", "/etc/htpasswd")
	vhost.vhost.Set("Require", "valid-user")

	check.DeepEqual(t, vhost.BasicAuth(), []types.BasicAuth{{Path: "/", UserFile: "/etc/htpasswd"}})

	check.NoError(t, vhost.ClearBasicAuth())
	check.Nil(t, vhost.BasicAuth())
}

func TestPHPVhostRateLimit(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.Nil(t, vhost.RateLimit())

	limit := &types.RateLimit{
		Rate: 512,
	}
	check.NoError(t, vhost.SetRateLimit(limit))
	check.DeepEqual(t, vhost.RateLimit(), limit)

	check.NoError(t, vhost.ClearRateLimit())
	check.Nil(t, vhost.RateLimit())
}

func TestPHPVhostReset(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	// 重置后应完全回到初始模板，而不只是丢掉改动值
	names, root := vhost.ServerName(), vhost.Root()
	check.NoError(t, vhost.SetServerName([]string{"modified.com"}))
	check.NoError(t, vhost.SetRoot("/modified/path"))

	check.NoError(t, vhost.Reset())
	check.DeepEqual(t, vhost.ServerName(), names)
	check.Equal(t, vhost.Root(), root)
}

func TestPHPVhostSave(t *testing.T) {
	vhost, configDir := newPHPVhost(t)

	check.NoError(t, vhost.SetServerName([]string{"save-test.com"}))
	check.NoError(t, vhost.Save())

	// 验证配置文件已保存
	configFile := filepath.Join(configDir, "apache.conf")
	content, err := os.ReadFile(configFile)
	must.NoError(t, err)
	check.Contains(t, string(content), "save-test.com")
}

func TestPHPVhostExport(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.NoError(t, vhost.SetServerName([]string{"export-test.com"}))
	check.NoError(t, vhost.SetRoot("/var/www/export-test"))

	content := Export(vhost.config)
	check.Contains(t, content, "export-test.com")
	check.Contains(t, content, "/var/www/export-test")
	check.Contains(t, content, "<VirtualHost")
	check.Contains(t, content, "</VirtualHost>")
}

func TestPHPVhostExportWithSSL(t *testing.T) {
	vhost, _ := newPHPVhost(t)

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
	vhost, _ := newPHPVhost(t)

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
	vhost, _ := newPHPVhost(t)

	root := "/var/www/test-dir"
	check.NoError(t, vhost.SetRoot(root))

	content := Export(vhost.config)
	check.Contains(t, content, "<Directory "+root+">")
	check.Contains(t, content, "</Directory>")
}

func TestPHPVhostFilesMatchBlock(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.NoError(t, vhost.SetPHP(84))

	content := vhost.Config("010-php.conf", "site")
	check.Contains(t, content, "proxy:unix:/tmp/php-cgi-84.sock|fcgi://localhost/")
}

func TestDefaultVhostConfIncludesServerD(t *testing.T) {
	// 验证默认配置包含 site 的 include
	check.Contains(t, DefaultVhostConf, "site")
	check.Contains(t, DefaultVhostConf, "IncludeOptional")
}

func TestPHPVhostRedirects(t *testing.T) {
	vhost, configDir := newPHPVhost(t)

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
	}
	check.NoError(t, vhost.SetRedirects(redirects))

	// 验证重定向文件已创建
	check.Len(t, confFiles(t, configDir, "site", "1", "-redirect.conf"), 2)

	check.DeepEqual(t, vhost.Redirects(), redirects)
}

func TestPHPVhostRedirectURL(t *testing.T) {
	vhost, configDir := newPHPVhost(t)

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

	check.Contains(t, string(content), "Redirect 301")
	check.Contains(t, string(content), "/old-page")
	check.Contains(t, string(content), "/new-page")
}

func TestPHPVhostRedirectHost(t *testing.T) {
	vhost, configDir := newPHPVhost(t)

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
	vhost, configDir := newPHPVhost(t)

	redirects := []types.Redirect{
		{
			Type: types.RedirectType404,
			To:   "/custom-404.html",
		},
	}
	check.NoError(t, vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	must.NoError(t, err)

	check.Contains(t, string(content), "ErrorDocument 404")
	check.Contains(t, string(content), "/custom-404.html")
}

func TestProxyVhostProxies(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

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

	// 验证代理文件已创建
	check.Len(t, confFiles(t, configDir, "site", "2", "-proxy.conf"), 2)

	check.DeepEqual(t, vhost.Proxies(), proxies, cmpopts.EquateEmpty())
}

func TestProxyVhostProxyConfig(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

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

	check.Contains(t, string(content), "ProxyPass /")
	check.Contains(t, string(content), "ProxyPassReverse")
	check.Contains(t, string(content), "http://backend:8080/")
	check.Contains(t, string(content), "RequestHeader set Host")
	check.Contains(t, string(content), "example.com")
}

func TestProxyVhostClearProxies(t *testing.T) {
	vhost, _ := newProxyVhost(t)

	proxies := []types.Proxy{
		{Location: "/", Pass: "http://backend/"},
	}
	check.NoError(t, vhost.SetProxies(proxies))
	check.Len(t, vhost.Proxies(), 1)

	check.NoError(t, vhost.ClearProxies())
	check.Empty(t, vhost.Proxies())
}

func TestProxyVhostUpstreams(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

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
			Algo:      "least_conn",
			Keepalive: 32,
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))

	// 验证 balancer 文件已创建
	check.NotEmpty(t, confFiles(t, configDir, "shared", "", ""))

	check.DeepEqual(t, vhost.Upstreams(), upstreams, cmpopts.EquateEmpty())
}

func TestProxyVhostBalancerConfig(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

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
	vhost, _ := newProxyVhost(t)

	upstreams := []types.Upstream{
		{
			Name:    "backend",
			Servers: map[string]string{"http://127.0.0.1:8080": ""},
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))
	check.Len(t, vhost.Upstreams(), 1)

	check.NoError(t, vhost.ClearUpstreams())
	check.Empty(t, vhost.Upstreams())
}

func TestProxyVhostSNI(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

	// 测试 SNI 配置的写入和解析
	proxies := []types.Proxy{
		{
			Location: "/",
			Pass:     "https://backend:443/",
			SNI:      "backend.example.com",
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 读取配置文件内容，验证 SNI 已写入
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	must.NoError(t, err)

	check.Contains(t, string(content), "SSLProxyEngine On")
	check.Contains(t, string(content), "# ace:sni backend.example.com")

	check.DeepEqual(t, vhost.Proxies(), proxies, cmpopts.EquateEmpty())
}

func TestProxyVhostSubstitute(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

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

	check.DeepEqual(t, vhost.Proxies(), proxies, cmpopts.EquateEmpty())
}

func TestProxyVhostSubstituteWithSlash(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

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

	check.DeepEqual(t, vhost.Proxies(), proxies, cmpopts.EquateEmpty())
}

func TestProxyVhostUpstreamMultipleServers(t *testing.T) {
	vhost, _ := newProxyVhost(t)

	// 测试多个 BalancerMember 的解析
	upstreams := []types.Upstream{
		{
			Name: "test_upstream",
			Servers: map[string]string{
				"127.0.0.1:8080": "",
				"127.0.0.1:8081": "",
				"127.0.0.1:8082": "weight=5",
			},
			Keepalive: 32,
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
	listenConf := filepath.Join(confDir, "extra", ListenConf)

	newSite := func(name string, addrs ...string) {
		dir := filepath.Join(root, "sites", name, "config")
		must.NoError(t, os.MkdirAll(dir, 0755))
		content := "<VirtualHost " + strings.Join(addrs, " ") + ">\n    ServerName " + name + "\n</VirtualHost>\n"
		must.NoError(t, os.WriteFile(filepath.Join(dir, ConfigName), []byte(content), 0600))
	}
	sites := filepath.Join(root, "sites")

	// 只有标准端口时不写监听项，但引用行要补上
	newSite("a", "*:80", "*:443")
	must.NoError(t, syncListen(sites, confDir))
	check.NotContains(t, readFile(t, listenConf), "Listen")
	check.Contains(t, readFile(t, mainConf), "IncludeOptional conf/extra/"+ListenConf)

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
	vhost, configDir := newProxyVhost(t)
	custom := "Header always set X-Custom value\n# 注释保留\nRequestHeader unset Accept-Encoding"
	must.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://backend:8080", Custom: custom}}))
	cfg, err := ParseFile(filepath.Join(configDir, "site", "200-proxy.conf"))
	must.NoError(t, err)
	// 要落在 <Location> 里才只对该规则生效
	must.NotNil(t, cfg.FindOne("IfModule.Location.RequestHeader"))
	check.Equal(t, cfg.FindOne("IfModule.Location").Arg(0), "/api")

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

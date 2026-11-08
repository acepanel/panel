package nginx

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

// newConfigDir 创建带 site/shared 子目录的临时配置目录
func newConfigDir(t *testing.T) string {
	t.Helper()
	configDir := t.TempDir()
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared"), 0755))
	return configDir
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

func newPHPVhost(t *testing.T) (*PHPVhost, string) {
	t.Helper()
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	must.NotNil(t, vhost)
	return vhost, configDir
}

func newProxyVhost(t *testing.T) (*ProxyVhost, string) {
	t.Helper()
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	return vhost, configDir
}

func TestNewVhost(t *testing.T) {
	vhost, configDir := newPHPVhost(t)
	check.Equal(t, vhost.configDir, configDir)
	check.NotNil(t, vhost.cfg)
}

func TestEnable(t *testing.T) {
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

func TestServerName(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	names := []string{"example.com", "www.example.com", "api.example.com"}
	check.NoError(t, vhost.SetServerName(names))
	check.DeepEqual(t, vhost.ServerName(), names)
}

func TestServerNameEmpty(t *testing.T) {
	vhost, _ := newPHPVhost(t)
	check.NoError(t, vhost.SetServerName([]string{}))
	check.Empty(t, vhost.ServerName())
}

func TestRoot(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	root := "/var/www/html"
	check.NoError(t, vhost.SetRoot(root))
	check.Equal(t, vhost.Root(), root)
}

func TestIndex(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	index := []string{"index.html", "index.php", "default.html"}
	check.NoError(t, vhost.SetIndex(index))
	check.DeepEqual(t, vhost.Index(), index)
}

func TestIndexEmpty(t *testing.T) {
	vhost, _ := newPHPVhost(t)
	check.NoError(t, vhost.SetIndex([]string{}))
	check.Empty(t, vhost.Index())
}

func TestListen(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	listens := []types.Listen{
		{Address: "80"},
		{Address: "443", Args: []string{"ssl"}},
	}
	check.NoError(t, vhost.SetListen(listens))
	check.DeepEqual(t, vhost.Listen(), listens, cmpopts.EquateEmpty())
}

func TestListenWithHTTP3(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	listens := []types.Listen{
		{Address: "443", Args: []string{"quic"}},
	}
	check.NoError(t, vhost.SetListen(listens))
	check.DeepEqual(t, vhost.Listen(), listens, cmpopts.EquateEmpty())
}

func TestListenWithSSLAndQUIC(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	// 测试 ssl 和 quic 同时存在时，应该分成两行 listen 指令
	// 但读取时应该合并为一个 Listen 对象
	listens := []types.Listen{
		{Address: "80"},
		{Address: "443", Args: []string{"ssl", "quic"}},
	}
	check.NoError(t, vhost.SetListen(listens))

	// 保存后验证顺序
	check.NoError(t, vhost.Save())

	// 验证生成的配置中 ssl 和 quic 是分开的
	dump := Render(vhost.cfg)
	must.Contains(t, dump, "listen 80;")
	must.Contains(t, dump, "listen 443 ssl;")
	check.Contains(t, dump, "listen 443 quic;")
	// 确保没有 "listen 443 ssl quic;" 这样的行
	check.NotContains(t, dump, "listen 443 ssl quic;")

	// 验证顺序：80 应该在 443 前面
	check.Less(t, strings.Index(dump, "listen 80;"), strings.Index(dump, "listen 443"),
		check.Msgf("listen 80 应排在 listen 443 之前，实际导出：\n%s", dump))

	// 读取时两行 443 应合并回一个 Listen
	check.DeepEqual(t, vhost.Listen(), listens, cmpopts.EquateEmpty())
}

func TestSSL(t *testing.T) {
	vhost, _ := newPHPVhost(t)
	check.False(t, vhost.SSL())
	check.Nil(t, vhost.SSLConfig())
}

func TestSetSSLConfig(t *testing.T) {
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
	// 证书路径不回读，只回读协议与各开关，证书由 TestDumpWithSSL 覆盖
	check.DeepEqual(t, vhost.SSLConfig(), &types.SSLConfig{
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
		HSTS:      true,
		OCSP:      true,
	})
}

func TestSetSSLConfigNil(t *testing.T) {
	vhost, _ := newPHPVhost(t)
	check.Error(t, vhost.SetSSLConfig(nil))
}

func TestClearSSL(t *testing.T) {
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

func TestPHP(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.Equal(t, vhost.PHP(), uint(0))

	check.NoError(t, vhost.SetPHP(84))
	check.Equal(t, vhost.PHP(), uint(84))

	check.NoError(t, vhost.SetPHP(0))
	check.Equal(t, vhost.PHP(), uint(0))
}

func TestAccessLog(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	accessLog := "/var/log/nginx/access.log"
	check.NoError(t, vhost.SetAccessLog(accessLog))
	check.Equal(t, vhost.AccessLog(), accessLog)
}

func TestErrorLog(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	errorLog := "/var/log/nginx/error.log"
	check.NoError(t, vhost.SetErrorLog(errorLog))
	check.Equal(t, vhost.ErrorLog(), errorLog)
}

func TestIncludes(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	includes := []types.IncludeFile{
		{Path: "/etc/nginx/conf.d/ssl.conf"},
		{Path: "/etc/nginx/conf.d/php.conf"},
	}
	check.NoError(t, vhost.SetIncludes(includes))
	check.DeepEqual(t, vhost.Includes(), includes)
}

func TestBasicAuth(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.Nil(t, vhost.BasicAuth())

	auths := []types.BasicAuth{
		{Path: "/", UserFile: "/etc/nginx/htpasswd_0"},
		{Path: "/admin", UserFile: "/etc/nginx/htpasswd_1"},
	}
	check.NoError(t, vhost.SetBasicAuth(auths))
	check.DeepEqual(t, vhost.BasicAuth(), auths)

	// map 片段应包含目录正则与整站 default
	content := vhost.Config(AuthConfName, types.ScopeShared)
	check.Contains(t, content, `~^/admin(/.*)?$ "Restricted";`)
	check.Contains(t, content, `default "/etc/nginx/htpasswd_0";`)

	check.NoError(t, vhost.ClearBasicAuth())
	check.Nil(t, vhost.BasicAuth())
}

func TestBasicAuthDirOnly(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	// 仅目录规则时整站不认证
	auths := []types.BasicAuth{
		{Path: "/private", UserFile: "/etc/nginx/htpasswd_0"},
	}
	check.NoError(t, vhost.SetBasicAuth(auths))

	content := vhost.Config(AuthConfName, types.ScopeShared)
	check.Contains(t, content, "default off;")
	check.Contains(t, content, `default "";`)

	check.DeepEqual(t, vhost.BasicAuth(), auths)
}

func TestBasicAuthNestedDir(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	// 嵌套目录时更精确的路径优先命中（map 按声明顺序取首个匹配）
	check.NoError(t, vhost.SetBasicAuth([]types.BasicAuth{
		{Path: "/admin", UserFile: "/etc/nginx/htpasswd_0"},
		{Path: "/admin/sub", UserFile: "/etc/nginx/htpasswd_1"},
	}))
	check.DeepEqual(t, vhost.BasicAuth(), []types.BasicAuth{
		{Path: "/admin/sub", UserFile: "/etc/nginx/htpasswd_1"},
		{Path: "/admin", UserFile: "/etc/nginx/htpasswd_0"},
	})
}

func TestRateLimit(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.Nil(t, vhost.RateLimit())

	limit := &types.RateLimit{
		PerServer: 300,
		PerIP:     25,
		Rate:      512,
	}
	check.NoError(t, vhost.SetRateLimit(limit))
	check.DeepEqual(t, vhost.RateLimit(), limit)

	check.NoError(t, vhost.ClearRateLimit())
	check.Nil(t, vhost.RateLimit())
}

func TestReset(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	// 重置后应完全回到初始模板，而不只是丢掉改动值
	names, root := vhost.ServerName(), vhost.Root()
	check.NoError(t, vhost.SetServerName([]string{"modified.com"}))
	check.NoError(t, vhost.SetRoot("/modified/path"))

	check.NoError(t, vhost.Reset())
	check.DeepEqual(t, vhost.ServerName(), names)
	check.Equal(t, vhost.Root(), root)
}

func TestSave(t *testing.T) {
	vhost, configDir := newPHPVhost(t)
	configFile := filepath.Join(configDir, "nginx.conf")

	check.NoError(t, vhost.SetServerName([]string{"save-test.com"}))
	check.NoError(t, vhost.Save())

	// 验证配置文件已保存
	content, err := os.ReadFile(configFile)
	must.NoError(t, err)
	check.Contains(t, string(content), "save-test.com")
}

func TestDump(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	check.NoError(t, vhost.SetServerName([]string{"dump-test.com"}))
	check.NoError(t, vhost.SetRoot("/var/www/dump-test"))

	content := Render(vhost.cfg)
	check.Contains(t, content, "dump-test.com")
	check.Contains(t, content, "/var/www/dump-test")
	check.Contains(t, content, "server {")
}

func TestDumpWithSSL(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	sslConfig := &types.SSLConfig{
		Cert:      "/etc/ssl/cert.pem",
		Key:       "/etc/ssl/key.pem",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	content := Render(vhost.cfg)
	check.Contains(t, content, "ssl_certificate")
	check.Contains(t, content, "ssl_certificate_key")
}

func TestHTTPSRedirect(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	sslConfig := &types.SSLConfig{
		Cert:         "/etc/ssl/cert.pem",
		Key:          "/etc/ssl/key.pem",
		HTTPRedirect: true,
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	got := vhost.SSLConfig()
	must.NotNil(t, got)
	check.True(t, got.HTTPRedirect)
}

func TestAltSvc(t *testing.T) {
	vhost, _ := newPHPVhost(t)

	sslConfig := &types.SSLConfig{
		Cert:   "/etc/ssl/cert.pem",
		Key:    "/etc/ssl/key.pem",
		AltSvc: `h3=":$server_port"; ma=2592000`,
	}
	check.NoError(t, vhost.SetSSLConfig(sslConfig))

	got := vhost.SSLConfig()
	must.NotNil(t, got)
	// 含引号与分号的头值应原样回读
	check.Equal(t, got.AltSvc, sslConfig.AltSvc)
}

func TestDefaultConfIncludesServerD(t *testing.T) {
	// 验证默认配置包含 site 的 include
	check.Contains(t, DefaultConf, "site")
	check.Contains(t, DefaultConf, "include")
}

func TestRedirects(t *testing.T) {
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

func TestRedirectURL(t *testing.T) {
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

	check.Contains(t, string(content), "location = /old-page")
	check.Contains(t, string(content), "return 301")
	check.Contains(t, string(content), "/new-page")
}

func TestRedirectHost(t *testing.T) {
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

	check.Contains(t, string(content), "$host")
	check.Contains(t, string(content), "old.example.com")
	check.Contains(t, string(content), "return 308")
	check.Contains(t, string(content), "$request_uri")
}

func TestRedirect404(t *testing.T) {
	vhost, configDir := newPHPVhost(t)

	redirects := []types.Redirect{
		{
			Type:       types.RedirectType404,
			To:         "/custom-404.html",
			StatusCode: 308,
		},
	}
	check.NoError(t, vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	must.NoError(t, err)

	check.Contains(t, string(content), "error_page 404")
	check.Contains(t, string(content), "@redirect_404")
}

func TestProxies(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

	// 初始应该没有代理配置
	check.Empty(t, vhost.Proxies())

	// 设置代理配置
	proxies := []types.Proxy{
		{
			Location: "/",
			Pass:     "http://backend",
			Host:     "example.com",
		},
		{
			Location:  "/api",
			Pass:      "http://api-backend:8080",
			Buffering: true,
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 验证代理文件已创建
	check.Len(t, confFiles(t, configDir, "site", "2", "-proxy.conf"), 2)

	// 未指定的 Host 与 HTTP 版本由 nginx 侧补默认值
	check.DeepEqual(t, vhost.Proxies(), []types.Proxy{
		{Location: "/", Pass: "http://backend", Host: "example.com", HTTPVersion: "1.1"},
		{Location: "/api", Pass: "http://api-backend:8080", Host: "$proxy_host", Buffering: true, HTTPVersion: "1.1"},
	}, cmpopts.EquateEmpty())
}

func TestProxyConfig(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

	proxies := []types.Proxy{
		{
			Location:  "/",
			Pass:      "https://backend",
			Host:      "example.com",
			SNI:       "example.com",
			Buffering: true,
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 读取配置文件内容
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	must.NoError(t, err)

	check.Contains(t, string(content), "location /")
	check.Contains(t, string(content), "proxy_pass https://backend")
	check.Contains(t, string(content), "proxy_set_header Host")
	check.Contains(t, string(content), "example.com")
	check.Contains(t, string(content), "proxy_ssl_name")
	check.Contains(t, string(content), "proxy_buffering on")
}

func TestClearProxies(t *testing.T) {
	vhost, _ := newProxyVhost(t)

	proxies := []types.Proxy{
		{Location: "/", Pass: "http://backend"},
	}
	check.NoError(t, vhost.SetProxies(proxies))
	check.Len(t, vhost.Proxies(), 1)

	check.NoError(t, vhost.ClearProxies())
	check.Empty(t, vhost.Proxies())
}

func TestUpstreams(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

	// 初始应该没有上游服务器配置
	check.Empty(t, vhost.Upstreams())

	// 设置上游服务器
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

	// 验证 upstream 文件已创建
	check.NotEmpty(t, confFiles(t, configDir, "shared", "", ""))

	check.DeepEqual(t, vhost.Upstreams(), upstreams, cmpopts.EquateEmpty())
}

func TestUpstreamConfig(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

	upstreams := []types.Upstream{
		{
			Name: "mybackend",
			Servers: map[string]string{
				"127.0.0.1:8080": "weight=5",
			},
			Algo:      "ip_hash",
			Keepalive: 16,
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))

	// 读取配置文件内容
	files := confFiles(t, configDir, "shared", "", "")
	must.NotEmpty(t, files)
	content, err := os.ReadFile(filepath.Join(configDir, "shared", files[0]))
	must.NoError(t, err)

	check.Contains(t, string(content), "upstream mybackend")
	check.Contains(t, string(content), "ip_hash")
	check.Contains(t, string(content), "server 127.0.0.1:8080")
	check.Contains(t, string(content), "weight=5")
	check.Contains(t, string(content), "keepalive 16")
}

func TestClearUpstreams(t *testing.T) {
	vhost, _ := newProxyVhost(t)

	upstreams := []types.Upstream{
		{
			Name:    "backend",
			Servers: map[string]string{"127.0.0.1:8080": ""},
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))
	check.Len(t, vhost.Upstreams(), 1)

	check.NoError(t, vhost.ClearUpstreams())
	check.Empty(t, vhost.Upstreams())
}

func TestProxyWithUpstream(t *testing.T) {
	vhost, configDir := newProxyVhost(t)

	// 先创建 upstream
	upstreams := []types.Upstream{
		{
			Name: "api-servers",
			Servers: map[string]string{
				"127.0.0.1:3000": "",
				"127.0.0.1:3001": "",
			},
			Algo: "least_conn",
		},
	}
	check.NoError(t, vhost.SetUpstreams(upstreams))

	// 然后创建引用 upstream 的 proxy
	proxies := []types.Proxy{
		{
			Location: "/api",
			Pass:     "http://api-servers",
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))

	// 验证两者都存在
	check.Len(t, vhost.Upstreams(), 1)
	check.Len(t, vhost.Proxies(), 1)

	// 验证 proxy 配置中引用了 upstream
	siteDir := filepath.Join(configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	must.NoError(t, err)
	check.Contains(t, string(content), "http://api-servers")
}

func TestDefaultWithSSLAndQUIC(t *testing.T) {
	vhost, configDir := newPHPVhost(t)
	must.NoError(t, vhost.SetListen([]types.Listen{
		{Address: "80"},
		{Address: "443", Args: []string{"ssl", "quic"}},
		{Address: "[::]:443", Args: []string{"ssl", "quic"}},
	}))
	for _, enabled := range []bool{true, false, true} {
		must.NoError(t, vhost.SetDefault(enabled))
		must.NoError(t, vhost.Save())
		var err error
		vhost, err = NewPHPVhost(configDir)
		must.NoError(t, err)
		must.Equal(t, vhost.Default(), enabled)

		dump := Render(vhost.cfg)
		for _, listen := range []string{"80", "443 ssl", "443 quic", "[::]:443 ssl", "[::]:443 quic"} {
			want := "listen " + listen
			if enabled {
				want += " default_server"
			}
			check.Contains(t, dump, want+";", check.Msgf("默认站点 %v 后的配置：\n%s", enabled, dump))
		}
	}
}

// 静态与 PHP 站点也能把子路径交给后端
func TestStaticProxies(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "^~ /api", Pass: "http://127.0.0.1:3000"}}))
	check.Len(t, confFiles(t, configDir, "site", "200", "-proxy.conf"), 1)
	check.Len(t, vhost.Proxies(), 1)
	check.Equal(t, vhost.Proxies()[0].Location, "^~ /api")

	php, _ := newPHPVhost(t)
	check.NoError(t, php.SetUpstreams([]types.Upstream{{Name: "api", Servers: map[string]string{"127.0.0.1:3000": ""}}}))
	check.NoError(t, php.SetProxies([]types.Proxy{{Location: "^~ /api", Pass: "http://api"}}))
	check.Len(t, php.Upstreams(), 1)
	check.Len(t, php.Proxies(), 1)
}

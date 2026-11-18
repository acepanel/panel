package openlitespeed

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	content, err := os.ReadFile(filepath.Join(configDir, vhostConfName))
	must.NoError(t, err)
	return string(content)
}

func TestDefaults(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newPHPVhost(configDir)
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
	vhost, err := newPHPVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl", "quic"}}}))
	check.NoError(t, vhost.SetServerName([]string{"example.com", "www.example.com"}))
	check.NoError(t, vhost.SetRoot("/var/www/html"))
	check.NoError(t, vhost.SetIndex([]string{"index.php", "index.html"}))
	check.NoError(t, vhost.SetAccessLog("/var/log/access.log"))
	check.NoError(t, vhost.SetErrorLog("/var/log/error.log"))
	check.NoError(t, vhost.SetPHP(84))
	// 手写的 include 行重新生成时要保住
	vhost.Includes = []string{"/etc/custom.conf"}
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "docRoot                  /var/www/html")
	check.Contains(t, conf, "indexFiles               index.php, index.html")
	check.Contains(t, conf, "accesslog /var/log/access.log {")
	check.Contains(t, conf, "errorlog /var/log/error.log {")
	check.Contains(t, conf, "include                  "+phpHandlerFile(84))
	check.Contains(t, conf, "include                  /etc/custom.conf")

	reloaded, err := newPHPVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Listen(), []types.Listen{{Address: "80", Args: []string{}}, {Address: "443", Args: []string{"ssl", "quic"}}})
	check.DeepEqual(t, reloaded.ServerName(), []string{"example.com", "www.example.com"})
	check.Equal(t, reloaded.Root(), "/var/www/html")
	check.DeepEqual(t, reloaded.Index(), []string{"index.php", "index.html"})
	check.Equal(t, reloaded.AccessLog(), "/var/log/access.log")
	check.Equal(t, reloaded.ErrorLog(), "/var/log/error.log")
	check.Equal(t, reloaded.PHP(), uint(84))
	check.DeepEqual(t, reloaded.Includes, []string{"/etc/custom.conf"})

	// 监听记录
	listen, err := os.ReadFile(filepath.Join(configDir, listenConfName))
	must.NoError(t, err)
	check.Contains(t, string(listen), "listen                   443 ssl quic")
	check.Contains(t, string(listen), "domain                   example.com www.example.com")
}

func TestEnable(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newStaticVhost(configDir)
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

func TestDefault(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newPHPVhost(configDir)
	must.NoError(t, err)
	check.False(t, vhost.Default())
	check.NoError(t, vhost.SetDefault(true))
	check.NoError(t, vhost.Save())

	// 通配域名排在最后，监听器汇总靠它判断是否还要默认站点兜底
	listen, err := os.ReadFile(filepath.Join(configDir, listenConfName))
	must.NoError(t, err)
	check.Contains(t, string(listen), "domain                   localhost *")

	reloaded, err := newPHPVhost(configDir)
	must.NoError(t, err)
	check.True(t, reloaded.Default())
	check.NoError(t, reloaded.SetDefault(false))
	check.False(t, reloaded.Default())
	check.DeepEqual(t, reloaded.ServerName(), []string{"localhost"})
}

func TestSSL(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newStaticVhost(configDir)
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
	check.Contains(t, conf, "https://%{HTTP_HOST}%{REQUEST_URI} [R=308,L]")

	reloaded, err := newStaticVhost(configDir)
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
	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	// 权重、算法、连接保持数 OLS 都表达不了，不写进配置也不回读
	check.NoError(t, vhost.SetUpstreams([]types.Upstream{{
		Name:      "backend",
		Servers:   map[string]string{"10.0.0.1:8080": "weight=5", "10.0.0.2:8080": ""},
		Algo:      "least_conn",
		Keepalive: 32,
	}}))
	check.NoError(t, vhost.SetProxies([]types.Proxy{
		{
			Location:  "^~ /",
			Pass:      "http://backend/v2/",
			Host:      "example.com",
			Buffering: true,
			Headers:   map[string]string{"X-Custom": "1"},
		},
		{
			Location:        "~ ^/api/v[0-9]+/",
			Pass:            "https://api.example.com/ignored/",
			Headers:         map[string]string{"X-Custom": "1", "X-Real-IP": "$remote_addr"},
			ResponseHeaders: &types.ResponseHeaderConfig{Add: map[string]string{"X-Cache": "HIT", "X-Scheme": "$scheme"}, Hide: []string{"X-Powered-By"}},
			AccessControl:   &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
			SNI:             "sni.example.com",
			Timeout:         &types.TimeoutConfig{Connect: 5 * time.Second, Read: 90 * time.Second, Send: 30 * time.Second},
		},
		{
			Location:      "/ws",
			Pass:          "http://127.0.0.1:3000",
			AccessControl: &types.AccessControlConfig{Deny: []string{"all"}},
		},
		{
			Location: "/passthrough",
			Pass:     "http://127.0.0.1:4000",
			Host:     "$host",
		},
	}))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "workers                  proxy::")
	// 上游要用负载均衡上下文，写成 proxy 类型 OLS 找不到处理器
	check.Contains(t, conf, "context / {\n  type                     loadbalancer\n")
	check.Contains(t, conf, "address                  https://api.example.com:443")
	check.Contains(t, conf, "context exp:^/api/v[0-9]+/ {")
	// 改 Host 靠上下文里的转发规则，RequestHeader 改 Host 会让后端收到两个 Host 头
	check.NotContains(t, conf, "RequestHeader set Host")
	check.Contains(t, conf, "RewriteRule              ^ https://"+vhost.safeName+"_proxy_1%{ORG_REQ_URI} [P,L,NE,E=Proxy-Host:api.example.com]")
	check.Contains(t, conf, "RewriteRule              ^ http://"+vhost.safeName+"_proxy_2%{ORG_REQ_URI} [P,L,NE,E=Proxy-Host:127.0.0.1:3000]")
	check.Contains(t, conf, "context /passthrough/ {\n  type                     proxy\n")
	// 头部的值按字面量发出，带变量的不写；负载均衡上下文不执行 RequestHeader，一条都不写
	check.Equal(t, strings.Count(conf, "RequestHeader set X-Custom 1"), 1)
	check.NotContains(t, conf, "X-Real-IP")
	check.NotContains(t, conf, "X-Scheme")
	check.Contains(t, conf, "Header set X-Cache HIT")
	check.Contains(t, conf, "Header unset X-Powered-By")
	// 只填允许列表时其余地址照样放行，要补上全部拒绝才是白名单
	check.Contains(t, conf, "    allow                    10.0.0.0/8\n    deny                     10.0.0.99, ALL\n")
	check.Contains(t, conf, "    deny                     ALL\n")
	// initTimeout 当不了读超时，不随规则变
	check.NotContains(t, conf, "initTimeout              90")
	check.NotContains(t, conf, "ace:sni")
	check.NotContains(t, conf, "ace:options")
	check.NotContains(t, conf, "ace:algo")
	check.NotContains(t, conf, "ace:keepalive")
	check.Contains(t, conf, "websocket /ws/ {")
	check.Contains(t, conf, "address                  127.0.0.1:3000")
	// 子路径反代跳过后续 rewrite，根规则不拦
	check.Contains(t, conf, "RewriteRule              ^/api/v[0-9]+/ - [L]")
	check.Contains(t, conf, "RewriteRule              ^/ws/ - [L]")
	check.NotContains(t, conf, "^/ - [L]")

	reloaded, err := newProxyVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Upstreams(), []types.Upstream{{
		Name:    "backend",
		Servers: map[string]string{"10.0.0.1:8080": "", "10.0.0.2:8080": ""},
	}}, cmpopts.EquateEmpty())

	// 回读到的是实际生效的配置：负载均衡只会原样转发，Host、路径、请求头与缓冲都没有；正则规则的目标带不了路径；
	// 未设置的 Host 是 nginx 的默认变量，HTTP 版本固定 1.1
	check.DeepEqual(t, reloaded.Proxies(), []types.Proxy{
		{Location: "^~ /", Pass: "http://backend", Host: "$host", HTTPVersion: "1.1"},
		{
			Location:        "~ ^/api/v[0-9]+/",
			Pass:            "https://api.example.com",
			Host:            "$proxy_host",
			HTTPVersion:     "1.1",
			Headers:         map[string]string{"X-Custom": "1"},
			ResponseHeaders: &types.ResponseHeaderConfig{Add: map[string]string{"X-Cache": "HIT"}, Hide: []string{"X-Powered-By"}},
			AccessControl:   &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
		},
		{
			Location: "/ws", Pass: "http://127.0.0.1:3000", Host: "$proxy_host", HTTPVersion: "1.1",
			AccessControl: &types.AccessControlConfig{Deny: []string{"all"}},
		},
		{Location: "/passthrough", Pass: "http://127.0.0.1:4000", Host: "$host", HTTPVersion: "1.1"},
	}, cmpopts.EquateEmpty())
}

func TestRedirects(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newStaticVhost(configDir)
	must.NoError(t, err)
	redirects := []types.Redirect{
		{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 301},
		{Type: types.RedirectTypeURL, From: "/docs", To: "https://docs.example.com", KeepURI: true, StatusCode: 302},
		{Type: types.RedirectTypeHost, From: "old.example.com", To: "https://example.com", KeepURI: true, StatusCode: 301},
		{Type: types.RedirectType404, To: "/404.html", StatusCode: 308},
		{Type: types.RedirectTypeHost, From: "old2.example.com", To: "https://example.com", StatusCode: 302},
		// 目标里的 % 与 $ 是字面量，不能被当成反向引用
		{Type: types.RedirectTypeURL, From: "/enc", To: "https://example.com/a%20b/$1", StatusCode: 302},
		{Type: types.RedirectTypeHost, From: "old3.example.com", To: "https://example.com/%E4%B8%AD", KeepURI: true, StatusCode: 302},
	}
	check.NoError(t, vhost.SetRedirects(redirects))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, `RewriteRule              ^/enc$ https://example.com/a\%20b/\$1 [R=302,L,QSD]`)
	check.Contains(t, conf, `RewriteRule              ^(.*)$ https://example.com/\%E4\%B8\%AD$1 [R=302,L]`)
	// 只匹配这一个路径；保留 URI 时接上原始路径，不保留时连查询串一起丢掉
	check.Contains(t, conf, "RewriteRule              ^/old$ /new [R=301,L,QSD]")
	check.Contains(t, conf, "RewriteRule              ^/docs$ https://docs.example.com%{REQUEST_URI} [R=302,L]")
	check.Contains(t, conf, `RewriteCond              %{HTTP_HOST} ^old\.example\.com$ [NC]`)
	check.Contains(t, conf, "RewriteRule              ^(.*)$ https://example.com$1 [R=301,L]")
	check.Contains(t, conf, "RewriteRule              ^(.*)$ https://example.com [R=302,L,QSD]")
	check.Contains(t, conf, "errorpage 404 {")
	check.NotContains(t, conf, "type                     redirect")

	reloaded, err := newStaticVhost(configDir)
	must.NoError(t, err)
	// 重定向分散在不同的块里，回读靠序号还原用户定义的顺序
	check.DeepEqual(t, reloaded.Redirects(), redirects)
}

// 旧版把 URL 重定向写成 redirect 上下文，主机名重定向没有序号
func TestLegacyRedirects(t *testing.T) {
	configDir := newConfigDir(t)
	legacy := `docRoot /var/www/html

context exp:^/old$ {
  type                     redirect
  externalRedirect         1
  statusCode               301
  location                 /new
}

context exp:^/docs(.*)$ {
  type                     redirect
  externalRedirect         1
  statusCode               302
  location                 https://docs.example.com$1
}

errorpage 404 {
  url                      /404.html
}

rewrite {
  enable                   1
  RewriteCond              %{HTTPS} !on
  RewriteRule              ^(.*)$ https://%{HTTP_HOST}%{REQUEST_URI} [R=308,L]
  RewriteCond              %{HTTP_HOST} ^old\.example\.com$ [NC]
  RewriteRule              ^(.*)$ https://example.com$1 [R=301,L]
}
`
	must.NoError(t, os.WriteFile(filepath.Join(configDir, vhostConfName), []byte(legacy), 0600))
	vhost, err := newStaticVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, vhost.Redirects(), []types.Redirect{
		{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 301},
		{Type: types.RedirectTypeURL, From: "/docs", To: "https://docs.example.com", KeepURI: true, StatusCode: 302},
		{Type: types.RedirectType404, To: "/404.html", StatusCode: 308},
		{Type: types.RedirectTypeHost, From: "old.example.com", To: "https://example.com", KeepURI: true, StatusCode: 301},
	})
}

// 目标写成 https://上游名 时成员走 TLS，协议记在成员地址里
func TestUpstreamTLS(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	upstreams := []types.Upstream{
		{Name: "secure", Servers: map[string]string{"10.0.0.1:8443": "", "unix:/tmp/a.sock": ""}},
		{Name: "plain", Servers: map[string]string{"10.0.0.2:8080": ""}},
	}
	must.NoError(t, vhost.SetUpstreams(upstreams))
	must.NoError(t, vhost.SetProxies([]types.Proxy{
		{Location: "/", Pass: "https://secure"},
		{Location: "/plain", Pass: "http://plain"},
	}))
	must.NoError(t, vhost.Save())
	conf := readConf(t, configDir)
	check.Contains(t, conf, "address                  https://10.0.0.1:8443")
	check.Contains(t, conf, "address                  uds://tmp/a.sock")
	check.Contains(t, conf, "address                  10.0.0.2:8080")

	reloaded, err := newProxyVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Upstreams(), upstreams, cmpopts.EquateEmpty())
}

// OLS 原样转发 Host 头，$host 与 $http_host 都不用改写；其余写法靠转发规则的 Proxy-Host，变量换成重写规则认的写法
func TestProxyHost(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.SetProxies([]types.Proxy{
		{Location: "/a", Pass: "http://127.0.0.1:3000", Host: "$http_host"},
		{Location: "/b", Pass: "http://127.0.0.1:3000", Host: "example.com"},
		{Location: "/c", Pass: "http://127.0.0.1:3000", Host: "$host:8080"},
		{Location: "/d", Pass: "http://127.0.0.1:3000", Host: "$proxy_host"},
	}))
	must.NoError(t, vhost.Save())
	conf := readConf(t, configDir)
	check.Contains(t, conf, "context /a/ {\n  type                     proxy\n")
	check.Contains(t, conf, "E=Proxy-Host:example.com]")
	check.Contains(t, conf, "E=Proxy-Host:%{HTTP_HOST}:8080]")
	// 同 nginx 的 $proxy_host，目标里写了端口就带上
	check.Contains(t, conf, "E=Proxy-Host:127.0.0.1:3000]")
	check.NotContains(t, conf, "RequestHeader")

	reloaded, err := newProxyVhost(configDir)
	must.NoError(t, err)
	got := reloaded.Proxies()
	must.Len(t, got, 4)
	check.Equal(t, got[0].Host, "$host")
	check.Equal(t, got[1].Host, "example.com")
	check.Equal(t, got[2].Host, "$host:8080")
	check.Equal(t, got[3].Host, "$proxy_host")
}

// 旧版用 RequestHeader 改 Host，回读后按新写法保存
func TestLegacyProxyHost(t *testing.T) {
	configDir := newConfigDir(t)
	legacy := `docRoot /var/www/html

extprocessor x_proxy_0 {
  type                     proxy
  address                  127.0.0.1:3000
  respBuffer               1
}

context /api/ {
  type                     proxy
  handler                  x_proxy_0
  # ace:location /api
  # ace:pass http://127.0.0.1:3000
  extraHeaders <<<END_extraHeaders
RequestHeader set Host example.com
RequestHeader set X-Custom 1
END_extraHeaders
}
`
	must.NoError(t, os.WriteFile(filepath.Join(configDir, vhostConfName), []byte(legacy), 0600))
	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, vhost.Proxies(), []types.Proxy{{
		Location: "/api", Pass: "http://127.0.0.1:3000", Host: "example.com", HTTPVersion: "1.1", Buffering: true,
		Headers: map[string]string{"X-Custom": "1"},
	}}, cmpopts.EquateEmpty())
}

// 目标带路径时同 nginx 换掉匹配到的前缀，上下文里的规则匹配的是去掉上下文 uri 之后的部分
func TestProxyPassPath(t *testing.T) {
	cases := []struct {
		location, pass, rule string
	}{
		{"/api/", "http://127.0.0.1:3000/v2/", "^(.*)$ http://%s/v2/$1 [P,L,NE]"},
		{"/api", "http://127.0.0.1:3000/v2", "^(.*)$ http://%s/v2/$1 [P,L,NE]"},
		{"^~ /api", "http://127.0.0.1:3000/v2/", "^(.*)$ http://%s/v2//$1 [P,L,NE]"},
		{"/", "https://127.0.0.1:3000/v2/", "^(.*)$ https://%s/v2/$1 [P,L,NE]"},
		{"= /health", "http://127.0.0.1:3000/status", "^ http://%s/status [P,L,NE]"},
	}
	for _, c := range cases {
		t.Run(c.location+" "+c.pass, func(t *testing.T) {
			configDir := newConfigDir(t)
			vhost, err := newProxyVhost(configDir)
			must.NoError(t, err)
			must.NoError(t, vhost.SetProxies([]types.Proxy{{Location: c.location, Pass: c.pass, Host: "$host"}}))
			must.NoError(t, vhost.Save())
			conf := readConf(t, configDir)
			check.Contains(t, conf, "RewriteRule              "+fmt.Sprintf(c.rule, vhost.safeName+"_proxy_0")+"\n")
			// 路径被改写的规则接不上 WebSocket
			check.NotContains(t, conf, "websocket")

			reloaded, err := newProxyVhost(configDir)
			must.NoError(t, err)
			must.Len(t, reloaded.Proxies(), 1)
			check.Equal(t, reloaded.Proxies()[0].Pass, c.pass)
			check.Equal(t, reloaded.Proxies()[0].Host, "$host")
		})
	}

	// 换成自己等于没换，仍是原样转发
	configDir := newConfigDir(t)
	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/", Pass: "http://127.0.0.1:3000/", Host: "$host"}}))
	must.NoError(t, vhost.Save())
	conf := readConf(t, configDir)
	check.Contains(t, conf, "context / {\n  type                     proxy\n")
	check.Contains(t, conf, "websocket / {")
}

func TestBasicAuth(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newProxyVhost(configDir)
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

	reloaded, err := newProxyVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.BasicAuth(), auths,
		cmpopts.SortSlices(func(a, b types.BasicAuth) bool { return a.Path < b.Path }))

	check.NoError(t, reloaded.SetBasicAuth(nil))
	check.NoError(t, reloaded.Save())
	check.NotContains(t, readConf(t, configDir), "realm")
}

func TestRootContext(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newPHPVhost(configDir)
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

	reloaded, err := newPHPVhost(configDir)
	must.NoError(t, err)
	sslCfg := reloaded.SSLConfig()
	must.NotNil(t, sslCfg)
	check.True(t, sslCfg.HSTS)
	check.DeepEqual(t, reloaded.BasicAuth(), []types.BasicAuth{{Path: "/", UserFile: "/opt/ace/sites/x/htpasswd_0"}})
}

func TestFragments(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newPHPVhost(configDir)
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

func TestRewrite(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newPHPVhost(configDir)
	must.NoError(t, err)
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

func TestLocationToURI(t *testing.T) {
	cases := map[string]string{
		"^~ /":         "/",
		"/":            "/",
		"/api":         "/api/",
		"^~ /static/":  "/static/",
		"~ ^/v1/":      "exp:^/v1/",
		"~* \\.(jpg)$": `exp:(?i)\.(jpg)$`,
		"= /exact":     "exp:^/exact$",
		"= /a.b/":      `exp:^/a\.b/$`,
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

// 代理规则里面的认证路径要另起上下文，转发方式、头部与访问控制都照搬那条规则，静态上下文会让该路径去磁盘找文件而 404
func TestProxyAuthContext(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.SetProxies([]types.Proxy{
		{Location: "/", Pass: "http://127.0.0.1:8080", Host: "$host"},
		{
			Location: "/api", Pass: "http://127.0.0.1:8081/v2/", Host: "api.example.com",
			ResponseHeaders: &types.ResponseHeaderConfig{Add: map[string]string{"X-Api": "1"}},
			AccessControl:   &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}},
		},
	}))
	auths := []types.BasicAuth{
		{Path: "/secret", UserFile: filepath.Join(configDir, "htpasswd_0")},
		{Path: "/api/admin", UserFile: filepath.Join(configDir, "htpasswd_1")},
	}
	must.NoError(t, vhost.SetBasicAuth(auths))
	must.NoError(t, vhost.Save())

	block := func(uri string) string {
		_, body, ok := strings.Cut(readConf(t, configDir), "context "+uri+" {")
		must.True(t, ok)
		body, _, _ = strings.Cut(body, "\n}")
		return body
	}
	secret := block("/secret/")
	check.Contains(t, secret, "type                     proxy")
	check.Contains(t, secret, "realm                    "+vhost.realmName(0))
	check.NotContains(t, secret, "$DOC_ROOT")
	check.Contains(t, readConf(t, configDir), "websocket /secret/ {")

	admin := block("/api/admin/")
	check.Contains(t, admin, "RewriteRule              ^(.*)$ http://"+vhost.safeName+"_proxy_1/v2//admin/$1 [P,L,NE,E=Proxy-Host:api.example.com]")
	check.Contains(t, admin, "Header set X-Api 1")
	check.Contains(t, admin, "deny                     ALL")
	check.Contains(t, admin, "realm                    "+vhost.realmName(1))

	// 认证上下文不能被回读成反代规则
	reloaded, err := newProxyVhost(configDir)
	must.NoError(t, err)
	check.Len(t, reloaded.Proxies(), 2)
	check.DeepEqual(t, reloaded.BasicAuth(), auths)
}

// OLS 的认证挂在上下文上，整站认证要落到每个上下文，否则子路径反代就绕过了认证
func TestAuthCoversContexts(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newStaticVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.SetProxies([]types.Proxy{
		{Location: "^~ /api", Pass: "http://127.0.0.1:3000", Host: "$host"},
		{Location: "~ \\.json$", Pass: "http://127.0.0.1:3001", Host: "$host"},
		{Location: "= /private/health", Pass: "http://127.0.0.1:3002", Host: "$host"},
	}))
	auths := []types.BasicAuth{
		{Path: "/", UserFile: "/opt/ace/sites/x/htpasswd_0"},
		{Path: "/private", UserFile: "/opt/ace/sites/x/htpasswd_1"},
	}
	must.NoError(t, vhost.SetBasicAuth(auths))
	must.NoError(t, vhost.Save())

	cfg, err := Parse(readConf(t, configDir))
	must.NoError(t, err)
	realms := make(map[string]string)
	for _, ctx := range cfg.Blocks("context") {
		realms[ctx.Arg(0)] = ctx.Value("realm")
	}
	check.DeepEqual(t, realms, map[string]string{
		"/":                     vhost.realmName(0),
		"/api/":                 vhost.realmName(0),
		`exp:\.json$`:           vhost.realmName(0),
		`exp:^/private/health$`: vhost.realmName(1),
		"/private/":             vhost.realmName(1),
	})

	reloaded, err := newStaticVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.BasicAuth(), auths)
	check.Len(t, reloaded.Proxies(), 3)
}

// autoLoadHtaccess 只在 vhost 级生效，写进 context 会被 OLS 静默忽略
func TestHtaccessAtVhostLevel(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newPHPVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.Save())

	cfg, err := Parse(readConf(t, configDir))
	must.NoError(t, err)
	check.Equal(t, cfg.GetBlock("rewrite").Value("autoLoadHtaccess"), "1")
	check.Empty(t, cfg.GetBlock("context", "/").GetBlock("rewrite").Value("autoLoadHtaccess"))
}

func TestProxyCustom(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newProxyVhost(configDir)
	must.NoError(t, err)
	custom := "addDefaultCharset        off\n# 注释保留\nenableIpGeo              1"
	check.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://127.0.0.1:8080", Custom: custom}}))
	check.NoError(t, vhost.Save())
	// 要落在该规则的 context 里才只对该规则生效
	check.Contains(t, readConf(t, configDir), "  # ace:custom\n  addDefaultCharset        off\n")

	reloaded, err := newProxyVhost(configDir)
	must.NoError(t, err)
	got := reloaded.Proxies()
	must.Len(t, got, 1)
	check.Equal(t, got[0].Custom, custom)

	check.Error(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://127.0.0.1:8080", Custom: "rewrite {"}}))
}

func TestStaticProxyPassthrough(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := newStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetRawConfig("800-spa.conf", types.ScopeSite, Dialect{}.SPAConf()))
	check.NoError(t, vhost.SetProxies([]types.Proxy{
		{Location: "^~ /api", Pass: "http://127.0.0.1:3000"},
		{Location: "= /health", Pass: "http://127.0.0.1:3000"},
	}))
	check.NoError(t, vhost.Save())

	conf := readConf(t, configDir)
	check.Contains(t, conf, "context /api/ {")
	check.Contains(t, conf, "context / {")
	// 透传规则在 SPA 回退片段之前
	passthrough := strings.Index(conf, "RewriteRule              ^/api/ - [L]")
	exact := strings.Index(conf, "RewriteRule              ^/health$ - [L]")
	spa := strings.Index(conf, "include                  "+filepath.Join(configDir, "site", "800-spa.conf"))
	must.True(t, passthrough > 0 && exact > 0 && spa > 0)
	check.True(t, passthrough < spa && exact < spa)

	reloaded, err := newStaticVhost(configDir)
	must.NoError(t, err)
	check.Len(t, reloaded.Proxies(), 2)
}

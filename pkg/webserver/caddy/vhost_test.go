package caddy

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"golang.org/x/crypto/bcrypt"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

func newConfigDir(t *testing.T) string {
	t.Helper()
	configDir := filepath.Join(t.TempDir(), "config")
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	must.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared"), 0755))
	return configDir
}

// upstreamSnippet 上游片段名，站点名取自临时目录
func upstreamSnippet(configDir, name string) string {
	return "ace_upstream_" + safeName(filepath.Base(filepath.Dir(configDir))) + "_" + safeName(name)
}

func siteConf(t *testing.T, configDir string) string {
	t.Helper()
	return readFile(t, filepath.Join(configDir, ConfName))
}

func TestVhostDefaults(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.True(t, vhost.Enable())
	check.Equal(t, vhost.Root(), filepath.Join(filepath.Dir(configDir), "public"))
	check.DeepEqual(t, vhost.Index(), []string{"index.html"})
	check.DeepEqual(t, vhost.Listen(), []types.Listen{{Address: "80", Args: []string{}}})
	check.DeepEqual(t, vhost.ServerName(), []string{"localhost"})
	check.Equal(t, vhost.ErrorLog(), ErrorLogPath)
	check.False(t, vhost.SSL())
	check.Nil(t, vhost.SSLConfig())
	check.Equal(t, vhost.PHP(), uint(0))
}

func TestVhostBasicRoundTrip(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl"}}}))
	check.NoError(t, vhost.SetServerName([]string{"example.com", "www.example.com"}))
	check.NoError(t, vhost.SetRoot("/var/www/html"))
	check.NoError(t, vhost.SetIndex([]string{"index.php", "index.html"}))
	check.NoError(t, vhost.SetAccessLog("/var/log/access.log"))
	check.NoError(t, vhost.SetPHP(84))
	check.NoError(t, vhost.SetIncludes([]types.IncludeFile{{Path: "/etc/custom.conf"}}))
	check.NoError(t, vhost.Save())

	conf := siteConf(t, configDir)
	check.NotContains(t, conf, "shared/*.conf", "共享目录为空时不引用")
	site := "ace_site_" + safeName(filepath.Base(filepath.Dir(configDir)))
	check.Contains(t, conf, "\n("+site+") {\n")
	check.Contains(t, conf, "\nhttp://example.com:80,\nhttp://www.example.com:80 {\n\timport "+site+"\n}\n")
	check.Contains(t, conf, "\nhttps://example.com:443,\nhttps://www.example.com:443 {\n\timport "+site+"\n}\n")
	check.Contains(t, conf, "\troot * /var/www/html\n")
	check.Contains(t, conf, "\tencode br zstd gzip\n")
	check.Contains(t, conf, "\t\toutput file /var/log/access.log\n")
	check.Contains(t, conf, "\timport "+filepath.Join(configDir, "site", "*.conf")+"\n")
	check.Contains(t, conf, "\timport /etc/custom.conf\n")
	check.Contains(t, conf, "\tphp_fastcgi unix//tmp/php-cgi-84.sock\n")
	check.Contains(t, conf, "\t\tindex index.php index.html\n")
	check.Contains(t, conf, "handle "+acmeMatcher+" {")
	check.NotContains(t, conf, "bind")
	check.NotContains(t, conf, "tls ")

	reloaded, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Listen(), []types.Listen{{Address: "80", Args: []string{}}, {Address: "443", Args: []string{"ssl"}}})
	check.DeepEqual(t, reloaded.ServerName(), []string{"example.com", "www.example.com"})
	check.Equal(t, reloaded.Root(), "/var/www/html")
	check.DeepEqual(t, reloaded.Index(), []string{"index.php", "index.html"})
	check.Equal(t, reloaded.AccessLog(), "/var/log/access.log")
	check.Equal(t, reloaded.PHP(), uint(84))
	check.DeepEqual(t, reloaded.Includes(), []types.IncludeFile{{Path: "/etc/custom.conf"}})
	check.Nil(t, reloaded.SSLConfig())

	// 重新保存不改变内容
	check.NoError(t, reloaded.Save())
	check.Equal(t, siteConf(t, configDir), conf)
}

func TestVhostBind(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.Error(t, vhost.SetListen([]types.Listen{{Address: "80"}, {Address: "1.2.3.4:8080"}}))
	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "1.2.3.4:80"}, {Address: "[::1]:80"}, {Address: "1.2.3.4:8443", Args: []string{"ssl"}}}))
	check.NoError(t, vhost.SetServerName([]string{"example.com"}))
	check.NoError(t, vhost.SetSSLConfig(&types.SSLConfig{Cert: "/c.pem", Key: "/k.pem"}))
	check.NoError(t, vhost.Save())

	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\nhttp://example.com:80 {\n\tbind 1.2.3.4 ::1\n")
	check.Contains(t, conf, "\nhttps://example.com:8443 {\n\tbind 1.2.3.4 ::1\n\ttls /c.pem /k.pem\n")

	reloaded, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Listen(), []types.Listen{
		{Address: "1.2.3.4:80", Args: []string{}},
		{Address: "[::1]:80", Args: []string{}},
		{Address: "1.2.3.4:8443", Args: []string{"ssl"}},
		{Address: "[::1]:8443", Args: []string{"ssl"}},
	})
}

func TestVhostHostless(t *testing.T) {
	configDir := newConfigDir(t)
	// phpMyAdmin 这类无域名站点，安装脚本写的最简配置也能解析并重新生成
	must.NoError(t, os.WriteFile(filepath.Join(configDir, ConfName), []byte(":888 {\n\troot * /opt/pma\n\tphp_fastcgi unix//tmp/php-cgi-83.sock\n\tfile_server\n}\n"), 0600))
	vhost, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, vhost.Listen(), []types.Listen{{Address: "888", Args: []string{}}})
	check.DeepEqual(t, vhost.ServerName(), []string{})
	check.Equal(t, vhost.Root(), "/opt/pma")
	check.Equal(t, vhost.PHP(), uint(83))
	check.Equal(t, vhost.AccessLog(), "")

	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "8888"}}))
	check.NoError(t, vhost.Save())
	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\nhttp://:8888 {\n")
	check.NotContains(t, conf, "\tlog {")
	reloaded, err := NewPHPVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Listen(), []types.Listen{{Address: "8888", Args: []string{}}})
	check.DeepEqual(t, reloaded.ServerName(), []string{})
}

func TestVhostEnable(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetEnable(false))
	check.False(t, vhost.Enable())
	check.NoError(t, vhost.Save())
	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\t# ace:stop\n\t"+stopMatcher+" expression true\n\thandle "+stopMatcher+" {\n\t\trewrite * /stop.html\n\t\tfile_server {\n\t\t\troot "+HTMLDir+"\n\t\t}\n\t}\n")

	check.NoError(t, vhost.SetEnable(true))
	check.NoError(t, vhost.Save())
	check.NotContains(t, siteConf(t, configDir), stopMatcher)
}

func TestVhostSSL(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl"}}}))
	check.NoError(t, vhost.SetSSLConfig(&types.SSLConfig{
		Cert:         "/path/fullchain.pem",
		Key:          "/path/private.key",
		Protocols:    []string{"TLSv1.3"},
		HSTS:         true,
		OCSP:         false,
		HTTPRedirect: true,
	}))
	check.NoError(t, vhost.Save())

	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\ttls /path/fullchain.pem /path/private.key {\n\t\tprotocols tls1.3\n\t}\n")
	check.Contains(t, conf, "\nhttp://localhost:80 {\n\t"+httpMatcher+" not path /.well-known/acme-challenge/*\n\tredir "+httpMatcher+" https://{host}{uri} 301\n\timport ")
	check.Contains(t, conf, "\theader Strict-Transport-Security max-age=31536000\n\timport ")

	reloaded, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.True(t, reloaded.SSL())
	// OCSP 装订由 Caddy 对所有证书自动完成
	check.DeepEqual(t, reloaded.SSLConfig(), &types.SSLConfig{
		Cert:         "/path/fullchain.pem",
		Key:          "/path/private.key",
		Protocols:    []string{"TLSv1.3"},
		HSTS:         true,
		OCSP:         true,
		HTTPRedirect: true,
	})

	// Caddy 不支持 TLS 1.0/1.1，过滤后与默认相同则省略 protocols
	check.NoError(t, reloaded.SetSSLConfig(&types.SSLConfig{Cert: "/c", Key: "/k", Protocols: []string{"TLSv1.1", "TLSv1.2", "TLSv1.3"}}))
	check.NoError(t, reloaded.Save())
	conf = siteConf(t, configDir)
	check.Contains(t, conf, "\ttls /c /k\n")
	check.NotContains(t, conf, "protocols")
	again, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, again.SSLConfig(), &types.SSLConfig{
		Cert:      "/c",
		Key:       "/k",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
		OCSP:      true,
	})

	check.NoError(t, again.ClearSSL())
	check.NoError(t, again.Save())
	check.NotContains(t, siteConf(t, configDir), "tls")
}

func TestVhostBasicAuth(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	siteDir := filepath.Dir(configDir)
	file0, file1, file2 := filepath.Join(siteDir, "htpasswd_0"), filepath.Join(siteDir, "htpasswd_1"), filepath.Join(siteDir, "htpasswd_2")
	must.NoError(t, os.WriteFile(file0, []byte("admin:{PLAIN}secret\n# comment\nbob:plain\n"), 0644))
	must.NoError(t, os.WriteFile(file1, []byte("ops:{PLAIN}pw\n"), 0644))
	must.NoError(t, os.WriteFile(file2, []byte(""), 0644))
	check.NoError(t, vhost.SetBasicAuth([]types.BasicAuth{
		{Path: "/", UserFile: file0},
		{Path: "/admin", UserFile: file1},
		{Path: "/empty", UserFile: file2},
	}))
	check.NoError(t, vhost.Save())

	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\t@ace_auth_0 {\n\t\tpath /*\n\t\tnot path /.well-known/acme-challenge/*\n\t}\n\tbasic_auth @ace_auth_0 {\n\t\timport "+file0+".caddy\n\t}\n")
	check.Contains(t, conf, "\t@ace_auth_1 path /admin*\n\tbasic_auth @ace_auth_1 {\n\t\timport "+file1+".caddy\n\t}\n")
	// 没有用户的规则退化为 401，不能静默放行
	check.Contains(t, conf, "\t@ace_auth_2 path /empty*\n\terror @ace_auth_2 401\n")

	// 明文文件转为 bcrypt 行
	hashed, err := os.ReadFile(file0 + ".caddy")
	must.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(hashed)), "\n")
	must.Len(t, lines, 2)
	check.True(t, strings.HasPrefix(lines[0], "admin $2a$"), check.Msgf("bcrypt 行格式不符: %s", lines[0]))
	check.True(t, strings.HasPrefix(lines[1], "bob $2a$"), check.Msgf("bcrypt 行格式不符: %s", lines[1]))
	check.NoError(t, bcrypt.CompareHashAndPassword([]byte(strings.TrimPrefix(lines[0], "admin ")), []byte("secret")))

	reloaded, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.BasicAuth(), []types.BasicAuth{
		{Path: "/", UserFile: file0},
		{Path: "/admin", UserFile: file1},
	})

	check.NoError(t, reloaded.ClearBasicAuth())
	check.NoError(t, reloaded.Save())
	check.NotContains(t, siteConf(t, configDir), "basic_auth")
}

func TestVhostRedirects(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	redirects := []types.Redirect{
		{Type: types.RedirectTypeHost, From: "old.example.com", To: "https://example.com", KeepURI: true, StatusCode: 301},
		{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 302},
		{Type: types.RedirectType404, To: "/404-page", StatusCode: 308},
	}
	check.NoError(t, vhost.SetRedirects(redirects))
	check.NoError(t, vhost.Save())

	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\t@ace_redirect_0 host old.example.com\n\tredir @ace_redirect_0 https://example.com{uri} 301\n")
	check.Contains(t, conf, "\t@ace_redirect_1 path /old\n\tredir @ace_redirect_1 /new 302\n")
	check.Contains(t, conf, "\thandle_errors 404 {\n\t\t# ace:redirect 2\n\t\tredir * /404-page 308\n\t}\n")

	reloaded, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Redirects(), redirects)
}

func TestVhostProxies(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	upstreams := []types.Upstream{{
		Name:     "backend",
		Servers:  map[string]string{"127.0.0.1:3001": "weight=5", "127.0.0.1:3002": ""},
		Resolver: []string{},
	}, {
		Name:     "api-pool",
		Servers:  map[string]string{"unix:/tmp/api.sock": "", "10.0.0.2:80": "weight=1 max_fails=3"},
		Algo:     "least_conn",
		Resolver: []string{},
	}}
	check.NoError(t, vhost.SetUpstreams(upstreams))
	proxies := []types.Proxy{
		{
			Location:  "/",
			Pass:      "http://backend",
			Host:      "$proxy_host",
			Buffering: true,
			Resolver:  []string{},
			Headers:   map[string]string{"X-Real-IP": "$remote_addr"},
			Replaces:  map[string]string{"http://old": "https://new"},
		},
		{
			Location:          "^~ /api/",
			Pass:              "https://10.0.0.5/v2/",
			SNI:               "api.internal",
			Buffering:         false,
			Resolver:          []string{},
			Headers:           map[string]string{},
			Replaces:          map[string]string{},
			HTTPVersion:       "2",
			Timeout:           &types.TimeoutConfig{Connect: 5 * time.Second, Read: 90 * time.Second},
			Retry:             &types.RetryConfig{Tries: 3, Timeout: 10 * time.Second},
			ClientMaxBodySize: 10485760,
			SSLBackend:        &types.SSLBackendConfig{Verify: true, TrustedCertificate: "/ca.pem"},
			ResponseHeaders:   &types.ResponseHeaderConfig{Hide: []string{"Server"}, Add: map[string]string{"X-Proxy": "caddy"}},
			AccessControl:     &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
		},
		{
			Location:  "~* \\.(jpg|png)$",
			Pass:      "http://api-pool",
			Buffering: true,
			Resolver:  []string{},
			Headers:   map[string]string{},
			Replaces:  map[string]string{},
		},
		{
			Location:  "= /health",
			Pass:      "http://unix:/tmp/app.sock",
			Buffering: true,
			Resolver:  []string{},
			Headers:   map[string]string{},
			Replaces:  map[string]string{},
		},
	}
	check.NoError(t, vhost.SetProxies(proxies))
	check.NoError(t, vhost.Save())

	conf := siteConf(t, configDir)
	check.Contains(t, conf, "("+upstreamSnippet(configDir, "backend")+") {\n\t# ace:upstream backend\n\tto 127.0.0.1:3001 127.0.0.1:3002\n\tlb_policy weighted_round_robin 5 1\n}\n")
	check.Contains(t, conf, "("+upstreamSnippet(configDir, "api-pool")+") {\n\t# ace:upstream api-pool\n\tto 10.0.0.2:80 unix//tmp/api.sock\n\tlb_policy least_conn\n}\n")
	// 精确、^~ 前缀、正则、普通前缀的顺序
	order := `(?s)@ace_proxy_3 path /health.*@ace_proxy_1 path /api/\*.*@ace_proxy_2 path_regexp \(\?i\)\\\.\(jpg\|png\)\$.*@ace_proxy_0 path /\*`
	check.True(t, regexp.MustCompile(order).MatchString(conf), check.Msgf("代理块顺序不符\n正则: %s\n配置:\n%s", order, conf))
	check.Contains(t, conf, "\t\thandle @ace_proxy_0 {\n\t\t\t# ace:location /\n\t\t\t# ace:pass http://backend\n\t\t\treplace http://old https://new\n\t\t\treverse_proxy {\n\t\t\t\timport "+upstreamSnippet(configDir, "backend")+"\n\t\t\t\theader_up Host {upstream_hostport}\n\t\t\t\theader_up X-Real-IP {remote_host}\n\t\t\t}\n\t\t}\n")
	check.Equal(t, strings.Count(conf, "header_up X-Real-IP {remote_host}\n\t\t\t}\n\t\t}\n\t\t@ace_proxy_1"), 1, "用户自定义的 X-Real-IP 不重复补默认值")
	check.Contains(t, conf, "\t\t\trequest_body {\n\t\t\t\tmax_size 10485760\n\t\t\t}\n")
	check.Contains(t, conf, "\t\t\t@ace_deny_1 remote_ip 10.0.0.99\n\t\t\trespond @ace_deny_1 403\n\t\t\t@ace_allow_1 not remote_ip 10.0.0.0/8\n\t\t\trespond @ace_allow_1 403\n")
	check.Contains(t, conf, "\t\t\turi path_regexp ^/api/ /v2/\n")
	check.Contains(t, conf, "\t\t\treverse_proxy 10.0.0.5:443 {\n\t\t\t\theader_up Host {upstream_hostport}\n\t\t\t\theader_up X-Real-IP {remote_host}\n\t\t\t\theader_down X-Proxy caddy\n\t\t\t\theader_down -Server\n\t\t\t\tflush_interval -1\n\t\t\t\tlb_retries 3\n\t\t\t\tlb_try_duration 10s\n\t\t\t\ttransport http {\n\t\t\t\t\ttls\n\t\t\t\t\ttls_server_name api.internal\n\t\t\t\t\ttls_trust_pool file /ca.pem\n\t\t\t\t\tversions h2c 2\n\t\t\t\t\tdial_timeout 5s\n\t\t\t\t\tresponse_header_timeout 1m30s\n\t\t\t\t}\n\t\t\t}\n")
	check.Contains(t, conf, "\t\t\treverse_proxy unix//tmp/app.sock {\n\t\t\t\theader_up Host {upstream_hostport}\n\t\t\t\theader_up X-Real-IP {remote_host}\n\t\t\t}\n")

	reloaded, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, reloaded.Upstreams(), []types.Upstream{{
		Name:     "backend",
		Servers:  map[string]string{"127.0.0.1:3001": "weight=5", "127.0.0.1:3002": ""},
		Resolver: []string{},
	}, {
		Name:     "api-pool",
		Servers:  map[string]string{"unix:/tmp/api.sock": "", "10.0.0.2:80": ""},
		Algo:     "least_conn",
		Resolver: []string{},
	}})

	// 回读与写入一致，只有两处归一化：未设置的 Host 补成 nginx 的默认变量、与默认值相同的 X-Real-IP 不算用户头
	wantProxies := slices.Clone(proxies)
	wantProxies[0].Headers = map[string]string{}
	for i := range wantProxies {
		if wantProxies[i].Host == "" {
			wantProxies[i].Host = "$proxy_host"
		}
	}
	check.DeepEqual(t, reloaded.Proxies(), wantProxies)

	// 再次保存内容稳定
	check.NoError(t, reloaded.Save())
	check.Equal(t, siteConf(t, configDir), conf)

	check.NoError(t, reloaded.ClearProxies())
	check.NoError(t, reloaded.ClearUpstreams())
	check.NoError(t, reloaded.Save())
	check.NotContains(t, siteConf(t, configDir), "route")
	check.NotContains(t, siteConf(t, configDir), "(ace_upstream_")
}

func TestVhostHTTPSBackendDefaults(t *testing.T) {
	configDir := newConfigDir(t)
	// 未配置后端校验时沿用 nginx 的默认行为：不校验证书
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/", Pass: "https://backend.example.com", Buffering: true}}))
	check.NoError(t, vhost.Save())
	check.Contains(t, siteConf(t, configDir), "\t\t\treverse_proxy backend.example.com:443 {\n\t\t\t\theader_up Host {upstream_hostport}\n\t\t\t\theader_up X-Real-IP {remote_host}\n\t\t\t\ttransport http {\n\t\t\t\t\ttls\n\t\t\t\t\ttls_insecure_skip_verify\n\t\t\t\t}\n\t\t\t}\n")
	reloaded, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	got := reloaded.Proxies()
	must.Len(t, got, 1)
	check.Nil(t, got[0].SSLBackend)
	check.Equal(t, got[0].SNI, "")
}

func TestVhostAccessControlAll(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetProxies([]types.Proxy{
		{Location: "/a", Pass: "http://127.0.0.1:1", Buffering: true, AccessControl: &types.AccessControlConfig{Deny: []string{"all"}}},
		{Location: "/b", Pass: "http://127.0.0.1:1", Buffering: true, AccessControl: &types.AccessControlConfig{Allow: []string{"10.0.0.1", "all"}, Deny: []string{"all"}}},
	}))
	check.NoError(t, vhost.Save())
	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\t\t\t@ace_deny_0 remote_ip 0.0.0.0/0 ::/0\n")
	check.NotContains(t, conf, "@ace_deny_1")
	check.Contains(t, conf, "\t\t\t@ace_allow_1 not remote_ip 10.0.0.1\n")

	reloaded, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	got := reloaded.Proxies()
	must.Len(t, got, 2)
	check.DeepEqual(t, got[0].AccessControl, &types.AccessControlConfig{Deny: []string{"all"}})
	// allow 列表里出现 all 等于不限制，只保留具体地址
	check.DeepEqual(t, got[1].AccessControl, &types.AccessControlConfig{Allow: []string{"10.0.0.1"}})
}

func TestVhostSharedImport(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetConfig("custom.conf", types.ScopeShared, "(shared_snippet) {\n\tencode gzip\n}\n"))
	check.NoError(t, vhost.Save())
	check.Contains(t, siteConf(t, configDir), "import "+filepath.Join(configDir, "shared", "*.conf")+"\n")
	check.Contains(t, vhost.Config("custom.conf", types.ScopeShared), "(shared_snippet) {\n\tencode gzip\n}")
}

func TestVhostDefaultSite(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl"}}}))
	check.NoError(t, vhost.SetServerName([]string{"example.com"}))
	check.NoError(t, vhost.SetSSLConfig(&types.SSLConfig{Cert: "/c", Key: "/k"}))
	check.False(t, vhost.Default())
	check.NoError(t, vhost.SetDefault(true))
	check.NoError(t, vhost.Save())
	conf := siteConf(t, configDir)
	check.Contains(t, conf, "\nhttp://example.com:80,\nhttp://:80 {\n")
	check.Contains(t, conf, "\nhttps://example.com:443,\nhttps://:443 {\n")

	reloaded, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.True(t, reloaded.Default())
	check.DeepEqual(t, reloaded.ServerName(), []string{"example.com"})
	check.DeepEqual(t, reloaded.Listen(), []types.Listen{{Address: "80", Args: []string{}}, {Address: "443", Args: []string{"ssl"}}})

	check.NoError(t, reloaded.SetDefault(false))
	check.NoError(t, reloaded.Save())
	check.NotContains(t, siteConf(t, configDir), "://:")
	again, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.False(t, again.Default())
}

func TestSafeName(t *testing.T) {
	check.Equal(t, safeName("a-b.c_d9"), "a_2db_2ec__d9")
	check.NotEqual(t, safeName("a-b"), safeName("a_b"))
}

func TestVhostReset(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetRoot("/custom"))
	check.NoError(t, vhost.SetServerName([]string{"a.com"}))
	check.NoError(t, vhost.Reset())
	check.Equal(t, vhost.Root(), filepath.Join(filepath.Dir(configDir), "public"))
	check.DeepEqual(t, vhost.ServerName(), []string{"localhost"})
}

func TestVhostConfigFragments(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewStaticVhost(configDir)
	must.NoError(t, err)
	check.NoError(t, vhost.SetConfig("010-cache.conf", types.ScopeSite, "encode gzip\n"))
	cache := vhost.Config("010-cache.conf", types.ScopeSite)
	check.True(t, strings.HasPrefix(cache, "# Auto-generated"), check.Msgf("缺少自动生成头: %s", cache))
	check.Contains(t, cache, "encode gzip")
	check.NoError(t, vhost.SetRawConfig("010-rewrite.conf", types.ScopeSite, "try_files {path} /index.html\n"))
	check.Equal(t, vhost.Config("010-rewrite.conf", types.ScopeSite), "try_files {path} /index.html")
	check.NoError(t, vhost.RemoveConfig("010-rewrite.conf", types.ScopeSite))
	check.Equal(t, vhost.Config("010-rewrite.conf", types.ScopeSite), "")
}

func TestDialect(t *testing.T) {
	d := Dialect{}
	check.Equal(t, d.Service(), "caddy")
	check.Equal(t, d.ConfigFile(), ConfName)
	check.DeepEqual(t, d.HTTPSListenArgs(), []string{"ssl"})
	check.DeepEqual(t, d.Features(), types.Features{Stat: true, DefaultSite: true})
	check.Equal(t, d.RewritesDir(), "caddy")
	shared, site := d.StatConf("demo")
	check.Equal(t, shared, "")
	check.Contains(t, site, "output net unixgram//tmp/ace_stats.sock")
	check.Contains(t, site, "\t\t\tsite demo\n")
	check.Equal(t, d.DefaultSiteConf(), "")
	check.NoError(t, d.WriteDefaultSite(true))
	check.Equal(t, d.HTPasswdLine("admin", "secret"), "admin:{PLAIN}secret")
	check.NoError(t, d.BeforeReload())
}

func TestVhostProxyCustom(t *testing.T) {
	configDir := newConfigDir(t)
	vhost, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	custom := "header X-Custom value\n# 注释保留\nencode gzip"
	check.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://127.0.0.1:8080", Buffering: true, Custom: custom}}))
	check.NoError(t, vhost.Save())
	// 要落在该规则的 handle 里才只对该规则生效
	check.Contains(t, siteConf(t, configDir), "\t\t\t}\n\t\t\t# ace:custom\n\t\t\theader X-Custom value\n")

	reloaded, err := NewProxyVhost(configDir)
	must.NoError(t, err)
	got := reloaded.Proxies()
	must.Len(t, got, 1)
	check.Equal(t, got[0].Custom, custom)
	check.Nil(t, got[0].ResponseHeaders)

	check.Error(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://127.0.0.1:8080", Custom: "header {"}}))
}

package apache

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

func proxyConf(t *testing.T, configDir string) string {
	t.Helper()
	return readFile(t, filepath.Join(configDir, "site", "200-proxy.conf"))
}

// 匹配优先级与 nginx 一致，不看规则添加的先后：精确、^~ 前缀、正则、最长前缀
func TestProxyVhostPriority(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)
	proxies := []types.Proxy{
		{Location: "/", Pass: "http://127.0.0.1:8080"},
		{Location: "/api", Pass: "http://127.0.0.1:8081"},
		{Location: "~* \\.(jpg|png)$", Pass: "http://127.0.0.1:8082"},
		{Location: "^~ /static/", Pass: "http://127.0.0.1:8083"},
		{Location: "= /health", Pass: "http://127.0.0.1:8084"},
		{Location: "/api/v2", Pass: "http://127.0.0.1:8085"},
	}
	must.NoError(t, vhost.SetProxies(proxies))
	check.Len(t, confFiles(t, configDir, "site", "2", "-proxy.conf"), 1)

	content := proxyConf(t, configDir)
	var written []string
	for _, m := range regexp.MustCompile(`# ace:location (.+)`).FindAllStringSubmatch(content, -1) {
		written = append(written, m[1])
	}
	check.DeepEqual(t, written, []string{"= /health", "^~ /static/", "~* \\.(jpg|png)$", "/api/v2", "/api", "/"})
	// 链的起点是证书验证路径，每条规则各占一节
	check.Contains(t, content, "    ProxyPassMatch ^/\\.well-known/acme-challenge/ !\n    <If \"%{REQUEST_URI} =~ m#^/\\.well-known/acme-challenge/#\">\n    </If>\n")
	check.Equal(t, strings.Count(content, "<If "), 1)
	check.Equal(t, strings.Count(content, "<ElseIf "), len(proxies))

	// 回读还原成用户定义的顺序
	check.DeepEqual(t, vhost.Proxies(), withProxyDefaults(proxies), cmpopts.EquateEmpty())
}

func TestRoute(t *testing.T) {
	for _, tt := range []struct {
		location, pass string
		want           route
	}{
		// proxy_pass 不带路径：原样转发 URI
		{"/", "http://b:8080", route{pattern: "^/", ahead: "/", target: "http://b:8080", origin: "http://b:8080", front: "/", back: "http://b:8080/"}},
		{"^~ /a.b", "http://b", route{pattern: `^/a\.b`, ahead: `/a\.b`, target: "http://b", origin: "http://b", front: "/", back: "http://b/"}},
		{"= /health", "http://b", route{pattern: "^/health$", ahead: "/health$", target: "http://b", origin: "http://b", front: "/", back: "http://b/"}},
		// 带路径：匹配到的部分换成该路径
		{"/api/", "http://b:8080/v2/", route{
			pattern: "^/api/(.*)$", ahead: "/api/", target: "http://b:8080/v2/$1", worker: "http://b:8080/v2/",
			origin: "http://b:8080", front: "/api/", back: "http://b:8080/v2/",
		}},
		{"/api", "balancer://pool/v2", route{
			pattern: "^/api(.*)$", ahead: "/api", target: "balancer://pool/v2$1", worker: "balancer://pool/v2",
			origin: "balancer://pool", front: "/api", back: "balancer://pool/v2",
		}},
		{"= /health", "http://b/status", route{
			pattern: "^/health()$", ahead: "/health$", target: "http://b/status$1", worker: "http://b/status",
			origin: "http://b", front: "/health", back: "http://b/status",
		}},
		// 正则不允许带路径
		{"~ ^/v[0-9]+/", "http://b/ignored", route{pattern: "^/v[0-9]+/", ahead: ".*(?:^/v[0-9]+/)", target: "http://b", origin: "http://b", front: "/", back: "http://b/"}},
		{"~* \\.(jpg|png)$", "http://b", route{
			pattern: `(?i)\.(jpg|png)$`, ahead: `.*(?:(?i)\.(jpg|png)$)`, target: "http://b", origin: "http://b", front: "/", back: "http://b/",
		}},
		// unix 套接字：ProxyPassReverse 与 <Proxy> 用的是 | 后面的地址
		{"/", "http://unix:/tmp/app.sock", route{
			pattern: "^/", ahead: "/", target: "unix:/tmp/app.sock|http://uds-a73c9e6d", origin: "http://uds-a73c9e6d", front: "/", back: "http://uds-a73c9e6d/",
		}},
		{"/api/", "https://unix:/tmp/app.sock:/v2/", route{
			pattern: "^/api/(.*)$", ahead: "/api/", target: "unix:/tmp/app.sock|https://uds-a73c9e6d/v2/$1", worker: "unix:/tmp/app.sock|https://uds-a73c9e6d/v2/",
			origin: "https://uds-a73c9e6d", front: "/api/", back: "https://uds-a73c9e6d/v2/",
		}},
	} {
		check.Equal(t, newRoute(tt.location, tt.pass), tt.want, tt.location+" -> "+tt.pass)
	}
}

func TestMatchExpr(t *testing.T) {
	expr, err := matchExpr("^/api/")
	check.NoError(t, err)
	check.Equal(t, expr, "%{REQUEST_URI} =~ m#^/api/#")
	// 表达式里的正则不认转义的定界符，正则里出现过的就换一个
	expr, err = matchExpr("^/a#b,c")
	check.NoError(t, err)
	check.Equal(t, expr, "%{REQUEST_URI} =~ m;^/a#b,c;")
	_, err = matchExpr("#,;:!%_-|")
	check.Error(t, err)
}

// 每条规则的 Host、请求头、响应头、访问控制都只在自己那一节里
func TestProxyVhostSections(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)
	proxies := []types.Proxy{
		{
			Location: "/a", Pass: "http://127.0.0.1:8080", Host: "a.example.com", Buffering: true,
			// 与内置头同名：改了值的顶替内置值，值相同的不重复写、回读也不算自定义头
			Headers:         map[string]string{"X-Rule": "a", "X-Forwarded-Proto": "https", "X-Real-IP": "$remote_addr"},
			ResponseHeaders: &types.ResponseHeaderConfig{Hide: []string{"Server"}, Add: map[string]string{"X-Resp": "a"}},
		},
		{Location: "/b", Pass: "http://127.0.0.1:8081", Host: "$http_host"},
		{
			Location: "/c", Pass: "https://127.0.0.1:8443/v2", ClientMaxBodySize: 1024,
			Timeout:    &types.TimeoutConfig{Connect: 5 * time.Second, Read: 30 * time.Second},
			SSLBackend: &types.SSLBackendConfig{Verify: true, TrustedCertificate: "/ca.pem", VerifyDepth: 2},
			Replaces:   map[string]string{"http://old": "https://new"},
		},
	}
	must.NoError(t, vhost.SetProxies(proxies))
	content := proxyConf(t, configDir)

	check.Contains(t, content, `    ProxyPassMatch ^/a http://127.0.0.1:8080 upgrade=websocket
    ProxyPassReverse / http://127.0.0.1:8080/
    <ElseIf "%{REQUEST_URI} =~ m#^/a#">
        RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
        RequestHeader set X-Forwarded-Proto https
        ProxyPreserveHost On
        RequestHeader set Host a.example.com
        RequestHeader set X-Rule a
        Header unset Server
        Header set X-Resp a
    </ElseIf>
`)
	// 不缓冲写成连接参数
	check.Contains(t, content, `    ProxyPassMatch ^/b http://127.0.0.1:8081 flushpackets=on upgrade=websocket
    ProxyPassReverse / http://127.0.0.1:8081/
    <ElseIf "%{REQUEST_URI} =~ m#^/b#">
        RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
        RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
        ProxyPreserveHost On
        RequestHeader set Host expr=%{HTTP_HOST}
    </ElseIf>
`)
	// 目标带路径时同一截地址另有一个 worker；后端 TLS 按后端地址圈定；请求体上限按 Content-Length 拦，不带长度的分块请求一律要求带上
	check.Contains(t, content, `    ProxyPassMatch ^/c(.*)$ https://127.0.0.1:8443/v2$1 connectiontimeout=5 timeout=30 flushpackets=on upgrade=websocket
    ProxyPassReverse /c https://127.0.0.1:8443/v2
    <Proxy https://127.0.0.1:8443/v2>
        ProxySet connectiontimeout=5 timeout=30 flushpackets=on upgrade=websocket
    </Proxy>
    <Proxy https://127.0.0.1:8443/>
        SSLProxyEngine On
        SSLProxyVerify require
        SSLProxyVerifyDepth 2
        SSLProxyCACertificateFile /ca.pem
    </Proxy>
    <ElseIf "%{REQUEST_URI} =~ m#^/c(.*)$#">
        RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
        RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
        <If "%{HTTP:Content-Length} -gt 1024">
            Redirect 413
        </If>
        <ElseIf "-n %{HTTP:Transfer-Encoding}">
            Redirect 411
        </ElseIf>
        <IfModule mod_substitute.c>
            RequestHeader unset Accept-Encoding
            AddOutputFilterByType SUBSTITUTE text/html text/plain text/xml
            Substitute "s|http://old|https://new|n"
        </IfModule>
    </ElseIf>
`)

	want := withProxyDefaults(proxies)
	want[0].Headers = map[string]string{"X-Rule": "a", "X-Forwarded-Proto": "https"}
	// $http_host 与 $host 在 Apache 下是同一个表达式，回读取规范写法
	want[1].Host = "$host"
	check.DeepEqual(t, vhost.Proxies(), want, cmpopts.EquateEmpty())
}

func TestProxyVhostAccessControl(t *testing.T) {
	for _, tt := range []struct {
		name string
		ac   types.AccessControlConfig
		conf string
		want *types.AccessControlConfig
	}{
		{"全部拒绝", types.AccessControlConfig{Deny: []string{"all"}}, "        AuthMerging And\n        Require all denied\n", &types.AccessControlConfig{Deny: []string{"all"}}},
		{"只填允许列表即白名单", types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}}, "        AuthMerging And\n        Require ip 10.0.0.0/8\n", &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}}},
		{"白名单带 deny all", types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"all"}}, "        AuthMerging And\n        Require ip 10.0.0.0/8\n", &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}}},
		{"全部允许", types.AccessControlConfig{Allow: []string{"all"}}, "", nil},
		{"黑名单", types.AccessControlConfig{Deny: []string{"10.0.0.99"}}, "            Require all granted\n            Require not ip 10.0.0.99\n", &types.AccessControlConfig{Deny: []string{"10.0.0.99"}}},
		{
			"允许并排除", types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
			"            Require ip 10.0.0.0/8\n            Require not ip 10.0.0.99\n",
			&types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			vhost, configDir := setupProxyVhost(t)
			must.NoError(t, vhost.SetProxies([]types.Proxy{{Location: "/api", Pass: "http://127.0.0.1:8080", AccessControl: &tt.ac}}))
			content := proxyConf(t, configDir)
			check.Contains(t, content, tt.conf)
			// all 不是合法的 IP，写进 Require ip 会让整个 Apache 拒载
			check.NotContains(t, content, "ip all")
			got := vhost.Proxies()
			must.Len(t, got, 1)
			check.DeepEqual(t, got[0].AccessControl, tt.want, cmpopts.EquateEmpty())
		})
	}
}

// Apache 表达不了的设置不写进配置，也不靠注释留存
func TestProxyVhostUnsupported(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)
	must.NoError(t, vhost.SetUpstreams([]types.Upstream{{
		Name: "backend", Servers: map[string]string{"127.0.0.1:8080": "weight=5 max_fails=3"},
		Algo: types.AlgoIPHash, Keepalive: 16, Resolver: []string{"8.8.8.8"},
	}}))
	balancer := readFile(t, filepath.Join(configDir, "shared", "100-balancer-backend.conf"))
	check.Contains(t, balancer, "        ProxySet lbmethod=byrequests\n")
	check.NotContains(t, balancer, "ace:")
	check.NotContains(t, balancer, "max=")
	check.DeepEqual(t, vhost.Upstreams(), []types.Upstream{{Name: "backend", Servers: map[string]string{"127.0.0.1:8080": "weight=5"}}}, cmpopts.EquateEmpty())

	must.NoError(t, vhost.SetProxies([]types.Proxy{{
		Location: "/", Pass: "https://backend.example.com", Buffering: true,
		SNI: "sni.example.com", HTTPVersion: "2", Resolver: []string{"8.8.8.8"},
		Timeout: &types.TimeoutConfig{Send: 30 * time.Second},
		Retry:   &types.RetryConfig{Conditions: []string{"error"}, Tries: 3, Timeout: 10 * time.Second},
	}}))
	content := proxyConf(t, configDir)
	for _, absent := range []string{"sni", "retry=", "timeout=", "ProxyIOBufferSize"} {
		check.NotContains(t, content, absent)
	}
	check.DeepEqual(t, vhost.Proxies(), []types.Proxy{
		{Location: "/", Pass: "https://backend.example.com", Host: "$proxy_host", HTTPVersion: "1.1", Buffering: true},
	}, cmpopts.EquateEmpty())
}

// 缓存只对开了它的规则生效：开关圈在只在该规则胜出时命中的 <LocationMatch> 里，排在前面的规则与证书验证路径都被排除
func TestProxyVhostCache(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)
	proxies := []types.Proxy{
		{Location: "/", Pass: "http://127.0.0.1:8080", Cache: &types.CacheConfig{
			Valid: map[string]string{"200 302": "10m", "404": "1m"}, NoCacheConditions: []string{"$cookie_nocache"},
			UseStale: []string{"error"}, BackgroundUpdate: true, Lock: true, MinUses: 2, Methods: []string{"GET"}, Key: "$uri",
		}},
		{Location: "^~ /api/", Pass: "http://127.0.0.1:8081"},
		{Location: "~* \\.(jpg|png)$", Pass: "http://127.0.0.1:8082", Cache: &types.CacheConfig{Valid: map[string]string{"any": "1d"}}},
		{Location: "= /health", Pass: "http://127.0.0.1:8083", Cache: &types.CacheConfig{}},
	}
	must.NoError(t, vhost.SetProxies(proxies))
	content := proxyConf(t, configDir)

	check.Equal(t, strings.Count(content, "CacheEnable disk"), 3)
	// 没填有效期的按 10 分钟，不落到 httpd 默认的一小时
	check.Contains(t, content, `    <LocationMatch ^(?!/\.well-known/acme-challenge/)(?=/health$)>
        CacheEnable disk
        CacheIgnoreNoLastMod On
        CacheStaleOnError Off
        Header merge Cache-Control private "expr=-n %{resp:Set-Cookie}"
        CacheDefaultExpire 600
        CacheMinExpire 600
        CacheMaxExpire 600
    </LocationMatch>
`)
	check.Contains(t, content, `    <LocationMatch ^(?!/\.well-known/acme-challenge/)(?!/health$)(?!/api/)(?=.*(?:(?i)\.(jpg|png)$))>
        CacheEnable disk
        CacheIgnoreNoLastMod On
        CacheStaleOnError Off
        Header merge Cache-Control private "expr=-n %{resp:Set-Cookie}"
        CacheDefaultExpire 86400
        CacheMinExpire 86400
        CacheMaxExpire 86400
    </LocationMatch>
`)
	check.Contains(t, content, `    <LocationMatch ^(?!/\.well-known/acme-challenge/)(?!/health$)(?!/api/)(?!.*(?:(?i)\.(jpg|png)$))(?=/)>
        CacheEnable disk
        CacheIgnoreNoLastMod On
        CacheStaleOnError Off
        Header merge Cache-Control private "expr=-n %{resp:Set-Cookie}"
        CacheDefaultExpire 600
        CacheMinExpire 600
        CacheMaxExpire 600
    </LocationMatch>
`)

	// Apache 只有一个有效期，其余细项表达不了，不写也不读
	want := withProxyDefaults(proxies)
	want[0].Cache = &types.CacheConfig{Valid: map[string]string{"200 301": "10m"}}
	want[2].Cache = &types.CacheConfig{Valid: map[string]string{"200 301": "1d"}}
	want[3].Cache = &types.CacheConfig{Valid: map[string]string{"200 301": "10m"}}
	check.DeepEqual(t, vhost.Proxies(), want, cmpopts.EquateEmpty())

	proxies[0].Cache.Valid = map[string]string{"200": "1h30m"}
	check.ErrorContains(t, vhost.SetProxies(proxies), "invalid cache duration")
	check.Equal(t, proxyConf(t, configDir), content)
}

func TestTTL(t *testing.T) {
	for _, tt := range []struct {
		in      string
		seconds int
		out     string
	}{
		{"90", 90, "90s"}, {"90s", 90, "90s"}, {"10m", 600, "10m"}, {"120m", 7200, "2h"}, {"1d", 86400, "1d"},
		{"1w", 604800, "7d"}, {"1M", 2592000, "30d"}, {"1y", 31536000, "365d"},
	} {
		seconds, ok := parseTTL(tt.in)
		check.True(t, ok, tt.in)
		check.Equal(t, seconds, tt.seconds, tt.in)
		check.Equal(t, formatTTL(seconds), tt.out, tt.in)
	}
	for _, in := range []string{"", "0", "-1m", "10ms", "1h30m", "abc"} {
		_, ok := parseTTL(in)
		check.False(t, ok, in)
	}
}

// nginx 的 unix 套接字写法要换成 Apache 的。worker 按 | 后面的地址区分，各个套接字的主机名不能相同，否则都连到先定义的那个
func TestProxyVhostUnixSocket(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)
	upstreams := []types.Upstream{{Name: "socks", Servers: map[string]string{"unix:/tmp/a.sock": "weight=2", "unix:/tmp/b.sock": ""}}}
	must.NoError(t, vhost.SetUpstreams(upstreams))
	balancer := readFile(t, filepath.Join(configDir, "shared", "100-balancer-socks.conf"))
	check.Contains(t, balancer, "        BalancerMember unix:/tmp/a.sock|http://uds-aa201f48 upgrade=websocket loadfactor=2\n")
	check.Contains(t, balancer, "        BalancerMember unix:/tmp/b.sock|http://uds-2cb46de6 upgrade=websocket\n")
	check.DeepEqual(t, vhost.Upstreams(), upstreams, cmpopts.EquateEmpty())

	proxies := []types.Proxy{
		{Location: "/", Pass: "http://unix:/tmp/a.sock", Host: "$host"},
		{Location: "/api/", Pass: "http://unix:/tmp/b.sock:/v2/"},
	}
	must.NoError(t, vhost.SetProxies(proxies))
	content := proxyConf(t, configDir)
	check.Contains(t, content, `    ProxyPassMatch ^/ unix:/tmp/a.sock|http://uds-aa201f48 flushpackets=on upgrade=websocket
    ProxyPassReverse / http://uds-aa201f48/
    <ElseIf "%{REQUEST_URI} =~ m#^/#">
        RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
        RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
        ProxyPreserveHost On
    </ElseIf>
`)
	// 主机名是编出来的，不能当 Host 发给后端：nginx 的 $proxy_host 在套接字上是 localhost
	check.Contains(t, content, `    ProxyPassMatch ^/api/(.*)$ unix:/tmp/b.sock|http://uds-2cb46de6/v2/$1 flushpackets=on upgrade=websocket
    ProxyPassReverse /api/ http://uds-2cb46de6/v2/
    <Proxy unix:/tmp/b.sock|http://uds-2cb46de6/v2/>
        ProxySet flushpackets=on upgrade=websocket
    </Proxy>
    <ElseIf "%{REQUEST_URI} =~ m#^/api/(.*)$#">
        RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
        RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
        ProxyPreserveHost On
        RequestHeader set Host localhost
    </ElseIf>
`)
	want := withProxyDefaults(proxies)
	want[1].Host = "localhost"
	check.DeepEqual(t, vhost.Proxies(), want, cmpopts.EquateEmpty())
}

// 旧版一条规则一个文件，Host 与自定义请求头写在 vhost 级，要能读回来，重新保存后换成新写法
func TestProxyVhostLegacyLayout(t *testing.T) {
	vhost, configDir := setupProxyVhost(t)
	siteDir := filepath.Join(configDir, "site")
	must.NoError(t, os.WriteFile(filepath.Join(siteDir, "200-proxy.conf"), []byte(`# Auto-generated by AcePanel. DO NOT EDIT MANUALLY!
# Reverse proxy: ~* \.(jpg|png)$ -> http://127.0.0.1:9000
<IfModule mod_proxy.c>
    # ace:location ~* \.(jpg|png)$
    # ace:pass http://127.0.0.1:9000
    # ace:sni img.internal
    ProxyPassMatch (?i)\.(jpg|png)$ http://127.0.0.1:9000 connectiontimeout=5 retry=10 upgrade=websocket
    ProxyPassReverse / http://127.0.0.1:9000
    RequestHeader set Host img.example.com
    RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
    RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
    ProxyIOBufferSize 65536
    <IfModule mod_cache.c>
        CacheEnable disk /
        CacheDefaultExpire 600
    </IfModule>
    <IfModule mod_headers.c>
        RequestHeader set X-Static expr=%{REQUEST_SCHEME}
    </IfModule>
    <IfModule mod_substitute.c>
        AddOutputFilterByType SUBSTITUTE text/html text/plain text/xml
        Substitute "s|http://old|https://new|n"
    </IfModule>
    <Location />
        Header always set X-Static 1
        LimitRequestBody 1048576
    </Location>
</IfModule>
`), 0600))
	must.NoError(t, os.WriteFile(filepath.Join(siteDir, "201-proxy.conf"), []byte(`# Auto-generated by AcePanel. DO NOT EDIT MANUALLY!
# Reverse proxy: /app -> http://127.0.0.1:9001
<IfModule mod_proxy.c>
    # ace:location /app
    # ace:pass http://127.0.0.1:9001
    ProxyPass /app http://127.0.0.1:9001/app upgrade=websocket
    ProxyPassReverse /app http://127.0.0.1:9001/app
    RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
    RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
    <Location /app>
        ProxyPreserveHost On
        RequestHeader set X-Rule app
        Require ip 10.0.0.0/8
        # ace:custom
        Header always set X-Custom value
    </Location>
</IfModule>
`), 0600))

	want := []types.Proxy{
		{
			Location: "~* \\.(jpg|png)$", Pass: "http://127.0.0.1:9000", Host: "img.example.com", HTTPVersion: "1.1", Buffering: true,
			Headers: map[string]string{"X-Static": "$scheme"}, Replaces: map[string]string{"http://old": "https://new"},
			ClientMaxBodySize: 1048576, Timeout: &types.TimeoutConfig{Connect: 5 * time.Second},
			ResponseHeaders: &types.ResponseHeaderConfig{Add: map[string]string{"X-Static": "1"}},
		},
		{
			Location: "/app", Pass: "http://127.0.0.1:9001", Host: "$host", HTTPVersion: "1.1",
			Headers:       map[string]string{"X-Rule": "app"},
			AccessControl: &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}},
			Custom:        "Header always set X-Custom value",
		},
	}
	check.DeepEqual(t, vhost.Proxies(), want, cmpopts.EquateEmpty())

	must.NoError(t, vhost.SetProxies(vhost.Proxies()))
	check.DeepEqual(t, confFiles(t, configDir, "site", "2", "-proxy.conf"), []string{"200-proxy.conf"})
	content := proxyConf(t, configDir)
	check.NotContains(t, content, "<Location")
	check.NotContains(t, content, "mod_headers.c")
	check.Contains(t, content, "    <ElseIf \"%{REQUEST_URI} =~ m#(?i)\\.(jpg|png)$#\">\n")
	check.Contains(t, content, "    <ElseIf \"%{REQUEST_URI} =~ m#^/app#\">\n")
	check.DeepEqual(t, vhost.Proxies(), want, cmpopts.EquateEmpty())
}

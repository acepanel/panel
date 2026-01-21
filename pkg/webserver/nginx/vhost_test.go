package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/acepanel/panel/pkg/webserver/types"
)

type VhostTestSuite struct {
	suite.Suite
	vhost     *PHPVhost
	configDir string
}

func TestVhostTestSuite(t *testing.T) {
	suite.Run(t, &VhostTestSuite{})
}

func (s *VhostTestSuite) SetupTest() {
	// 创建临时配置目录
	configDir, err := os.MkdirTemp("", "nginx-test-*")
	s.Require().NoError(err)
	s.configDir = configDir

	// 创建 site 目录
	err = os.MkdirAll(filepath.Join(configDir, "site"), 0755)
	s.Require().NoError(err)

	vhost, err := NewPHPVhost(configDir)
	s.Require().NoError(err)
	s.Require().NotNil(vhost)
	s.vhost = vhost
}

func (s *VhostTestSuite) TearDownTest() {
	// 清理临时目录
	if s.configDir != "" {
		s.NoError(os.RemoveAll(s.configDir))
	}
}

func (s *VhostTestSuite) TestNewVhost() {
	s.Equal(s.configDir, s.vhost.configDir)
	s.NotNil(s.vhost.parser)
}

func (s *VhostTestSuite) TestEnable() {
	// 默认应该是启用状态
	s.True(s.vhost.Enable())

	// 禁用网站
	s.NoError(s.vhost.SetEnable(false))
	s.False(s.vhost.Enable())

	// 重新启用
	s.NoError(s.vhost.SetEnable(true))
	s.True(s.vhost.Enable())
}

func (s *VhostTestSuite) TestServerName() {
	names := []string{"example.com", "www.example.com", "api.example.com"}
	s.NoError(s.vhost.SetServerName(names))

	got := s.vhost.ServerName()
	s.Len(got, 3)
	s.Equal("example.com", got[0])
	s.Equal("www.example.com", got[1])
	s.Equal("api.example.com", got[2])
}

func (s *VhostTestSuite) TestServerNameEmpty() {
	s.NoError(s.vhost.SetServerName([]string{}))
}

func (s *VhostTestSuite) TestRoot() {
	root := "/var/www/html"
	s.NoError(s.vhost.SetRoot(root))
	s.Equal(root, s.vhost.Root())
}

func (s *VhostTestSuite) TestIndex() {
	index := []string{"index.html", "index.php", "default.html"}
	s.NoError(s.vhost.SetIndex(index))

	got := s.vhost.Index()
	s.Len(got, 3)
	s.Equal(index, got)
}

func (s *VhostTestSuite) TestIndexEmpty() {
	s.NoError(s.vhost.SetIndex([]string{}))
}

func (s *VhostTestSuite) TestListen() {
	listens := []types.Listen{
		{Address: "80"},
		{Address: "443", Args: []string{"ssl"}},
	}
	s.NoError(s.vhost.SetListen(listens))

	got := s.vhost.Listen()
	s.Len(got, 2)
}

func (s *VhostTestSuite) TestListenWithHTTP3() {
	listens := []types.Listen{
		{Address: "443", Args: []string{"quic"}},
	}
	s.NoError(s.vhost.SetListen(listens))

	got := s.vhost.Listen()
	s.Len(got, 1)
	s.Equal("quic", got[0].Args[0])
}

func (s *VhostTestSuite) TestListenWithSSLAndQUIC() {
	// 测试 ssl 和 quic 同时存在时，应该分成两行 listen 指令
	// 但读取时应该合并为一个 Listen 对象
	listens := []types.Listen{
		{Address: "80"},
		{Address: "443", Args: []string{"ssl", "quic"}},
	}
	s.NoError(s.vhost.SetListen(listens))

	// 保存后验证顺序
	s.NoError(s.vhost.Save())

	// 验证生成的配置中 ssl 和 quic 是分开的
	dump := s.vhost.parser.Dump()
	s.Contains(dump, "listen 443 ssl;")
	s.Contains(dump, "listen 443 quic;")
	// 确保没有 "listen 443 ssl quic;" 这样的行
	s.NotContains(dump, "listen 443 ssl quic;")

	// 验证顺序：80 应该在 443 前面
	idx80 := strings.Index(dump, "listen 80;")
	idx443 := strings.Index(dump, "listen 443")
	s.Greater(idx443, idx80, "listen 80 should come before listen 443")

	// 读取时应该合并为一个 Listen 对象
	got := s.vhost.Listen()
	s.Len(got, 2) // 80 和 443
	// 验证顺序
	s.Equal("80", got[0].Address)
	s.Equal("443", got[1].Address)
	// 验证 443 的 args
	s.Contains(got[1].Args, "ssl")
	s.Contains(got[1].Args, "quic")
}

func (s *VhostTestSuite) TestSSL() {
	s.False(s.vhost.SSL())
	s.Nil(s.vhost.SSLConfig())
}

func (s *VhostTestSuite) TestSetSSLConfig() {
	sslConfig := &types.SSLConfig{
		Cert:      "/etc/ssl/cert.pem",
		Key:       "/etc/ssl/key.pem",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
		HSTS:      true,
		OCSP:      true,
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))

	s.True(s.vhost.SSL())

	got := s.vhost.SSLConfig()
	s.NotNil(got)
	s.True(got.HSTS)
	s.True(got.OCSP)
}

func (s *VhostTestSuite) TestSetSSLConfigNil() {
	s.Error(s.vhost.SetSSLConfig(nil))
}

func (s *VhostTestSuite) TestClearSSL() {
	sslConfig := &types.SSLConfig{
		Cert: "/etc/ssl/cert.pem",
		Key:  "/etc/ssl/key.pem",
		HSTS: true,
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))
	s.True(s.vhost.SSL())

	s.NoError(s.vhost.ClearSSL())
	s.False(s.vhost.SSL())
}

func (s *VhostTestSuite) TestPHP() {
	s.Equal(uint(0), s.vhost.PHP())

	s.NoError(s.vhost.SetPHP(84))
	s.Equal(uint(84), s.vhost.PHP())

	s.NoError(s.vhost.SetPHP(0))
	s.Equal(uint(0), s.vhost.PHP())
}

func (s *VhostTestSuite) TestAccessLog() {
	accessLog := "/var/log/nginx/access.log"
	s.NoError(s.vhost.SetAccessLog(accessLog))
	s.Equal(accessLog, s.vhost.AccessLog())
}

func (s *VhostTestSuite) TestErrorLog() {
	errorLog := "/var/log/nginx/error.log"
	s.NoError(s.vhost.SetErrorLog(errorLog))
	s.Equal(errorLog, s.vhost.ErrorLog())
}

func (s *VhostTestSuite) TestIncludes() {
	includes := []types.IncludeFile{
		{Path: "/etc/nginx/conf.d/ssl.conf"},
		{Path: "/etc/nginx/conf.d/php.conf"},
	}
	s.NoError(s.vhost.SetIncludes(includes))

	got := s.vhost.Includes()
	s.Len(got, 2)
	s.Equal(includes[0].Path, got[0].Path)
	s.Equal(includes[1].Path, got[1].Path)
}

func (s *VhostTestSuite) TestBasicAuth() {
	s.Nil(s.vhost.BasicAuth())

	auth := map[string]string{
		"realm":     "Test Realm",
		"user_file": "/etc/nginx/htpasswd",
	}
	s.NoError(s.vhost.SetBasicAuth(auth))

	got := s.vhost.BasicAuth()
	s.NotNil(got)
	s.Equal(auth["user_file"], got["user_file"])

	s.NoError(s.vhost.ClearBasicAuth())
	s.Nil(s.vhost.BasicAuth())
}

func (s *VhostTestSuite) TestRateLimit() {
	s.Nil(s.vhost.RateLimit())

	limit := &types.RateLimit{
		PerServer: 300,
		PerIP:     25,
		Rate:      512,
	}
	s.NoError(s.vhost.SetRateLimit(limit))

	got := s.vhost.RateLimit()
	s.NotNil(got)
	s.Equal(300, got.PerServer)
	s.Equal(25, got.PerIP)
	s.Equal(512, got.Rate)

	s.NoError(s.vhost.ClearRateLimit())
	s.Nil(s.vhost.RateLimit())
}

func (s *VhostTestSuite) TestReset() {
	s.NoError(s.vhost.SetServerName([]string{"modified.com"}))
	s.NoError(s.vhost.SetRoot("/modified/path"))

	s.NoError(s.vhost.Reset())

	names := s.vhost.ServerName()
	s.NotContains(names, "modified.com")
}

func (s *VhostTestSuite) TestSave() {
	// 设置配置文件路径
	configFile := filepath.Join(s.configDir, "nginx.conf")
	s.vhost.parser.SetConfigPath(configFile)

	s.NoError(s.vhost.SetServerName([]string{"save-test.com"}))
	s.NoError(s.vhost.Save())

	// 验证配置文件已保存
	content, err := os.ReadFile(configFile)
	s.NoError(err)
	s.Contains(string(content), "save-test.com")
}

func (s *VhostTestSuite) TestDump() {
	s.NoError(s.vhost.SetServerName([]string{"dump-test.com"}))
	s.NoError(s.vhost.SetRoot("/var/www/dump-test"))

	content := s.vhost.parser.Dump()
	s.NotEmpty(content)
	s.Contains(content, "dump-test.com")
	s.Contains(content, "/var/www/dump-test")
	s.Contains(content, "server")
}

func (s *VhostTestSuite) TestDumpWithSSL() {
	sslConfig := &types.SSLConfig{
		Cert:      "/etc/ssl/cert.pem",
		Key:       "/etc/ssl/key.pem",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))

	content := s.vhost.parser.Dump()
	s.Contains(content, "ssl_certificate")
	s.Contains(content, "ssl_certificate_key")
}

func (s *VhostTestSuite) TestHTTPSRedirect() {
	sslConfig := &types.SSLConfig{
		Cert:         "/etc/ssl/cert.pem",
		Key:          "/etc/ssl/key.pem",
		HTTPRedirect: true,
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))

	got := s.vhost.SSLConfig()
	s.NotNil(got)
	s.True(got.HTTPRedirect)
}

func (s *VhostTestSuite) TestAltSvc() {
	sslConfig := &types.SSLConfig{
		Cert:   "/etc/ssl/cert.pem",
		Key:    "/etc/ssl/key.pem",
		AltSvc: `h3=":$server_port"; ma=2592000`,
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))

	got := s.vhost.SSLConfig()
	s.NotNil(got)
	s.Contains(got.AltSvc, "h3=")
}

func (s *VhostTestSuite) TestDefaultConfIncludesServerD() {
	// 验证默认配置包含 site 的 include
	s.Contains(DefaultConf, "site")
	s.Contains(DefaultConf, "include")
}

func (s *VhostTestSuite) TestRedirects() {
	// 初始应该没有重定向
	s.Empty(s.vhost.Redirects())

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
	s.NoError(s.vhost.SetRedirects(redirects))

	// 验证重定向文件已创建
	siteDir := filepath.Join(s.configDir, "site")
	entries, err := os.ReadDir(siteDir)
	s.NoError(err)

	redirectCount := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "1") && strings.HasSuffix(entry.Name(), "-redirect.conf") {
			redirectCount++
		}
	}
	s.Equal(2, redirectCount)

	// 验证可以读取回来
	got := s.vhost.Redirects()
	s.Len(got, 2)
}

func (s *VhostTestSuite) TestRedirectURL() {
	redirects := []types.Redirect{
		{
			Type:       types.RedirectTypeURL,
			From:       "/old-page",
			To:         "/new-page",
			StatusCode: 301,
		},
	}
	s.NoError(s.vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	s.NoError(err)

	s.Contains(string(content), "location = /old-page")
	s.Contains(string(content), "return 301")
	s.Contains(string(content), "/new-page")
}

func (s *VhostTestSuite) TestRedirectHost() {
	redirects := []types.Redirect{
		{
			Type:       types.RedirectTypeHost,
			From:       "old.example.com",
			To:         "https://new.example.com",
			KeepURI:    true,
			StatusCode: 308,
		},
	}
	s.NoError(s.vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	s.NoError(err)

	s.Contains(string(content), "$host")
	s.Contains(string(content), "old.example.com")
	s.Contains(string(content), "return 308")
	s.Contains(string(content), "$request_uri")
}

func (s *VhostTestSuite) TestRedirect404() {
	redirects := []types.Redirect{
		{
			Type:       types.RedirectType404,
			To:         "/custom-404.html",
			StatusCode: 308,
		},
	}
	s.NoError(s.vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	s.NoError(err)

	s.Contains(string(content), "error_page 404")
	s.Contains(string(content), "@redirect_404")
}

// ProxyVhost 测试套件
type ProxyVhostTestSuite struct {
	suite.Suite
	vhost     *ProxyVhost
	configDir string
}

func TestProxyVhostTestSuite(t *testing.T) {
	suite.Run(t, &ProxyVhostTestSuite{})
}

func (s *ProxyVhostTestSuite) SetupTest() {
	configDir, err := os.MkdirTemp("", "nginx-proxy-test-*")
	s.Require().NoError(err)
	s.configDir = configDir

	// 创建 site 和 shared 目录
	s.NoError(os.MkdirAll(filepath.Join(configDir, "site"), 0755))
	s.NoError(os.MkdirAll(filepath.Join(configDir, "shared"), 0755))

	vhost, err := NewProxyVhost(configDir)
	s.Require().NoError(err)
	s.vhost = vhost
}

func (s *ProxyVhostTestSuite) TearDownTest() {
	if s.configDir != "" {
		s.NoError(os.RemoveAll(s.configDir))
	}
}

func (s *ProxyVhostTestSuite) TestProxies() {
	// 初始应该没有代理配置
	s.Empty(s.vhost.Proxies())

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
	s.NoError(s.vhost.SetProxies(proxies))

	// 验证代理文件已创建
	siteDir := filepath.Join(s.configDir, "site")
	entries, err := os.ReadDir(siteDir)
	s.NoError(err)

	proxyCount := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "2") && strings.HasSuffix(entry.Name(), "-proxy.conf") {
			proxyCount++
		}
	}
	s.Equal(2, proxyCount)

	// 验证可以读取回来
	got := s.vhost.Proxies()
	s.Len(got, 2)
}

func (s *ProxyVhostTestSuite) TestProxyConfig() {
	proxies := []types.Proxy{
		{
			Location:  "/",
			Pass:      "https://backend",
			Host:      "example.com",
			SNI:       "example.com",
			Buffering: true,
		},
	}
	s.NoError(s.vhost.SetProxies(proxies))

	// 读取配置文件内容
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	s.NoError(err)

	s.Contains(string(content), "location /")
	s.Contains(string(content), "proxy_pass https://backend")
	s.Contains(string(content), "proxy_set_header Host")
	s.Contains(string(content), "example.com")
	s.Contains(string(content), "proxy_ssl_name")
	s.Contains(string(content), "proxy_buffering on")
}

func (s *ProxyVhostTestSuite) TestClearProxies() {
	proxies := []types.Proxy{
		{Location: "/", Pass: "http://backend"},
	}
	s.NoError(s.vhost.SetProxies(proxies))
	s.Len(s.vhost.Proxies(), 1)

	s.NoError(s.vhost.ClearProxies())
	s.Empty(s.vhost.Proxies())
}

func (s *ProxyVhostTestSuite) TestUpstreams() {
	// 初始应该没有上游服务器配置
	s.Empty(s.vhost.Upstreams())

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
	s.NoError(s.vhost.SetUpstreams(upstreams))

	// 验证 upstream 文件已创建
	sharedDir := filepath.Join(s.configDir, "shared")
	entries, err := os.ReadDir(sharedDir)
	s.NoError(err)
	s.NotEmpty(entries)

	// 验证可以读取回来
	got := s.vhost.Upstreams()
	s.Len(got, 1)
	s.Equal("backend", got[0].Name)
	s.Equal("least_conn", got[0].Algo)
	s.Equal(32, got[0].Keepalive)
}

func (s *ProxyVhostTestSuite) TestUpstreamConfig() {
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
	s.NoError(s.vhost.SetUpstreams(upstreams))

	// 读取配置文件内容
	sharedDir := filepath.Join(s.configDir, "shared")
	entries, err := os.ReadDir(sharedDir)
	s.NoError(err)
	s.Require().NotEmpty(entries)

	content, err := os.ReadFile(filepath.Join(sharedDir, entries[0].Name()))
	s.NoError(err)

	s.Contains(string(content), "upstream mybackend")
	s.Contains(string(content), "ip_hash")
	s.Contains(string(content), "server 127.0.0.1:8080")
	s.Contains(string(content), "weight=5")
	s.Contains(string(content), "keepalive 16")
}

func (s *ProxyVhostTestSuite) TestClearUpstreams() {
	upstreams := []types.Upstream{
		{
			Name:    "backend",
			Servers: map[string]string{"127.0.0.1:8080": ""},
		},
	}
	s.NoError(s.vhost.SetUpstreams(upstreams))
	s.Len(s.vhost.Upstreams(), 1)

	s.NoError(s.vhost.ClearUpstreams())
	s.Empty(s.vhost.Upstreams())
}

func (s *ProxyVhostTestSuite) TestProxyWithUpstream() {
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
	s.NoError(s.vhost.SetUpstreams(upstreams))

	// 然后创建引用 upstream 的 proxy
	proxies := []types.Proxy{
		{
			Location: "/api",
			Pass:     "http://api-servers",
		},
	}
	s.NoError(s.vhost.SetProxies(proxies))

	// 验证两者都存在
	s.Len(s.vhost.Upstreams(), 1)
	s.Len(s.vhost.Proxies(), 1)

	// 验证 proxy 配置中引用了 upstream
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	s.NoError(err)
	s.Contains(string(content), "http://api-servers")
}

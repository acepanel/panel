package apache

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
	configDir, err := os.MkdirTemp("", "apache-test-*")
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
	s.NotNil(s.vhost.config)
	s.NotNil(s.vhost.vhost)
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
	s.Empty(s.vhost.Index())
}

func (s *VhostTestSuite) TestListen() {
	listens := []types.Listen{
		{Address: "*:80"},
		{Address: "*:443"},
	}
	s.NoError(s.vhost.SetListen(listens))

	got := s.vhost.Listen()
	s.Len(got, 2)
	s.Equal("*:80", got[0].Address)
	s.Equal("*:443", got[1].Address)
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
	s.Equal(sslConfig.Cert, got.Cert)
	s.Equal(sslConfig.Key, got.Key)
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

func (s *VhostTestSuite) TestClearHTTPSPreservesOtherHeaders() {
	// 添加一个非 HSTS 的 Header
	s.vhost.vhost.AddDirective("Header", "set", "X-Custom-Header", "value")

	// 设置 SSL 和 HSTS
	sslConfig := &types.SSLConfig{
		Cert: "/etc/ssl/cert.pem",
		Key:  "/etc/ssl/key.pem",
		HSTS: true,
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))

	// 清除 HTTPS
	s.NoError(s.vhost.ClearSSL())

	// 检查自定义 Header 是否保留
	headers := s.vhost.vhost.GetDirectives("Header")
	s.NotEmpty(headers)
	found := false
	for _, h := range headers {
		if len(h.Args) >= 2 && h.Args[1] == "X-Custom-Header" {
			found = true
			break
		}
	}
	s.True(found, "自定义 Header 应该被保留")
}

func (s *VhostTestSuite) TestPHP() {
	s.Equal(uint(0), s.vhost.PHP())

	s.NoError(s.vhost.SetPHP(84))
	s.NotEqual(uint(0), s.vhost.PHP())

	s.NoError(s.vhost.SetPHP(0))
	s.Equal(uint(0), s.vhost.PHP())
}

func (s *VhostTestSuite) TestAccessLog() {
	accessLog := "/var/log/apache/access.log"
	s.NoError(s.vhost.SetAccessLog(accessLog))
	s.Equal(accessLog, s.vhost.AccessLog())
}

func (s *VhostTestSuite) TestErrorLog() {
	errorLog := "/var/log/apache/error.log"
	s.NoError(s.vhost.SetErrorLog(errorLog))
	s.Equal(errorLog, s.vhost.ErrorLog())
}

func (s *VhostTestSuite) TestIncludes() {
	includes := []types.IncludeFile{
		{Path: "/etc/apache/conf.d/ssl.conf"},
		{Path: "/etc/apache/conf.d/php.conf"},
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
		"user_file": "/etc/htpasswd",
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
		Rate: 512,
	}
	s.NoError(s.vhost.SetRateLimit(limit))

	got := s.vhost.RateLimit()
	s.NotNil(got)
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
	s.NoError(s.vhost.SetServerName([]string{"save-test.com"}))
	s.NoError(s.vhost.Save())

	// 验证配置文件已保存
	configFile := filepath.Join(s.configDir, "apache.conf")
	content, err := os.ReadFile(configFile)
	s.NoError(err)
	s.Contains(string(content), "save-test.com")
}

func (s *VhostTestSuite) TestExport() {
	s.NoError(s.vhost.SetServerName([]string{"export-test.com"}))
	s.NoError(s.vhost.SetRoot("/var/www/export-test"))

	content := s.vhost.config.Export()
	s.NotEmpty(content)
	s.Contains(content, "export-test.com")
	s.Contains(content, "/var/www/export-test")
	s.Contains(content, "<VirtualHost")
	s.Contains(content, "</VirtualHost>")
}

func (s *VhostTestSuite) TestExportWithSSL() {
	sslConfig := &types.SSLConfig{
		Cert:      "/etc/ssl/cert.pem",
		Key:       "/etc/ssl/key.pem",
		Protocols: []string{"TLSv1.2", "TLSv1.3"},
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))

	content := s.vhost.config.Export()
	s.Contains(content, "SSLEngine on")
	s.Contains(content, "SSLCertificateFile")
	s.Contains(content, "SSLCertificateKeyFile")
}

func (s *VhostTestSuite) TestListenProtocolDetection() {
	listens := []types.Listen{
		{Address: "*:443"},
	}
	s.NoError(s.vhost.SetListen(listens))

	sslConfig := &types.SSLConfig{
		Cert: "/etc/ssl/cert.pem",
		Key:  "/etc/ssl/key.pem",
	}
	s.NoError(s.vhost.SetSSLConfig(sslConfig))

	got := s.vhost.Listen()
	s.Len(got, 1)
	s.Equal("*:443", got[0].Address)
}

func (s *VhostTestSuite) TestDirectoryBlock() {
	root := "/var/www/test-dir"
	s.NoError(s.vhost.SetRoot(root))

	content := s.vhost.config.Export()
	s.Contains(content, "<Directory "+root+">")
	s.Contains(content, "</Directory>")
}

func (s *VhostTestSuite) TestPHPFilesMatchBlock() {
	s.NoError(s.vhost.SetPHP(84))

	content := s.vhost.Config("010-php.conf", "site")
	s.Contains(content, "proxy:unix:/tmp/php-cgi-84.sock|fcgi://localhost/")
}

func (s *VhostTestSuite) TestDefaultVhostConfIncludesServerD() {
	// 验证默认配置包含 site 的 include
	s.Contains(DefaultVhostConf, "site")
	s.Contains(DefaultVhostConf, "IncludeOptional")
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

	s.Contains(string(content), "Redirect 301")
	s.Contains(string(content), "/old-page")
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

	s.Contains(string(content), "RewriteEngine")
	s.Contains(string(content), "RewriteCond")
	s.Contains(string(content), "old.example.com")
	s.Contains(string(content), "R=308")
}

func (s *VhostTestSuite) TestRedirect404() {
	redirects := []types.Redirect{
		{
			Type: types.RedirectType404,
			To:   "/custom-404.html",
		},
	}
	s.NoError(s.vhost.SetRedirects(redirects))

	// 读取配置文件内容
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "100-redirect.conf"))
	s.NoError(err)

	s.Contains(string(content), "ErrorDocument 404")
	s.Contains(string(content), "/custom-404.html")
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
	configDir, err := os.MkdirTemp("", "apache-proxy-test-*")
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
			Pass:     "http://backend:8080/",
			Host:     "example.com",
		},
		{
			Location:  "/api",
			Pass:      "http://api-backend:8080/",
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
			Pass:      "http://backend:8080/",
			Host:      "example.com",
			Buffering: true,
		},
	}
	s.NoError(s.vhost.SetProxies(proxies))

	// 读取配置文件内容
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	s.NoError(err)

	s.Contains(string(content), "ProxyPass /")
	s.Contains(string(content), "ProxyPassReverse")
	s.Contains(string(content), "http://backend:8080/")
	s.Contains(string(content), "RequestHeader set Host")
	s.Contains(string(content), "example.com")
}

func (s *ProxyVhostTestSuite) TestClearProxies() {
	proxies := []types.Proxy{
		{Location: "/", Pass: "http://backend/"},
	}
	s.NoError(s.vhost.SetProxies(proxies))
	s.Len(s.vhost.Proxies(), 1)

	s.NoError(s.vhost.ClearProxies())
	s.Empty(s.vhost.Proxies())
}

func (s *ProxyVhostTestSuite) TestUpstreams() {
	// 初始应该没有上游服务器配置
	s.Empty(s.vhost.Upstreams())

	// 设置上游服务器（Apache 使用 balancer）
	upstreams := []types.Upstream{
		{
			Name: "backend",
			Servers: map[string]string{
				"http://127.0.0.1:8080": "loadfactor=5",
				"http://127.0.0.1:8081": "loadfactor=3",
			},
			Algo:      "bybusyness",
			Keepalive: 32,
		},
	}
	s.NoError(s.vhost.SetUpstreams(upstreams))

	// 验证 balancer 文件已创建
	sharedDir := filepath.Join(s.configDir, "shared")
	entries, err := os.ReadDir(sharedDir)
	s.NoError(err)
	s.NotEmpty(entries)

	// 验证可以读取回来
	got := s.vhost.Upstreams()
	s.Len(got, 1)
	s.Equal("backend", got[0].Name)
}

func (s *ProxyVhostTestSuite) TestBalancerConfig() {
	upstreams := []types.Upstream{
		{
			Name: "mybackend",
			Servers: map[string]string{
				"http://127.0.0.1:8080": "loadfactor=5",
			},
			Algo:      "bybusyness",
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

	s.Contains(string(content), "balancer://mybackend")
	s.Contains(string(content), "BalancerMember")
	s.Contains(string(content), "http://127.0.0.1:8080")
	s.Contains(string(content), "lbmethod=bybusyness")
}

func (s *ProxyVhostTestSuite) TestClearUpstreams() {
	upstreams := []types.Upstream{
		{
			Name:    "backend",
			Servers: map[string]string{"http://127.0.0.1:8080": ""},
		},
	}
	s.NoError(s.vhost.SetUpstreams(upstreams))
	s.Len(s.vhost.Upstreams(), 1)

	s.NoError(s.vhost.ClearUpstreams())
	s.Empty(s.vhost.Upstreams())
}

func (s *ProxyVhostTestSuite) TestProxySNI() {
	// 测试 SNI 配置的写入和解析
	proxies := []types.Proxy{
		{
			Location: "/",
			Pass:     "https://backend:443/",
			SNI:      "backend.example.com",
		},
	}
	s.NoError(s.vhost.SetProxies(proxies))

	// 读取配置文件内容，验证 SNI 已写入
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	s.NoError(err)

	s.Contains(string(content), "SSLProxyEngine On")
	s.Contains(string(content), "# SNI: backend.example.com")

	// 验证可以解析回来
	got := s.vhost.Proxies()
	s.Require().Len(got, 1)
	s.Equal("backend.example.com", got[0].SNI)
}

func (s *ProxyVhostTestSuite) TestProxySubstitute() {
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
	s.NoError(s.vhost.SetProxies(proxies))

	// 读取配置文件内容，验证 Substitute 已写入
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	s.NoError(err)

	s.Contains(string(content), "mod_substitute")
	s.Contains(string(content), "Substitute")

	// 验证可以解析回来
	got := s.vhost.Proxies()
	s.Require().Len(got, 1)
	s.Require().NotNil(got[0].Replaces)
	s.Equal("https://new.example.com", got[0].Replaces["http://old.example.com"])
	s.Equal("bar", got[0].Replaces["foo"])
}

func (s *ProxyVhostTestSuite) TestProxySubstituteWithSlash() {
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
	s.NoError(s.vhost.SetProxies(proxies))

	// 读取配置文件内容
	siteDir := filepath.Join(s.configDir, "site")
	content, err := os.ReadFile(filepath.Join(siteDir, "200-proxy.conf"))
	s.NoError(err)

	// 验证使用 | 作为分隔符
	s.Contains(string(content), "Substitute \"s|http://old.example.com/path/to/resource|https://new.example.com/new/path|n\"")

	// 验证可以解析回来
	got := s.vhost.Proxies()
	s.Require().Len(got, 1)
	s.Require().NotNil(got[0].Replaces)
	s.Equal("https://new.example.com/new/path", got[0].Replaces["http://old.example.com/path/to/resource"])
}

func (s *ProxyVhostTestSuite) TestUpstreamMultipleServers() {
	// 测试多个 BalancerMember 的解析
	upstreams := []types.Upstream{
		{
			Name: "test_upstream",
			Servers: map[string]string{
				"127.0.0.1:8080": "",
				"127.0.0.1:8081": "",
				"127.0.0.1:8082": "loadfactor=5",
			},
			Keepalive: 32,
		},
	}
	s.NoError(s.vhost.SetUpstreams(upstreams))

	// 验证可以解析回来
	got := s.vhost.Upstreams()
	s.Require().Len(got, 1)
	s.Equal("test_upstream", got[0].Name)
	s.Len(got[0].Servers, 3)
	s.Contains(got[0].Servers, "127.0.0.1:8080")
	s.Contains(got[0].Servers, "127.0.0.1:8081")
	s.Contains(got[0].Servers, "127.0.0.1:8082")
	s.Equal("loadfactor=5", got[0].Servers["127.0.0.1:8082"])
}

package apache

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/internal/golden"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 用真实 httpd 校验生成的配置
func TestValidateWithApache(t *testing.T) {
	bin := os.Getenv("APACHE_BIN")
	if bin == "" {
		t.Skip("APACHE_BIN not set")
	}

	root := t.TempDir()
	if smokeRoot := os.Getenv("APACHE_SMOKE_DIR"); smokeRoot != "" {
		must.NoError(t, os.RemoveAll(smokeRoot))
		must.NoError(t, os.MkdirAll(smokeRoot, 0755))
		root = smokeRoot
	}
	sites := filepath.Join(root, "sites")
	certPath, keyPath := writeSelfSigned(t, root)

	newSite := func(name string) string {
		configDir := filepath.Join(sites, name, "config")
		for _, dir := range []string{"site", "shared"} {
			must.NoError(t, os.MkdirAll(filepath.Join(configDir, dir), 0755))
		}
		must.NoError(t, os.MkdirAll(filepath.Join(sites, name, "public"), 0755))
		must.NoError(t, os.MkdirAll(filepath.Join(sites, name, "log"), 0755))
		return configDir
	}
	d := Dialect{}
	// 默认模板里的 include 指向 /opt/ace，改写到临时目录才能把 site/shared 片段一起校验
	save := func(v interface{ Save() error }, configDir string) {
		must.NoError(t, v.Save())
		path := filepath.Join(configDir, "apache.conf")
		name := filepath.Base(filepath.Dir(configDir))
		content := strings.ReplaceAll(readFile(t, path), filepath.Join(sitesPath, name, "config"), configDir)
		must.NoError(t, os.WriteFile(path, []byte(content), 0600))
	}

	phpDir := newSite("php")
	php, err := newPHPVhost(phpDir)
	must.NoError(t, err)
	must.NoError(t, php.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl"}}}))
	must.NoError(t, php.SetServerName([]string{"php.example.com", "www.php.example.com"}))
	must.NoError(t, php.SetRoot(filepath.Join(sites, "php", "public")))
	must.NoError(t, php.SetIndex([]string{"index.php", "index.html"}))
	must.NoError(t, php.SetAccessLog(filepath.Join(sites, "php", "log", "access.log")))
	must.NoError(t, php.SetPHP(84))
	must.NoError(t, php.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath, Protocols: []string{"TLSv1.1", "TLSv1.2"}, HSTS: true, HTTPRedirect: true}))
	htpasswd := filepath.Join(sites, "php", "htpasswd_0")
	must.NoError(t, os.WriteFile(htpasswd, []byte(d.HTPasswdLine("admin", "secret")+"\n"), 0644))
	must.NoError(t, php.SetBasicAuth([]types.BasicAuth{{Path: "/", UserFile: htpasswd}, {Path: "/admin", UserFile: htpasswd}}))
	must.NoError(t, php.SetRedirects([]types.Redirect{
		{Type: types.RedirectTypeHost, From: "old.example.com", To: "https://php.example.com", KeepURI: true, StatusCode: 301},
		{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 302},
		{Type: types.RedirectType404, To: "https://php.example.com", KeepURI: true, StatusCode: 302},
	}))
	must.NoError(t, php.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	must.NoError(t, php.SetConfig("010-cache.conf", types.ScopeSite, d.PHPCacheConf()))
	save(php, phpDir)

	presets, _ := filepath.Glob(filepath.Join("..", "..", "embed", "rewrites", d.RewritesDir(), "*.conf"))
	must.NotEmpty(t, presets)
	for _, preset := range presets {
		name := strings.TrimSuffix(filepath.Base(preset), ".conf")
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(preset)
			must.NoError(t, err)
			presetDir := newSite("preset-" + name)
			site, err := newPHPVhost(presetDir)
			must.NoError(t, err)
			must.NoError(t, site.SetServerName([]string{name + ".preset.test"}))
			must.NoError(t, site.SetRoot(filepath.Join(sites, "preset-"+name, "public")))
			must.NoError(t, site.SetPHP(84))
			must.NoError(t, site.SetRawConfig("010-rewrite.conf", types.ScopeSite, string(content)))
			save(site, presetDir)
		})
	}

	proxyDir := newSite("proxy")
	proxy, err := newProxyVhost(proxyDir)
	must.NoError(t, err)
	must.NoError(t, proxy.SetListen([]types.Listen{{Address: "80"}, {Address: "8443", Args: []string{"ssl"}}}))
	must.NoError(t, proxy.SetServerName([]string{"proxy.example.com"}))
	must.NoError(t, proxy.SetRoot(filepath.Join(sites, "proxy", "public")))
	must.NoError(t, proxy.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath, Protocols: []string{"TLSv1.2"}}))
	must.NoError(t, proxy.SetUpstreams([]types.Upstream{
		{Name: "backend", Servers: map[string]string{"http://127.0.0.1:3001": "weight=5", "http://127.0.0.1:3002": ""}},
		{Name: "pool", Servers: map[string]string{"http://10.0.0.2:80": "", "unix:/tmp/a.sock": "", "unix:/tmp/b.sock": ""}, Algo: "least_conn"},
	}))
	must.NoError(t, proxy.SetProxies([]types.Proxy{
		{Location: "/", Pass: "http://backend", Host: "$proxy_host", Buffering: true, Headers: map[string]string{"X-Real-IP": "$remote_addr"}, Replaces: map[string]string{"http://old": "https://new"}},
		{
			Location: "^~ /api/", Pass: "https://10.0.0.5/v2/", SNI: "api.internal", HTTPVersion: "2",
			Timeout: &types.TimeoutConfig{Connect: 5 * time.Second, Read: 90 * time.Second},
			Retry:   &types.RetryConfig{Tries: 3, Timeout: 10 * time.Second}, ClientMaxBodySize: 10485760,
			SSLBackend:      &types.SSLBackendConfig{Verify: true, TrustedCertificate: certPath},
			ResponseHeaders: &types.ResponseHeaderConfig{Hide: []string{"Server"}, Add: map[string]string{"X-Proxy": "apache"}},
			AccessControl:   &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
		},
		{Location: "~* \\.(jpg|png)$", Pass: "http://pool", Buffering: true, Cache: &types.CacheConfig{Valid: map[string]string{"200": "1h"}}},
		{Location: "= /health", Pass: "http://127.0.0.1:9000", Buffering: true},
		{Location: "/ws", Pass: "https://backend.example.com", Buffering: false},
		{Location: "/sock/", Pass: "http://unix:/tmp/a.sock:/v2/", Cache: &types.CacheConfig{}},
	}))
	must.NoError(t, proxy.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	save(proxy, proxyDir)

	staticDir := newSite("static")
	static, err := newStaticVhost(staticDir)
	must.NoError(t, err)
	must.NoError(t, static.SetListen([]types.Listen{{Address: "127.0.0.1:80"}, {Address: "127.0.0.1:443", Args: []string{"ssl"}}}))
	must.NoError(t, static.SetServerName([]string{"static.example.com"}))
	must.NoError(t, static.SetRoot(filepath.Join(sites, "static", "public")))
	must.NoError(t, static.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath, Protocols: []string{"TLSv1.2"}}))
	must.NoError(t, static.SetRawConfig("800-spa.conf", types.ScopeSite, d.SPAConf()))
	must.NoError(t, static.SetEnable(false))
	save(static, staticDir)

	// 冒烟站点用高位端口，可在本机启动后用 curl 验证
	smokeDir := newSite("smoke")
	smoke, err := newProxyVhost(smokeDir)
	must.NoError(t, err)
	must.NoError(t, smoke.SetListen([]types.Listen{{Address: "18080"}, {Address: "18443", Args: []string{"ssl"}}}))
	must.NoError(t, smoke.SetServerName([]string{"localhost"}))
	must.NoError(t, smoke.SetRoot(filepath.Join(sites, "smoke", "public")))
	must.NoError(t, smoke.SetAccessLog(filepath.Join(sites, "smoke", "log", "access.log")))
	must.NoError(t, smoke.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath, Protocols: []string{"TLSv1.2"}, HSTS: true, HTTPRedirect: true}))
	must.NoError(t, smoke.SetUpstreams([]types.Upstream{{Name: "smokeup", Servers: map[string]string{"127.0.0.1:13011": ""}}}))
	must.NoError(t, smoke.SetProxies([]types.Proxy{
		{Location: "/api/", Pass: "http://127.0.0.1:13011/v2/"},
		{Location: "= /health", Pass: "http://127.0.0.1:13011"},
		{Location: "~* \\.(jpg|png)$", Pass: "http://127.0.0.1:13011"},
		{Location: "/bal", Pass: "http://smokeup"},
	}))
	must.NoError(t, smoke.SetBasicAuth([]types.BasicAuth{{Path: "/admin", UserFile: htpasswd}}))
	save(smoke, smokeDir)

	// 快照测试的全功能站点也交给真实 httpd 校验；macOS 自带 httpd 的 mod_ssl 不认 TLSv1.3
	for _, site := range golden.Build(t, d, root, "golden-", certPath, keyPath, goldenCustom) {
		path := filepath.Join(site.ConfigDir, "apache.conf")
		content := strings.NewReplacer(filepath.Join(sitesPath, site.Name, "config"), site.ConfigDir, "+TLSv1.3", "+TLSv1.2").Replace(readFile(t, path))
		must.NoError(t, os.WriteFile(path, []byte(content), 0600))
	}

	mainPath := filepath.Join(root, "httpd.conf")
	must.NoError(t, os.WriteFile(mainPath, []byte(mainConf(t, root, sites, certPath, keyPath)), 0600))

	out, err := exec.Command(bin, "-t", "-f", mainPath).CombinedOutput()
	must.NoError(t, err, must.Msgf("%s\n---- php ----\n%s\n---- proxy ----\n%s\n---- proxy rules ----\n%s\n---- balancers ----\n%s",
		out, readFile(t, filepath.Join(phpDir, "apache.conf")), readFile(t, filepath.Join(proxyDir, "apache.conf")),
		readDir(t, filepath.Join(proxyDir, "site")), readDir(t, filepath.Join(proxyDir, "shared"))))
}

// mainConf 最小主配置，模块路径与本机 httpd 一致
func mainConf(t *testing.T, root, sites, cert, key string) string {
	t.Helper()
	modDir := os.Getenv("APACHE_MOD_DIR")
	if modDir == "" {
		modDir = "/usr/libexec/apache2"
	}
	var mods strings.Builder
	for _, m := range []string{
		"mpm_event_module:mod_mpm_event", "authn_file_module:mod_authn_file", "authn_core_module:mod_authn_core",
		"authz_host_module:mod_authz_host", "authz_user_module:mod_authz_user", "authz_core_module:mod_authz_core",
		"auth_basic_module:mod_auth_basic", "access_compat_module:mod_access_compat", "socache_shmcb_module:mod_socache_shmcb",
		"reqtimeout_module:mod_reqtimeout", "filter_module:mod_filter", "substitute_module:mod_substitute",
		"mime_module:mod_mime", "log_config_module:mod_log_config", "env_module:mod_env", "headers_module:mod_headers",
		"setenvif_module:mod_setenvif", "version_module:mod_version", "ssl_module:mod_ssl", "proxy_module:mod_proxy",
		"proxy_http_module:mod_proxy_http", "proxy_fcgi_module:mod_proxy_fcgi", "proxy_balancer_module:mod_proxy_balancer",
		"proxy_wstunnel_module:mod_proxy_wstunnel", "lbmethod_byrequests_module:mod_lbmethod_byrequests",
		"lbmethod_bytraffic_module:mod_lbmethod_bytraffic", "lbmethod_bybusyness_module:mod_lbmethod_bybusyness",
		"slotmem_shm_module:mod_slotmem_shm", "cache_module:mod_cache", "cache_disk_module:mod_cache_disk",
		"expires_module:mod_expires", "rewrite_module:mod_rewrite", "dir_module:mod_dir", "alias_module:mod_alias",
		"unixd_module:mod_unixd", "ratelimit_module:mod_ratelimit", "remoteip_module:mod_remoteip",
	} {
		name, file, _ := strings.Cut(m, ":")
		_, _ = fmt.Fprintf(&mods, "LoadModule %s %s\n", name, filepath.Join(modDir, file+".so"))
	}

	return strings.NewReplacer("{mods}", mods.String(), "{root}", root, "{sites}", sites, "{cert}", cert, "{key}", key).Replace(`ServerRoot {root}
ServerName localhost
PidFile {root}/httpd.pid
Listen 80
Listen 443
Listen 8443
Listen 18080
Listen 18443
{mods}
User nobody
Group nobody
DocumentRoot {root}
ErrorLog {root}/error.log
LogFormat "%h %l %u %t \"%r\" %>s %b \"%{Referer}i\" \"%{User-Agent}i\"" combined
SSLCertificateFile {cert}
SSLCertificateKeyFile {key}
SSLStaplingCache "shmcb:{root}/ssl_stapling(32768)"
CacheRoot {root}
CacheQuickHandler off
<Directory />
    AllowOverride none
    Require all denied
</Directory>

IncludeOptional {sites}/*/config/*.conf
`)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	must.NoError(t, err)
	return string(content)
}

func readDir(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	must.NoError(t, err)
	var b strings.Builder
	for _, e := range entries {
		_, _ = fmt.Fprintf(&b, "== %s ==\n%s\n", e.Name(), readFile(t, filepath.Join(dir, e.Name())))
	}
	return b.String()
}

func writeSelfSigned(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	must.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost", "php.example.com", "proxy.example.com", "static.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	must.NoError(t, err)
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	must.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	must.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	return certPath, keyPath
}

package caddy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 用真实 caddy 二进制校验生成的配置
func TestValidateWithCaddy(t *testing.T) {
	bin := os.Getenv("CADDY_BIN")
	if bin == "" {
		t.Skip("CADDY_BIN not set")
	}

	root := t.TempDir()
	// 指定目录时布局保留下来，便于本机启动 caddy 冒烟测试
	if smokeRoot := os.Getenv("CADDY_SMOKE_DIR"); smokeRoot != "" {
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

	phpDir := newSite("php")
	php, err := NewPHPVhost(phpDir)
	must.NoError(t, err)
	must.NoError(t, php.SetListen([]types.Listen{{Address: "80"}, {Address: "443", Args: []string{"ssl"}}}))
	must.NoError(t, php.SetServerName([]string{"php.example.com", "www.php.example.com"}))
	must.NoError(t, php.SetRoot(filepath.Join(sites, "php", "public")))
	must.NoError(t, php.SetIndex([]string{"index.php", "index.html"}))
	must.NoError(t, php.SetAccessLog(filepath.Join(sites, "php", "log", "access.log")))
	must.NoError(t, php.SetPHP(84))
	must.NoError(t, php.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath, Protocols: []string{"TLSv1.2", "TLSv1.3"}, HSTS: true, HTTPRedirect: true}))
	htpasswd := filepath.Join(sites, "php", "htpasswd_0")
	must.NoError(t, os.WriteFile(htpasswd, []byte(d.HTPasswdLine("admin", "secret")+"\n"), 0644))
	must.NoError(t, php.SetBasicAuth([]types.BasicAuth{{Path: "/", UserFile: htpasswd}, {Path: "/admin", UserFile: htpasswd}}))
	must.NoError(t, php.SetRedirects([]types.Redirect{
		{Type: types.RedirectTypeHost, From: "old.example.com", To: "https://php.example.com", KeepURI: true, StatusCode: 301},
		{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 302},
		{Type: types.RedirectType404, To: "/", StatusCode: 308},
	}))
	must.NoError(t, php.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	must.NoError(t, php.SetConfig("010-cache.conf", types.ScopeSite, d.PHPCacheConf()))
	must.NoError(t, php.Save())

	// 预置之间匹配器名可能重复，真实站点一次只用一个，这里各占一个站点
	presets, _ := filepath.Glob(filepath.Join("..", "..", "embed", "rewrites", "caddy", "*.conf"))
	must.NotEmpty(t, presets)
	for _, preset := range presets {
		name := strings.TrimSuffix(filepath.Base(preset), ".conf")
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(preset)
			must.NoError(t, err)
			site, err := NewPHPVhost(newSite("preset-" + name))
			must.NoError(t, err)
			must.NoError(t, site.SetServerName([]string{name + ".preset.test"}))
			must.NoError(t, site.SetRoot(filepath.Join(sites, "preset-"+name, "public")))
			must.NoError(t, site.SetPHP(84))
			must.NoError(t, site.SetRawConfig("010-rewrite.conf", types.ScopeSite, string(content)))
			must.NoError(t, site.Save())
		})
	}

	proxyDir := newSite("proxy")
	proxy, err := NewProxyVhost(proxyDir)
	must.NoError(t, err)
	must.NoError(t, proxy.SetListen([]types.Listen{{Address: "80"}, {Address: "8443", Args: []string{"ssl"}}}))
	must.NoError(t, proxy.SetServerName([]string{"proxy.example.com"}))
	must.NoError(t, proxy.SetRoot(filepath.Join(sites, "proxy", "public")))
	must.NoError(t, proxy.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath, Protocols: []string{"TLSv1.3"}}))
	must.NoError(t, proxy.SetUpstreams([]types.Upstream{
		{Name: "backend", Servers: map[string]string{"127.0.0.1:3001": "weight=5", "127.0.0.1:3002": ""}},
		{Name: "pool", Servers: map[string]string{"unix:/tmp/api.sock": "", "10.0.0.2:80": ""}, Algo: "least_conn"},
	}))
	must.NoError(t, proxy.SetProxies([]types.Proxy{
		{Location: "/", Pass: "http://backend", Host: "$proxy_host", Buffering: true, Headers: map[string]string{"X-Real-IP": "$remote_addr"}, Replaces: map[string]string{"http://old": "https://new"}},
		{
			Location: "^~ /api/", Pass: "https://10.0.0.5/v2/", SNI: "api.internal", HTTPVersion: "2",
			Timeout: &types.TimeoutConfig{Connect: 5 * time.Second, Read: 90 * time.Second},
			Retry:   &types.RetryConfig{Tries: 3, Timeout: 10 * time.Second}, ClientMaxBodySize: 10485760,
			SSLBackend:      &types.SSLBackendConfig{Verify: true, TrustedCertificate: certPath},
			ResponseHeaders: &types.ResponseHeaderConfig{Hide: []string{"Server"}, Add: map[string]string{"X-Proxy": "caddy"}},
			AccessControl:   &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
		},
		{Location: "~* \\.(jpg|png)$", Pass: "http://pool", Buffering: true},
		{Location: "= /health", Pass: "http://unix:/tmp/app.sock", Buffering: true},
		{Location: "/ws", Pass: "https://backend.example.com", Buffering: false},
	}))
	must.NoError(t, proxy.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	must.NoError(t, proxy.Save())

	staticDir := newSite("static")
	static, err := NewStaticVhost(staticDir)
	must.NoError(t, err)
	must.NoError(t, static.SetListen([]types.Listen{{Address: "127.0.0.1:80"}, {Address: "127.0.0.1:443", Args: []string{"ssl"}}}))
	must.NoError(t, static.SetServerName([]string{"static.example.com"}))
	must.NoError(t, static.SetRoot(filepath.Join(sites, "static", "public")))
	must.NoError(t, static.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath}))
	must.NoError(t, static.SetRawConfig("800-spa.conf", types.ScopeSite, d.SPAConf()))
	must.NoError(t, static.SetEnable(false))
	must.NoError(t, static.Save())

	// 无域名站点
	pmaDir := newSite("phpmyadmin")
	must.NoError(t, os.WriteFile(filepath.Join(pmaDir, ConfName), []byte(":888 {\n\troot * "+filepath.Join(sites, "phpmyadmin", "public")+"\n\tphp_fastcgi unix//tmp/php-cgi-83.sock\n\tfile_server {\n\t\tindex index.php\n\t}\n}\n"), 0600))

	main := strings.NewReplacer("{root}", root, "{sites}", sites, "{cert}", certPath, "{key}", keyPath).Replace(`{
	admin unix/{root}/admin.sock
	auto_https off
	persist_config off
	grace_period 10s
	default_sni localhost
	fallback_sni localhost
	storage file_system {
		root {root}/data
	}
	log {
		output file {root}/error.log
		exclude http.log.access
	}
}

import {sites}/*/config/caddy.conf

:80 {
	root * {root}/html
	handle /.well-known/acme-challenge/* {
		root * {root}/acme
		file_server
	}
	file_server
}

:443 {
	tls {cert} {key}
	root * {root}/html
	file_server
}
`)
	// 冒烟站点用高位端口，可在本机启动后用 curl 验证
	smokeDir := newSite("smoke")
	smoke, err := NewStaticVhost(smokeDir)
	must.NoError(t, err)
	must.NoError(t, smoke.SetListen([]types.Listen{{Address: "18080"}, {Address: "18443", Args: []string{"ssl"}}}))
	must.NoError(t, smoke.SetServerName([]string{"localhost"}))
	must.NoError(t, smoke.SetRoot(filepath.Join(sites, "smoke", "public")))
	must.NoError(t, smoke.SetAccessLog(filepath.Join(sites, "smoke", "log", "access.log")))
	must.NoError(t, smoke.SetSSLConfig(&types.SSLConfig{Cert: certPath, Key: keyPath, HSTS: true, HTTPRedirect: true}))
	must.NoError(t, smoke.SetBasicAuth([]types.BasicAuth{{Path: "/admin", UserFile: htpasswd}}))
	must.NoError(t, smoke.SetRedirects([]types.Redirect{{Type: types.RedirectTypeURL, From: "/old", To: "/new", StatusCode: 302}}))
	must.NoError(t, smoke.SetRawConfig("800-spa.conf", types.ScopeSite, d.SPAConf()))
	// 静态前端 + /api 后端，SPA 回退不能吞掉代理路径
	must.NoError(t, smoke.SetProxies([]types.Proxy{{Location: "^~ /api", Pass: "http://127.0.0.1:18090", Host: "$host"}}))
	must.NoError(t, smoke.SetConfig("010-error-404.conf", types.ScopeSite, d.ErrorPageConf()))
	_, stat := d.StatConf("smoke")
	must.NoError(t, smoke.SetConfig("021-stats-log.conf", types.ScopeSite, stat))
	must.NoError(t, smoke.SetDefault(true))
	must.NoError(t, smoke.Save())
	must.NoError(t, os.WriteFile(filepath.Join(sites, "smoke", "public", "index.html"), []byte("smoke index"), 0644))
	must.NoError(t, os.WriteFile(filepath.Join(sites, "smoke", "public", "404.html"), []byte("smoke 404"), 0644))
	must.NoError(t, os.MkdirAll(filepath.Join(sites, "smoke", "public", "admin"), 0755))
	must.NoError(t, os.WriteFile(filepath.Join(sites, "smoke", "public", "admin", "index.html"), []byte("admin area"), 0644))
	stoppedDir := newSite("stopped")
	stopped, err := NewStaticVhost(stoppedDir)
	must.NoError(t, err)
	must.NoError(t, stopped.SetListen([]types.Listen{{Address: "18081"}}))
	must.NoError(t, stopped.SetServerName([]string{"localhost"}))
	must.NoError(t, stopped.SetRoot(filepath.Join(sites, "stopped", "public")))
	must.NoError(t, stopped.SetEnable(false))
	must.NoError(t, stopped.Save())
	must.NoError(t, os.WriteFile(filepath.Join(sites, "stopped", "public", "index.html"), []byte("must not be served"), 0644))

	mainPath := filepath.Join(root, "Caddyfile")
	must.NoError(t, os.WriteFile(mainPath, []byte(main), 0600))

	out, err := exec.Command(bin, "validate", "--config", mainPath).CombinedOutput()
	must.NoError(t, err, must.Msgf("%s\n---- php ----\n%s\n---- proxy ----\n%s", out, readFile(t, filepath.Join(phpDir, ConfName)), readFile(t, filepath.Join(proxyDir, ConfName))))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	must.NoError(t, err)
	return string(content)
}

func writeSelfSigned(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	must.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost", "php.example.com", "www.php.example.com", "proxy.example.com", "static.example.com"},
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

package acme

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/libdns/alidns"
	"github.com/libdns/cloudflare"
	"github.com/libdns/cloudns"
	"github.com/libdns/gcore"
	"github.com/libdns/huaweicloud"
	"github.com/libdns/libdns"
	"github.com/libdns/namesilo"
	"github.com/libdns/porkbun"
	"github.com/libdns/tencentcloud"
	"github.com/libdns/westcn"
	"github.com/mholt/acmez/v3/acme"
	"golang.org/x/net/publicsuffix"

	pkgos "github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
)

var panelSolverGlobal sync.Mutex

type panelSolver struct {
	ip        []string
	conf      string
	webServer string // "nginx" or "apache"
	server    *http.Server
	// tokens 存储所有待验证的 challenge，key 为路径，value 为 token
	tokens map[string]string
	// presentCount Present 调用计数
	presentCount int
	// cleanupCount CleanUp 调用计数
	cleanupCount int
	// useBuiltin 标记是否使用内置 HTTP 服务器
	useBuiltin bool
}

func (s *panelSolver) Present(_ context.Context, challenge acme.Challenge) error {
	if s.presentCount == 0 {
		panelSolverGlobal.Lock()
	}

	path := challenge.HTTP01ResourcePath()
	token := challenge.KeyAuthorization

	// 初始化 tokens map
	if s.tokens == nil {
		s.tokens = make(map[string]string)
	}

	// 收集所有域名的 token
	s.tokens[path] = token
	s.presentCount++
	if s.presentCount < len(s.ip) {
		return nil
	}

	// 如果 80 端口没有被占用，则使用内置的 HTTP 服务器
	if !pkgos.TCPPortInUse(80) {
		s.useBuiltin = true
		return s.startServer()
	}

	// 否则使用 web 服务器配置
	s.useBuiltin = false
	if s.webServer == "apache" {
		return s.writeApacheConfig()
	}
	return s.writeNginxConfig()
}

func (s *panelSolver) startServer() error {
	s.server = &http.Server{
		Addr: ":80",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := s.tokens[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(token))
		}),
	}

	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	// 等待一小段时间确保服务器启动成功
	select {
	case err := <-errChan:
		s.server = nil
		return fmt.Errorf("failed to start HTTP server: %w", err)
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (s *panelSolver) writeNginxConfig() error {
	var conf strings.Builder
	_, _ = fmt.Fprintf(&conf, "server {\n    listen 80;\n    server_name %s;\n", strings.Join(s.ip, " "))
	for path, token := range s.tokens {
		_, _ = fmt.Fprintf(&conf, "    location = %s {\n        default_type text/plain;\n        return 200 %q;\n    }\n", path, token)
	}
	conf.WriteString("}\n")

	if err := os.WriteFile(s.conf, []byte(conf.String()), 0600); err != nil {
		return fmt.Errorf("failed to write nginx config %q: %w", s.conf, err)
	}

	if err := systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	return nil
}

func (s *panelSolver) writeApacheConfig() error {
	// Apache 使用 Alias 指向一个临时目录，将 token 写入文件
	tokenDir := "/tmp/acme-challenge"
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// 写入 token 文件
	for path, token := range s.tokens {
		// path 格式为 /.well-known/acme-challenge/xxx
		tokenFile := filepath.Join(tokenDir, filepath.Base(path))
		if err := os.WriteFile(tokenFile, []byte(token), 0644); err != nil {
			return fmt.Errorf("failed to write token file: %w", err)
		}
	}

	var conf strings.Builder
	_, _ = fmt.Fprintf(&conf, "<VirtualHost *:80>\n    ServerName %s\n", s.ip[0])
	if len(s.ip) > 1 {
		for _, ip := range s.ip[1:] {
			_, _ = fmt.Fprintf(&conf, "    ServerAlias %s\n", ip)
		}
	}
	_, _ = fmt.Fprintf(&conf, "    Alias /.well-known/acme-challenge %s\n", tokenDir)
	_, _ = fmt.Fprintf(&conf, "    <Directory %s>\n", tokenDir)
	conf.WriteString("        Require all granted\n")
	conf.WriteString("        ForceType text/plain\n")
	conf.WriteString("    </Directory>\n")
	conf.WriteString("</VirtualHost>\n")

	if err := os.WriteFile(s.conf, []byte(conf.String()), 0600); err != nil {
		return fmt.Errorf("failed to write apache config %q: %w", s.conf, err)
	}

	if err := systemctl.Reload("apache"); err != nil {
		_, err = shell.Execf("apachectl -t")
		return fmt.Errorf("failed to reload apache: %w", err)
	}

	return nil
}

// CleanUp cleans up the HTTP server on last call.
func (s *panelSolver) CleanUp(ctx context.Context, _ acme.Challenge) error {
	s.cleanupCount++

	// 等待最后一次 CleanUp
	if s.cleanupCount < len(s.ip) {
		return nil
	}

	defer panelSolverGlobal.Unlock()

	if s.useBuiltin && s.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
		s.server = nil
		return nil
	}

	// 清理配置文件
	if err := os.WriteFile(s.conf, []byte(""), 0600); err != nil {
		return fmt.Errorf("failed to write to config %q: %w", s.conf, err)
	}

	// 清理 Apache token 目录
	if s.webServer == "apache" {
		_ = os.RemoveAll("/tmp/acme-challenge")
		if err := systemctl.Reload("apache"); err != nil {
			_, _ = shell.Execf("apachectl -t")
			return fmt.Errorf("failed to reload apache: %w", err)
		}
		return nil
	}

	if err := systemctl.Reload("nginx"); err != nil {
		_, _ = shell.Execf("nginx -t")
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	return nil
}

type httpSolver struct {
	conf      string
	webServer string // "nginx" or "apache"
}

func (s httpSolver) Present(_ context.Context, challenge acme.Challenge) error {
	path := challenge.HTTP01ResourcePath()
	token := challenge.KeyAuthorization

	if s.webServer == "apache" {
		return s.presentApache(path, token)
	}
	return s.presentNginx(path, token)
}

func (s httpSolver) presentNginx(path, token string) error {
	conf := fmt.Sprintf(`location = %s {
    default_type text/plain;
    return 200 %q;
}
`, path, token)

	file, err := os.OpenFile(s.conf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open nginx config %q: %w", s.conf, err)
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if _, err = file.Write([]byte(conf)); err != nil {
		return fmt.Errorf("failed to write to nginx config %q: %w", s.conf, err)
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	return nil
}

func (s httpSolver) presentApache(path, token string) error {
	// 创建 token 目录
	tokenDir := filepath.Dir(s.conf) + "/acme-challenge"
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// 写入 token 文件
	tokenFile := filepath.Join(tokenDir, filepath.Base(path))
	if err := os.WriteFile(tokenFile, []byte(token), 0644); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	// 写入 Apache 配置
	conf := fmt.Sprintf(`Alias /.well-known/acme-challenge %s
<Directory %s>
    Require all granted
    ForceType text/plain
</Directory>
`, tokenDir, tokenDir)

	file, err := os.OpenFile(s.conf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open apache config %q: %w", s.conf, err)
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if _, err = file.Write([]byte(conf)); err != nil {
		return fmt.Errorf("failed to write to apache config %q: %w", s.conf, err)
	}

	if err = systemctl.Reload("apache"); err != nil {
		_, err = shell.Execf("apachectl -t")
		return fmt.Errorf("failed to reload apache: %w", err)
	}

	return nil
}

// CleanUp cleans up the HTTP server if it is the last one to finish.
func (s httpSolver) CleanUp(_ context.Context, challenge acme.Challenge) error {
	path := challenge.HTTP01ResourcePath()
	token := challenge.KeyAuthorization

	if s.webServer == "apache" {
		return s.cleanUpApache(path, token)
	}
	return s.cleanUpNginx(path, token)
}

func (s httpSolver) cleanUpNginx(path, token string) error {
	conf, err := os.ReadFile(s.conf)
	if err != nil {
		return fmt.Errorf("failed to read nginx config %q: %w", s.conf, err)
	}

	target := fmt.Sprintf(`location = %s {
    default_type text/plain;
    return 200 %q;
}
`, path, token)

	newConf := strings.ReplaceAll(string(conf), target, "")
	if err = os.WriteFile(s.conf, []byte(newConf), 0600); err != nil {
		return fmt.Errorf("failed to write to nginx config %q: %w", s.conf, err)
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	return nil
}

func (s httpSolver) cleanUpApache(path, token string) error {
	tokenDir := filepath.Dir(s.conf) + "/acme-challenge"

	// 删除 token 文件
	tokenFile := filepath.Join(tokenDir, filepath.Base(path))
	_ = os.Remove(tokenFile)

	// 清理配置文件
	conf, err := os.ReadFile(s.conf)
	if err != nil {
		return fmt.Errorf("failed to read apache config %q: %w", s.conf, err)
	}

	target := fmt.Sprintf(`Alias /.well-known/acme-challenge %s
<Directory %s>
    Require all granted
    ForceType text/plain
</Directory>
`, tokenDir, tokenDir)

	newConf := strings.ReplaceAll(string(conf), target, "")
	if err = os.WriteFile(s.conf, []byte(newConf), 0600); err != nil {
		return fmt.Errorf("failed to write to apache config %q: %w", s.conf, err)
	}

	if err = systemctl.Reload("apache"); err != nil {
		_, err = shell.Execf("apachectl -t")
		return fmt.Errorf("failed to reload apache: %w", err)
	}

	return nil
}

type dnsSolver struct {
	dns              DnsType
	param            DNSParam
	records          []libdns.Record
	mu               sync.Mutex
	dnsServer        string       // DNS 验证服务器地址
	skipVerify       bool         // 跳过解析验证
	progressCallback func(string) // 进度回调
}

func (s *dnsSolver) Present(ctx context.Context, challenge acme.Challenge) error {
	dnsName := challenge.DNS01TXTRecordName()
	keyAuth := challenge.DNS01KeyAuthorization()
	provider, err := s.getDNSProvider()
	if err != nil {
		return fmt.Errorf("failed to get DNS provider: %w", err)
	}
	zone, err := publicsuffix.EffectiveTLDPlusOne(dnsName)
	if err != nil {
		return fmt.Errorf("failed to get the effective TLD+1 for %q: %w", dnsName, err)
	}

	s.report(fmt.Sprintf("setting DNS TXT record %s", dnsName))

	rec := libdns.TXT{
		Name: libdns.RelativeName(dnsName+".", zone+"."),
		Text: keyAuth,
	}

	results, err := provider.SetRecords(ctx, zone+".", []libdns.Record{rec})
	if err != nil {
		return fmt.Errorf("failed to set DNS record %q for %q: %w", dnsName, zone, err)
	}
	if len(results) == 0 {
		return fmt.Errorf("DNS provider returned no records after setting %q", dnsName)
	}

	s.mu.Lock()
	s.records = append(s.records, results...)
	s.mu.Unlock()

	s.report(fmt.Sprintf("DNS TXT record %s set successfully", dnsName))
	return nil
}

// Wait 实现 acmez.Waiter 接口，等待 DNS TXT 记录传播后再通知 CA 进行验证
func (s *dnsSolver) Wait(ctx context.Context, challenge acme.Challenge) error {
	if s.skipVerify {
		s.report("skip DNS verification, waiting 60s for propagation")
		timer := time.NewTimer(60 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	dnsName := challenge.DNS01TXTRecordName()
	expected := challenge.DNS01KeyAuthorization()

	// 确定 DNS 服务器
	dnsServer := s.dnsServer
	if dnsServer == "" {
		dnsServer = "8.8.8.8"
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			server := dnsServer
			if !strings.Contains(server, ":") {
				server += ":53"
			}
			return d.DialContext(ctx, "udp", server)
		},
	}

	const maxAttempts = 120
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	s.report(fmt.Sprintf("verifying TXT record %s via DNS server %s", dnsName, dnsServer))

	for i := 1; i <= maxAttempts; i++ {
		txts, err := resolver.LookupTXT(ctx, dnsName)
		if err == nil {
			for _, txt := range txts {
				if txt == expected {
					s.report(fmt.Sprintf("DNS TXT record verified (attempt %d)", i))
					return nil
				}
			}
		}

		s.report(fmt.Sprintf("polling DNS record (%d/%d)", i, maxAttempts))

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("DNS propagation timeout after %d attempts", maxAttempts)
}

func (s *dnsSolver) CleanUp(ctx context.Context, challenge acme.Challenge) error {
	dnsName := challenge.DNS01TXTRecordName()
	provider, err := s.getDNSProvider()
	if err != nil {
		return fmt.Errorf("failed to get DNS provider: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	zone, err := publicsuffix.EffectiveTLDPlusOne(dnsName)
	if err != nil {
		return fmt.Errorf("failed to get the effective TLD+1 for %q: %w", dnsName, err)
	}

	s.report("cleaning up DNS TXT records")

	s.mu.Lock()
	records := s.records
	s.mu.Unlock()

	_, _ = provider.DeleteRecords(ctx, zone+".", records)
	return nil
}

func (s *dnsSolver) getDNSProvider() (DNSProvider, error) {
	var dns DNSProvider

	switch s.dns {
	case AliYun:
		dns = &alidns.Provider{
			CredentialInfo: alidns.CredentialInfo{
				AccessKeyID:     s.param.AK,
				AccessKeySecret: s.param.SK,
			},
		}
	case Tencent:
		dns = &tencentcloud.Provider{
			SecretId:  s.param.AK,
			SecretKey: s.param.SK,
		}
	case Huawei:
		dns = &huaweicloud.Provider{
			AccessKeyId:     s.param.AK,
			SecretAccessKey: s.param.SK,
		}
	case Westcn:
		dns = &westcn.Provider{
			Username:    s.param.SK,
			APIPassword: s.param.AK,
		}
	case CloudFlare:
		dns = &cloudflare.Provider{
			APIToken: s.param.AK,
		}
	case Gcore:
		dns = &gcore.Provider{
			APIKey: s.param.AK,
		}
	case Porkbun:
		dns = &porkbun.Provider{
			APIKey:       s.param.AK,
			APISecretKey: s.param.SK,
		}
	case NameSilo:
		dns = &namesilo.Provider{
			APIToken: s.param.AK,
		}
	case ClouDNS:
		if after, ok := strings.CutPrefix(s.param.AK, "sub-"); ok {
			dns = &cloudns.Provider{
				SubAuthId:    after,
				AuthPassword: s.param.SK,
			}
		} else {
			dns = &cloudns.Provider{
				AuthId:       s.param.AK,
				AuthPassword: s.param.SK,
			}
		}
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", s.dns)
	}

	return dns, nil
}

// report 安全地调用进度回调
func (s *dnsSolver) report(msg string) {
	if s.progressCallback != nil {
		s.progressCallback(msg)
	}
}

type DnsType string

const (
	AliYun     DnsType = "aliyun"
	Tencent    DnsType = "tencent"
	Huawei     DnsType = "huawei"
	Westcn     DnsType = "westcn"
	CloudFlare DnsType = "cloudflare"
	Gcore      DnsType = "gcore"
	Porkbun    DnsType = "porkbun"
	NameSilo   DnsType = "namesilo"
	ClouDNS    DnsType = "cloudns"
)

type DNSParam struct {
	AK         string `form:"ak" json:"ak"`
	SK         string `form:"sk" json:"sk"`
	DnsServer  string `form:"dns_server" json:"dns_server"`   // DNS 验证服务器
	SkipVerify bool   `form:"skip_verify" json:"skip_verify"` // 跳过解析验证
}

type DNSProvider interface {
	libdns.RecordSetter
	libdns.RecordDeleter
}

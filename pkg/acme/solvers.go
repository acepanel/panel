package acme

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
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
)

// HTTPChallengeWriter 由 Web 服务器方言实现，bool 返回值表示配置是否变化，未变化则跳过重载
type HTTPChallengeWriter interface {
	WriteSiteChallenge(conf, path, token string) (bool, error)
	RemoveSiteChallenge(conf, path, token string) (bool, error)
	WritePanelChallenge(conf string, names []string, tokens map[string]string) (bool, error)
	RemovePanelChallenge(conf string) (bool, error)
	Reload(ctx context.Context) error
}

var panelSolverGlobal sync.Mutex

type panelSolver struct {
	names  []string
	conf   string
	writer HTTPChallengeWriter
	server *http.Server
	// tokens 存储所有待验证的 challenge，key 为路径，value 为 token
	tokens map[string]string
	// presentCount Present 调用计数
	presentCount int
	// cleanupCount CleanUp 调用计数
	cleanupCount int
	// useBuiltin 标记是否使用内置 HTTP 服务器
	useBuiltin bool
}

func (s *panelSolver) Present(ctx context.Context, challenge acme.Challenge) error {
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
	s.names = append(s.names, challenge.Identifier.Value)
	s.presentCount++

	// 内置服务器启动后只需继续追加 token
	if s.server != nil {
		return nil
	}

	// 没有 Web 服务器或 80 端口空闲时使用内置的 HTTP 服务器，端口被占用会在启动时报错
	if s.writer == nil || !pkgos.TCPPortInUse(80) { //nolint:contextcheck
		s.useBuiltin = true
		return s.startServer()
	}

	// 否则使用 web 服务器配置
	s.useBuiltin = false
	changed, err := s.writer.WritePanelChallenge(s.conf, s.names, s.tokens)
	if err != nil || !changed {
		return err
	}

	return s.writer.Reload(ctx)
}

func (s *panelSolver) startServer() error {
	s.server = &http.Server{
		Addr:              ":80",
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
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
	case <-time.After(500 * time.Millisecond):
		return nil
	}
}

// CleanUp cleans up the HTTP server on last call.
func (s *panelSolver) CleanUp(ctx context.Context, _ acme.Challenge) error {
	s.cleanupCount++

	// 等待所有实际执行过 Present 的验证完成
	if s.cleanupCount < s.presentCount {
		return nil
	}

	defer panelSolverGlobal.Unlock()

	// 善后必须做完：取消时跳过会留下占着 80 端口的服务器或运行中的 challenge 配置
	ctx = context.WithoutCancel(ctx)

	// 内置服务器启动失败时 Present 已报错，这里同样不能去动 Web 服务器配置
	if s.useBuiltin {
		if s.server == nil {
			return nil
		}
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
		s.server = nil
		return nil
	}

	changed, err := s.writer.RemovePanelChallenge(s.conf)
	if err != nil || !changed {
		return err
	}

	return s.writer.Reload(ctx)
}

type httpSolver struct {
	// confs 域名到 acme 配置文件的映射，用于把 token 精确投放到域名所属网站
	confs map[string]string
	// fallback 域名未命中 confs 时写入的配置文件列表
	fallback []string
	writer   HTTPChallengeWriter
}

// confsFor 取域名对应的配置文件列表
func (s httpSolver) confsFor(domain string) []string {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if conf, ok := s.confs[domain]; ok {
		return []string{conf}
	}
	// 网站的 server_name 可能写成泛域名，如 *.example.com 覆盖 a.example.com
	if _, parent, found := strings.Cut(domain, "."); found {
		if conf, ok := s.confs["*."+parent]; ok {
			return []string{conf}
		}
	}

	return s.fallback
}

func (s httpSolver) Present(ctx context.Context, challenge acme.Challenge) error {
	path := challenge.HTTP01ResourcePath()
	token := challenge.KeyAuthorization
	reload := false
	for _, conf := range s.confsFor(challenge.Identifier.Value) {
		changed, err := s.writer.WriteSiteChallenge(conf, path, token)
		if err != nil {
			return err
		}
		reload = reload || changed
	}
	if !reload {
		return nil
	}

	return s.writer.Reload(ctx)
}

// CleanUp cleans up the HTTP server if it is the last one to finish.
func (s httpSolver) CleanUp(ctx context.Context, challenge acme.Challenge) error {
	path := challenge.HTTP01ResourcePath()
	token := challenge.KeyAuthorization
	reload := false
	for _, conf := range s.confsFor(challenge.Identifier.Value) {
		changed, err := s.writer.RemoveSiteChallenge(conf, path, token)
		if err != nil {
			return err
		}
		reload = reload || changed
	}
	if !reload {
		return nil
	}

	// 善后必须做完：取消时跳过会把 challenge 片段留在运行中的配置里
	return s.writer.Reload(context.WithoutCancel(ctx))
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

const defaultDNSServer = "8.8.8.8"

type DNSParam struct {
	AK         string `form:"ak" json:"ak"`
	SK         string `form:"sk" json:"sk"`
	DnsServer  string `form:"dns_server" json:"dns_server"`   // DNS 验证服务器
	SkipVerify bool   `form:"skip_verify" json:"skip_verify"` // 跳过解析验证
}

type DNSProvider interface {
	libdns.RecordAppender
	libdns.RecordDeleter
}

type dnsSolver struct {
	mu               sync.Mutex
	dns              DnsType
	param            DNSParam
	records          map[string][]libdns.Record // dnsName → 已写入的记录
	alias            map[string]string          // DNS 验证别名映射
	dnsServer        string                     // DNS 验证服务器地址
	skipVerify       bool                       // 跳过解析验证
	progressCallback func(string)               // 进度回调
}

func (s *dnsSolver) Present(ctx context.Context, challenge acme.Challenge) error {
	dnsName, zone, err := s.resolveAlias(challenge)
	if err != nil {
		return err
	}
	keyAuth := challenge.DNS01KeyAuthorization()
	provider, err := s.getDNSProvider()
	if err != nil {
		return fmt.Errorf("failed to get DNS provider: %w", err)
	}

	s.report("setting DNS TXT record " + dnsName)

	// 同时签主域 + 通配符（如 example.com 与 *.example.com）会产生两个 challenge，
	// 它们落在同一个 _acme-challenge.example.com TXT 名下，但 keyAuth 不同
	rec := libdns.TXT{
		Name: libdns.RelativeName(dnsName+".", zone+"."),
		Text: keyAuth,
	}
	results, err := provider.AppendRecords(ctx, zone+".", []libdns.Record{rec})
	if err != nil {
		return fmt.Errorf("failed to append DNS record %q for %q: %w", dnsName, zone, err)
	}
	if len(results) != 1 {
		return fmt.Errorf("DNS provider returned %d records after appending %q, expected 1", len(results), dnsName)
	}

	s.mu.Lock()
	s.records[dnsName] = append(s.records[dnsName], results[0])
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

	dnsName, _, err := s.resolveAlias(challenge)
	if err != nil {
		return err
	}
	expected := challenge.DNS01KeyAuthorization()

	// 确定 DNS 服务器
	dnsServer := s.dnsServer
	if dnsServer == "" {
		dnsServer = defaultDNSServer
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			server := dnsServer
			if _, _, err := net.SplitHostPort(server); err != nil {
				server = net.JoinHostPort(server, "53")
			}
			return d.DialContext(ctx, network, server)
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
	dnsName, zone, err := s.resolveAlias(challenge)
	if err != nil {
		return err
	}
	provider, err := s.getDNSProvider()
	if err != nil {
		return fmt.Errorf("failed to get DNS provider: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	s.report("cleaning up DNS TXT records")

	// 同名 TXT 记录下的多条 challenge 记录由首次 CleanUp 一并删除
	s.mu.Lock()
	records := s.records[dnsName]
	delete(s.records, dnsName)
	s.mu.Unlock()

	if len(records) > 0 {
		_, _ = provider.DeleteRecords(ctx, zone+".", records)
	}
	return nil
}

func (s *dnsSolver) getDNSProvider() (DNSProvider, error) {
	return NewDNSProvider(s.dns, s.param)
}

// NewDNSProvider 按提供商类型构造 libdns 客户端，供证书签发之外的模块（如邮件服务写 DNS 记录）复用
func NewDNSProvider(dnsType DnsType, param DNSParam) (DNSProvider, error) {
	var dns DNSProvider

	switch dnsType {
	case AliYun:
		dns = &alidns.Provider{
			CredentialInfo: alidns.CredentialInfo{
				AccessKeyID:     param.AK,
				AccessKeySecret: param.SK,
			},
		}
	case Tencent:
		dns = &tencentcloud.Provider{
			SecretId:  param.AK,
			SecretKey: param.SK,
		}
	case Huawei:
		dns = &huaweicloud.Provider{
			AccessKeyId:     param.AK,
			SecretAccessKey: param.SK,
		}
	case Westcn:
		dns = &westcn.Provider{
			Username:    param.SK,
			APIPassword: param.AK,
		}
	case CloudFlare:
		dns = &cloudflare.Provider{
			APIToken: param.AK,
		}
	case Gcore:
		dns = &gcore.Provider{
			APIKey: param.AK,
		}
	case Porkbun:
		dns = &porkbun.Provider{
			APIKey:       param.AK,
			APISecretKey: param.SK,
		}
	case NameSilo:
		dns = &namesilo.Provider{
			APIToken: param.AK,
		}
	case ClouDNS:
		if after, ok := strings.CutPrefix(param.AK, "sub-"); ok {
			dns = &cloudns.Provider{
				SubAuthId:    after,
				AuthPassword: param.SK,
			}
		} else {
			dns = &cloudns.Provider{
				AuthId:       param.AK,
				AuthPassword: param.SK,
			}
		}
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", dnsType)
	}

	return dns, nil
}

// resolveAlias 根据别名映射解析实际的 DNS 记录名和 zone
func (s *dnsSolver) resolveAlias(challenge acme.Challenge) (dnsName string, zone string, err error) {
	dnsName = challenge.DNS01TXTRecordName()

	// 先用原始域名查别名（如 *.example.com），再用裸域名兜底（如 example.com）
	if s.alias != nil {
		domain := challenge.Identifier.Value
		if target, ok := s.alias[domain]; ok {
			dnsName = target
		} else if bare := strings.TrimPrefix(domain, "*."); bare != domain {
			if target, ok := s.alias[bare]; ok {
				dnsName = target
			}
		}
	}

	zone, err = publicsuffix.EffectiveTLDPlusOne(dnsName)
	if err != nil {
		err = fmt.Errorf("failed to get the effective TLD+1 for %q: %w", dnsName, err)
	}
	return
}

// report 安全地调用进度回调
func (s *dnsSolver) report(msg string) {
	if s.progressCallback != nil {
		s.progressCallback(msg)
	}
}

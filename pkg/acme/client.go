package acme

import (
	"context"
	"crypto/x509"
	"net"

	"github.com/libdns/libdns"
	"github.com/mholt/acmez/v3"
	"github.com/mholt/acmez/v3/acme"

	"github.com/acepanel/panel/v3/pkg/cert"
)

type Certificate struct {
	PrivateKey []byte
	acme.Certificate
}

type Client struct {
	Account acme.Account
	zClient acmez.Client
}

// DnsOption DNS 验证的可选配置
type DnsOption struct {
	Alias            map[string]string // DNS 验证别名映射 (domain → delegated domain)
	DnsServer        string            // DNS 验证服务器地址
	SkipVerify       bool              // 跳过解析验证
	ProgressCallback func(string)      // 进度回调
}

// UseDns 使用 DNS 接口验证
func (c *Client) UseDns(dnsType DnsType, param DNSParam, opt ...DnsOption) {
	solver := &dnsSolver{
		dns:     dnsType,
		param:   param,
		records: make(map[string][]libdns.Record),
	}
	if len(opt) > 0 {
		solver.alias = opt[0].Alias
		solver.dnsServer = opt[0].DnsServer
		solver.skipVerify = opt[0].SkipVerify
		solver.progressCallback = opt[0].ProgressCallback
	}
	c.zClient.ChallengeSolvers = map[string]acmez.Solver{
		acme.ChallengeTypeDNS01: solver,
	}
}

// UseHTTP 使用 HTTP 验证
// confs 域名到配置文件路径的映射，token 会按域名投放到对应网站
// fallback 域名未命中 confs 时写入的配置文件列表
// writer 负责按 Web 服务器语法投放验证配置
func (c *Client) UseHTTP(confs map[string]string, fallback []string, writer HTTPChallengeWriter) {
	c.zClient.ChallengeSolvers = map[string]acmez.Solver{
		acme.ChallengeTypeHTTP01: httpSolver{
			confs:    confs,
			fallback: fallback,
			writer:   writer,
		},
	}
}

// UsePanel 使用面板 HTTP 验证
// conf 配置文件路径
// writer 负责按 Web 服务器语法投放验证配置
func (c *Client) UsePanel(conf string, writer HTTPChallengeWriter) {
	c.zClient.ChallengeSolvers = map[string]acmez.Solver{
		acme.ChallengeTypeHTTP01: &panelSolver{
			conf:   conf,
			writer: writer,
			tokens: make(map[string]string),
		},
	}
}

// ObtainCertificate 签发 SSL 证书
func (c *Client) ObtainCertificate(ctx context.Context, sans []string, keyType KeyType) (Certificate, error) {
	// IP 地址
	for _, san := range sans {
		if net.ParseIP(san) != nil {
			return c.ObtainIPCertificate(ctx, sans, keyType)
		}
	}

	certPrivateKey, err := generatePrivateKey(keyType)
	if err != nil {
		return Certificate{}, err
	}
	pemPrivateKey, err := cert.EncodeKey(certPrivateKey)
	if err != nil {
		return Certificate{}, err
	}

	certs, err := c.zClient.ObtainCertificateForSANs(ctx, c.Account, certPrivateKey, sans)
	if err != nil {
		return Certificate{}, err
	}

	return Certificate{PrivateKey: pemPrivateKey, Certificate: certs[0]}, nil
}

// ObtainIPCertificate 签发 IP SSL 证书
func (c *Client) ObtainIPCertificate(ctx context.Context, sans []string, keyType KeyType) (Certificate, error) {
	certPrivateKey, err := generatePrivateKey(keyType)
	if err != nil {
		return Certificate{}, err
	}
	pemPrivateKey, err := cert.EncodeKey(certPrivateKey)
	if err != nil {
		return Certificate{}, err
	}

	csr, err := acmez.NewCSR(certPrivateKey, sans)
	if err != nil {
		return Certificate{}, err
	}

	params, err := acmez.OrderParametersFromCSR(c.Account, csr)
	if err != nil {
		return Certificate{}, err
	}
	params.Profile = "shortlived"

	certs, err := c.zClient.ObtainCertificate(ctx, params)
	if err != nil {
		return Certificate{}, err
	}

	return Certificate{PrivateKey: pemPrivateKey, Certificate: certs[0]}, nil
}

// GetRenewalInfo 获取续签建议
func (c *Client) GetRenewalInfo(ctx context.Context, cert x509.Certificate) (acme.RenewalInfo, error) {
	return c.zClient.GetRenewalInfo(ctx, &cert)
}

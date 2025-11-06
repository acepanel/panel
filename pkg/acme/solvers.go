package acme

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/libdns/alidns"
	"github.com/libdns/cloudflare"
	"github.com/libdns/cloudns"
	"github.com/libdns/gcore"
	"github.com/libdns/hetzner/v2"
	"github.com/libdns/huaweicloud"
	"github.com/libdns/libdns"
	"github.com/libdns/namesilo"
	"github.com/libdns/porkbun"
	"github.com/libdns/tencentcloud"
	"github.com/libdns/westcn"
	"github.com/mholt/acmez/v3/acme"
	"golang.org/x/net/publicsuffix"

	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
)

type httpSolver struct {
	conf string
}

func (s httpSolver) Present(_ context.Context, challenge acme.Challenge) error {
	conf := fmt.Sprintf(`location = %s {
    default_type text/plain;
    return 200 %q;
}
`, challenge.HTTP01ResourcePath(), challenge.KeyAuthorization)

	file, err := os.OpenFile(s.conf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open nginx config %q: %w", s.conf, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	if _, err = file.Write([]byte(conf)); err != nil {
		return fmt.Errorf("failed to write to nginx config %q: %w", s.conf, err)
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	return nil
}

// CleanUp cleans up the HTTP server if it is the last one to finish.
func (s httpSolver) CleanUp(_ context.Context, challenge acme.Challenge) error {
	conf, err := os.ReadFile(s.conf)
	if err != nil {
		return fmt.Errorf("failed to read nginx config %q: %w", s.conf, err)
	}

	target := fmt.Sprintf(`location = %s {
    default_type text/plain;
    return 200 %q;
}
`, challenge.HTTP01ResourcePath(), challenge.KeyAuthorization)

	newConf := strings.ReplaceAll(string(conf), target, "")
	if err = os.WriteFile(s.conf, []byte(newConf), 0644); err != nil {
		return fmt.Errorf("failed to write to nginx config %q: %w", s.conf, err)
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	return nil
}

type dnsSolver struct {
	dns     DnsType
	param   DNSParam
	records []libdns.Record
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

	rec := libdns.TXT{
		Name: libdns.RelativeName(dnsName+".", zone+"."),
		Text: keyAuth,
		TTL:  10 * time.Minute,
	}

	results, err := provider.SetRecords(ctx, zone+".", []libdns.Record{rec})
	if err != nil {
		return fmt.Errorf("failed to set DNS record %q for %q: %w", dnsName, zone, err)
	}
	if len(results) != 1 {
		return fmt.Errorf("expected to add 1 record, but actually added %d records", len(results))
	}

	s.records = results
	return nil
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

	_, _ = provider.DeleteRecords(ctx, zone+".", s.records)
	return nil
}

func (s *dnsSolver) getDNSProvider() (DNSProvider, error) {
	var dns DNSProvider

	switch s.dns {
	case AliYun:
		dns = &alidns.Provider{
			AccKeyID:     s.param.AK,
			AccKeySecret: s.param.SK,
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
		if strings.HasPrefix(s.param.AK, "sub-") {
			dns = &cloudns.Provider{
				SubAuthId:    strings.TrimPrefix(s.param.AK, "sub-"),
				AuthPassword: s.param.SK,
			}
		} else {
			dns = &cloudns.Provider{
				AuthId:       s.param.AK,
				AuthPassword: s.param.SK,
			}
		}
	case Hetzner:
		dns = &hetzner.Provider{
			AuthAPIToken: s.param.AK,
		}
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", s.dns)
	}

	return dns, nil
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
	Hetzner    DnsType = "hetzner"
)

type DNSParam struct {
	AK string `form:"ak" json:"ak"`
	SK string `form:"sk" json:"sk"`
}

type DNSProvider interface {
	libdns.RecordSetter
	libdns.RecordDeleter
}

type manualDNSSolver struct {
	check       bool // 是否检查 DNS 解析，目前没写
	controlChan chan struct{}
	dnsChan     chan any
	certChan    chan any
	records     []DNSRecord
}

func (s *manualDNSSolver) Present(ctx context.Context, challenge acme.Challenge) error {
	full := challenge.DNS01TXTRecordName()
	keyAuth := challenge.DNS01KeyAuthorization()
	domain, err := publicsuffix.EffectiveTLDPlusOne(full)
	if err != nil {
		return fmt.Errorf("failed to get the effective TLD+1 for %q: %w", full, err)
	}

	s.records = append(s.records, DNSRecord{
		Name:   strings.TrimSuffix(full, "."+domain),
		Domain: domain,
		Value:  keyAuth,
	})
	s.dnsChan <- s.records

	select {
	case <-s.controlChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *manualDNSSolver) CleanUp(_ context.Context, _ acme.Challenge) error {
	defer func() {
		_ = recover()
	}()
	close(s.controlChan)
	close(s.dnsChan)
	close(s.certChan)
	return nil
}

type DNSRecord struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Value  string `json:"value"`
}

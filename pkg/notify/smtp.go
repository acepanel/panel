package notify

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTP 加密方式
const (
	encryptionNone     = "none"
	encryptionSSL      = "ssl"
	encryptionSTARTTLS = "starttls"
)

// smtpConfig SMTP 渠道配置
type smtpConfig struct {
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	Encryption string   `json:"encryption"` // none / ssl / starttls
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	From       string   `json:"from"`
	FromName   string   `json:"from_name"`
	To         []string `json:"to"`
	SkipVerify bool     `json:"skip_verify"`
}

type smtpNotifier struct {
	conf smtpConfig
}

// newSMTP 构造 SMTP 通知器
func newSMTP(config json.RawMessage) (Notifier, error) {
	var conf smtpConfig
	if err := json.Unmarshal(config, &conf); err != nil {
		return nil, err
	}
	if conf.Host == "" || conf.Port <= 0 {
		return nil, errors.New("smtp host and port are required")
	}
	if conf.From == "" {
		conf.From = conf.Username
	}
	if conf.From == "" {
		return nil, errors.New("smtp sender address is required")
	}
	if len(conf.To) == 0 {
		return nil, errors.New("smtp recipients are required")
	}
	// 未知取值不能落到明文分支，否则拼写错误会让预期加密的连接静默降级
	if !slices.Contains([]string{encryptionNone, encryptionSSL, encryptionSTARTTLS}, conf.Encryption) {
		return nil, fmt.Errorf("unsupported smtp encryption: %s", conf.Encryption)
	}

	return &smtpNotifier{conf: conf}, nil
}

func (s *smtpNotifier) Send(ctx context.Context, msg *Message) error {
	m := mail.NewMsg()
	if err := m.FromFormat(s.conf.FromName, s.conf.From); err != nil {
		return err
	}
	if err := m.To(s.conf.To...); err != nil {
		return err
	}
	m.Subject(msg.Subject)
	m.SetBodyString(mail.TypeTextHTML, msg.Body)

	options := []mail.Option{
		mail.WithPort(s.conf.Port),
		mail.WithTimeout(30 * time.Second),
		mail.WithTLSConfig(&tls.Config{
			ServerName:         s.conf.Host,
			InsecureSkipVerify: s.conf.SkipVerify, //nolint:gosec
			MinVersion:         tls.VersionTLS11,
		}),
	}

	switch s.conf.Encryption {
	case encryptionSSL:
		options = append(options, mail.WithSSL())
	case encryptionSTARTTLS:
		options = append(options, mail.WithTLSPolicy(mail.TLSMandatory))
	default:
		options = append(options, mail.WithTLSPolicy(mail.NoTLS))
	}

	// 无用户名视为匿名投递
	if s.conf.Username != "" {
		options = append(options,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(s.conf.Username),
			mail.WithPassword(s.conf.Password),
		)
	} else {
		options = append(options, mail.WithSMTPAuth(mail.SMTPAuthNoAuth))
	}

	client, err := mail.NewClient(s.conf.Host, options...)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	return client.DialAndSendWithContext(ctx, m)
}

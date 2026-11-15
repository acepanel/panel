package nginx

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

const phpCacheConf = `# browser cache
location ~ .*\.(bmp|jpg|jpeg|png|gif|svg|ico|tiff|webp|avif|heif|heic|jxl)$ {
    expires 30d;
    access_log /dev/null;
    error_log /dev/null;
}
location ~ .*\.(js|css|ttf|otf|woff|woff2|eot)$ {
    expires 6h;
    access_log /dev/null;
    error_log /dev/null;
}
# deny sensitive files
location ~ ^/(\.user.ini|\.htaccess|\.git|\.svn|\.env) {
    return 404;
}
`

const spaConf = `# single-page application route fallback, remove if not needed
location / {
    try_files $uri $uri/ /index.html;
}
`

// tcpReuseportRe 只看 80 端口的 TCP 监听，quic 监听始终带 reuseport
var tcpReuseportRe = regexp.MustCompile(`(?m)^\s*listen\s+80\b[^;]*\breuseport\b`)

type Dialect struct{}

func (Dialect) Service() string {
	return "nginx"
}

func (Dialect) ConfigTest() string {
	return "nginx -t 2>&1"
}

func (Dialect) HTMLDir() string {
	return htmlDir
}

func (Dialect) ConfigFile() string {
	return "nginx.conf"
}

func (Dialect) PanelACMEConf() string {
	return "/opt/ace/server/nginx/conf/acme.conf"
}

func (Dialect) Features() types.Features {
	return types.Features{IPv6Listen: true, Stat: true, DefaultSite: true}
}

func (Dialect) HTTPSListenArgs() []string {
	return []string{"ssl", "quic"}
}

func (Dialect) ErrorPageConf() string {
	return "error_page 404 /404.html;"
}

func (Dialect) PHPCacheConf() string {
	return phpCacheConf
}

func (Dialect) SPAConf() string {
	return spaConf
}

func (Dialect) LSCacheConf(string) string {
	return ""
}

// StatConf 站点名经 SafeName 才能做 log_format 名与 syslog tag
func (Dialect) StatConf(name string) (string, string) {
	safe := types.SafeName(name)
	shared := fmt.Sprintf(`log_format ace_stat_%s escape=json
  '{"site":"%s",'
  '"uri":"$request_uri",'
  '"status":$status,'
  '"bytes":$body_bytes_sent,'
  '"ua":"$http_user_agent",'
  '"ip":"$remote_addr",'
  '"host":"$host",'
  '"method":"$request_method",'
  '"referer":"$http_referer",'
  '"xff":"$http_x_forwarded_for",'
  '"rt":$request_time,'
  '"proto":"$server_protocol",'
  '"port":"$remote_port",'
  '"body":"$request_body",'
  '"content_type":"$sent_http_content_type",'
  '"req_length":$request_length,'
  '"https":"$https",'
  '"upstream_time":"$upstream_response_time",'
  '"upstream_status":"$upstream_status"}';`, safe, name)
	site := fmt.Sprintf("client_body_in_single_buffer on;\naccess_log syslog:server=unix:/tmp/ace_stats.sock,nohostname,tag=%s ace_stat_%s;", safe, safe)
	return shared, site
}

func (Dialect) DefaultSiteConf() string {
	return defaultSiteConf
}

// WriteDefaultSite asDefault 为 false 时 default_server 由某个站点持有
func (Dialect) WriteDefaultSite(asDefault bool) error {
	return os.WriteFile(defaultSiteConf, []byte(defaultSiteContent(asDefault, DefaultSiteReuseport())), 0600)
}

func (Dialect) HTPasswdLine(username, password string) string {
	return username + ":{PLAIN}" + password
}

func (Dialect) RewritesDir() string {
	return "nginx"
}

func (Dialect) BeforeReload() error {
	return nil
}

func (Dialect) NewStaticVhost(configDir string) (types.StaticVhost, error) {
	vhost, err := newStaticVhost(configDir)
	if err != nil {
		return nil, err
	}
	return vhost, nil
}

func (Dialect) NewPHPVhost(configDir string) (types.PHPVhost, error) {
	vhost, err := newPHPVhost(configDir)
	if err != nil {
		return nil, err
	}
	return vhost, nil
}

func (Dialect) NewProxyVhost(configDir string) (types.ProxyVhost, error) {
	vhost, err := newProxyVhost(configDir)
	if err != nil {
		return nil, err
	}
	return vhost, nil
}

func (Dialect) WriteSiteChallenge(conf, path, token string) (bool, error) {
	file, err := os.OpenFile(conf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return false, fmt.Errorf("failed to open nginx config %q: %w", conf, err)
	}
	_, err = file.WriteString(challengeConf(path, token))
	_ = file.Close()
	if err != nil {
		return false, fmt.Errorf("failed to write to nginx config %q: %w", conf, err)
	}

	return true, nil
}

func (Dialect) RemoveSiteChallenge(conf, path, token string) (bool, error) {
	raw, err := os.ReadFile(conf)
	if err != nil {
		return false, fmt.Errorf("failed to read nginx config %q: %w", conf, err)
	}
	content := strings.ReplaceAll(string(raw), challengeConf(path, token), "")
	if err = os.WriteFile(conf, []byte(content), 0600); err != nil {
		return false, fmt.Errorf("failed to write to nginx config %q: %w", conf, err)
	}

	return true, nil
}

func (Dialect) WritePanelChallenge(conf string, names []string, tokens map[string]string) (bool, error) {
	var b strings.Builder
	// 域名有 AAAA 记录时 CA 走 IPv6 验证；默认站点本就监听 [::]:80，不必顾虑纯 IPv4 环境
	b.WriteString("server {\n    listen 80;\n    listen [::]:80;\n")
	wrapped := lo.Map(names, func(name string, _ int) string {
		return tools.WrapIPv6(name)
	})
	_, _ = fmt.Fprintf(&b, "    server_name %s;\n", strings.Join(wrapped, " "))
	for path, token := range tokens {
		_, _ = fmt.Fprintf(&b, "    location = %s {\n        default_type text/plain;\n        return 200 %q;\n    }\n", path, token)
	}
	b.WriteString("}\n")

	if err := os.WriteFile(conf, []byte(b.String()), 0600); err != nil {
		return false, fmt.Errorf("failed to write nginx config %q: %w", conf, err)
	}

	return true, nil
}

func (Dialect) RemovePanelChallenge(conf string) (bool, error) {
	if err := os.WriteFile(conf, []byte(""), 0600); err != nil {
		return false, fmt.Errorf("failed to write to config %q: %w", conf, err)
	}

	return true, nil
}

// DefaultSiteReuseport 内置默认站点的 TCP 监听是否带 reuseport
func DefaultSiteReuseport() bool {
	content, _ := os.ReadFile(defaultSiteConf)
	return tcpReuseportRe.Match(content)
}

func SetDefaultSiteReuseport(enable bool) error {
	content, err := os.ReadFile(defaultSiteConf)
	if err != nil {
		return err
	}
	asDefault := bytes.Contains(content, []byte("default_server"))
	return os.WriteFile(defaultSiteConf, []byte(defaultSiteContent(asDefault, enable)), 0600)
}

// defaultSiteContent quic 必须带 reuseport，否则多个 worker 共用一个 UDP socket，同一连接的包会被别的 worker 读走
func defaultSiteContent(asDefault, reuseport bool) string {
	var flag string
	if asDefault {
		flag = " default_server"
	}
	tcp := flag
	if reuseport {
		tcp += " reuseport"
	}
	return fmt.Sprintf(`server
{
    listen 80%[1]s;
    listen [::]:80%[1]s;
    listen 443 ssl%[1]s;
    listen [::]:443 ssl%[1]s;
    listen 443 quic%[2]s reuseport;
    listen [::]:443 quic%[2]s reuseport;
    server_name _;
    index index.html;
    root %[3]s;
    ssl_reject_handshake on;
}
`, tcp, flag, htmlDir)
}

// challengeConf 单个 HTTP-01 验证的 location 片段
func challengeConf(path, token string) string {
	return fmt.Sprintf("location = %s {\n    default_type text/plain;\n    return 200 %q;\n}\n", path, token)
}

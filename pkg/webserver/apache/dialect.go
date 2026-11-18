package apache

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

const panelTokenDir = "/tmp/acme-challenge"

const phpCacheConf = `# browser cache
<IfModule mod_expires.c>
    ExpiresActive On
    ExpiresByType image/bmp "access plus 30 days"
    ExpiresByType image/jpeg "access plus 30 days"
    ExpiresByType image/png "access plus 30 days"
    ExpiresByType image/gif "access plus 30 days"
    ExpiresByType image/svg+xml "access plus 30 days"
    ExpiresByType image/x-icon "access plus 30 days"
    ExpiresByType image/tiff "access plus 30 days"
    ExpiresByType image/webp "access plus 30 days"
    ExpiresByType image/avif "access plus 30 days"
    ExpiresByType image/heif "access plus 30 days"
    ExpiresByType image/heic "access plus 30 days"
    ExpiresByType image/jxl "access plus 30 days"
    ExpiresByType text/css "access plus 6 hours"
    ExpiresByType application/javascript "access plus 6 hours"
    ExpiresByType font/ttf "access plus 6 hours"
    ExpiresByType font/otf "access plus 6 hours"
    ExpiresByType font/woff "access plus 6 hours"
    ExpiresByType font/woff2 "access plus 6 hours"
    ExpiresByType application/vnd.ms-fontobject "access plus 6 hours"
</IfModule>
# deny sensitive files
<FilesMatch "^(\.user\.ini|\.htaccess|\.git|\.svn|\.env)">
    Require all denied
</FilesMatch>
`

const spaConf = `# single-page application route fallback, remove if not needed
FallbackResource /index.html
`

type Dialect struct{}

func (Dialect) Service() string {
	return "apache"
}

func (Dialect) ConfigTest() string {
	return "apachectl configtest 2>&1"
}

func (Dialect) HTMLDir() string {
	return htmlDir
}

func (Dialect) ConfigFile() string {
	return configName
}

func (Dialect) PanelACMEConf() string {
	return ServerRoot + "/conf/extra/acme.conf"
}

func (Dialect) Features() types.Features {
	return types.Features{Htaccess: true}
}

func (Dialect) HTTPSListenArgs() []string {
	return []string{"ssl"}
}

func (Dialect) ErrorPageConf() string {
	return "ErrorDocument 404 /404.html"
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

func (Dialect) StatConf(string) (string, string) {
	return "", ""
}

func (Dialect) DefaultSiteConf() string {
	return ""
}

func (Dialect) WriteDefaultSite(bool) error {
	return nil
}

// HTPasswdLine 保持明文以便面板回读，保存站点时再转成 Apache 需要的 bcrypt 用户文件
func (Dialect) HTPasswdLine(username, password string) string {
	return username + ":{PLAIN}" + password
}

func (Dialect) RewritesDir() string {
	return "apache"
}

func (Dialect) BeforeReload() error {
	return syncListen(sitesPath, filepath.Join(ServerRoot, "conf"))
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

// WriteSiteChallenge token 落盘到站点配置目录旁的 acme-challenge，再用 Alias 映射
func (Dialect) WriteSiteChallenge(conf, path, token string) (bool, error) {
	tokenDir := filepath.Join(filepath.Dir(conf), "acme-challenge")
	if err := writeToken(tokenDir, path, token); err != nil {
		return false, err
	}

	file, err := os.OpenFile(conf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return false, fmt.Errorf("failed to open apache config %q: %w", conf, err)
	}
	_, err = file.WriteString(challengeConf(tokenDir))
	_ = file.Close()
	if err != nil {
		return false, fmt.Errorf("failed to write to apache config %q: %w", conf, err)
	}

	return true, nil
}

func (Dialect) RemoveSiteChallenge(conf, path, _ string) (bool, error) {
	tokenDir := filepath.Join(filepath.Dir(conf), "acme-challenge")
	_ = os.Remove(filepath.Join(tokenDir, filepath.Base(path)))

	raw, err := os.ReadFile(conf)
	if err != nil {
		return false, fmt.Errorf("failed to read apache config %q: %w", conf, err)
	}
	content := strings.ReplaceAll(string(raw), challengeConf(tokenDir), "")
	if err = os.WriteFile(conf, []byte(content), 0600); err != nil {
		return false, fmt.Errorf("failed to write to apache config %q: %w", conf, err)
	}

	return true, nil
}

func (Dialect) WritePanelChallenge(conf string, names []string, tokens map[string]string) (bool, error) {
	if len(names) == 0 {
		return false, errors.New("no domain to validate")
	}
	for path, token := range tokens {
		if err := writeToken(panelTokenDir, path, token); err != nil {
			return false, err
		}
	}

	wrapped := lo.Map(names, func(name string, _ int) string {
		return tools.WrapIPv6(name)
	})
	var b strings.Builder
	b.WriteString("<VirtualHost *:80>\n")
	_, _ = fmt.Fprintf(&b, "    ServerName %s\n", wrapped[0])
	if len(wrapped) > 1 {
		_, _ = fmt.Fprintf(&b, "    ServerAlias %s\n", strings.Join(wrapped[1:], " "))
	}
	_, _ = fmt.Fprintf(&b, "    Alias /.well-known/acme-challenge %s\n", panelTokenDir)
	_, _ = fmt.Fprintf(&b, "    <Directory %s>\n", panelTokenDir)
	b.WriteString("        Require all granted\n")
	b.WriteString("        ForceType text/plain\n")
	b.WriteString("    </Directory>\n")
	b.WriteString("</VirtualHost>\n")

	if err := os.WriteFile(conf, []byte(b.String()), 0600); err != nil {
		return false, fmt.Errorf("failed to write apache config %q: %w", conf, err)
	}

	return true, nil
}

func (Dialect) RemovePanelChallenge(conf string) (bool, error) {
	if err := os.WriteFile(conf, []byte(""), 0600); err != nil {
		return false, fmt.Errorf("failed to write to config %q: %w", conf, err)
	}
	_ = os.RemoveAll(panelTokenDir)

	return true, nil
}

// syncListen 站点用到的非标端口要在全局声明，Apache 不像 nginx 在站点块里自带监听，少了全局 Listen 站点就完全不可达；
// 同一端口重复声明又会让 Apache 起不来，所以整份重写一个面板独占文件，主配置里只保证有一行引用它
func syncListen(sitesDir, confDir string) error {
	ports := make(map[string]bool)
	confs, _ := filepath.Glob(filepath.Join(sitesDir, "*", "config", configName))
	for _, path := range confs {
		cfg, err := ParseFile(path)
		if err != nil {
			continue
		}
		for _, vhost := range cfg.Blocks("VirtualHost") {
			for _, addr := range vhost.Values() {
				// 80 与 443 由安装脚本写入，这里只补非标端口
				if port := portOf(addr); port != "" && port != "80" && port != "443" {
					ports[port] = true
				}
			}
		}
	}

	lines := []string{"# Auto-generated by AcePanel. DO NOT EDIT MANUALLY!"}
	for _, port := range slices.Sorted(maps.Keys(ports)) {
		lines = append(lines, "Listen "+port)
	}
	if err := os.WriteFile(filepath.Join(confDir, "extra", listenConfName), []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write listen config: %w", err)
	}

	return ensureListenInclude(filepath.Join(confDir, "httpd.conf"))
}

// ensureListenInclude 老版本装出来的主配置里没有这行，补一次
func ensureListenInclude(mainConf string) error {
	include := "IncludeOptional conf/extra/" + listenConfName
	raw, err := os.ReadFile(mainConf)
	if err != nil {
		return fmt.Errorf("failed to read apache config: %w", err)
	}
	if strings.Contains(string(raw), include) {
		return nil
	}

	content := strings.TrimRight(string(raw), "\n") + "\n" + include + "\n"
	if err = os.WriteFile(mainConf, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write apache config: %w", err)
	}
	return nil
}

// writeToken 将验证 token 写入目录，文件名取 URL 路径的最后一段
func writeToken(dir, path, token string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(path)), []byte(token), 0644); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// challengeConf 把 /.well-known/acme-challenge 映射到 token 目录的片段
func challengeConf(tokenDir string) string {
	return fmt.Sprintf(`Alias /.well-known/acme-challenge %s
<Directory %s>
    Require all granted
    ForceType text/plain
</Directory>
`, tokenDir, tokenDir)
}

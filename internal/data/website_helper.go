package data

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

const (
	// customConfigStartNum 自定义配置起始序号
	customConfigStartNum = 800
	// customConfigEndNum 自定义配置结束序号
	customConfigEndNum = 999
)

// dialect 取当前 Web 服务器方言
func (r *websiteRepo) dialect() (webserver.Dialect, error) {
	webServer, err := r.setting.Get(biz.SettingKeyWebserver)
	if err != nil {
		return webserver.Dialect{}, err
	}

	return webserver.Get(webserver.Type(webServer))
}

func (r *websiteRepo) getVhost(website *biz.Website) (webservertypes.Vhost, error) {
	d, err := r.dialect()
	if err != nil {
		return nil, err
	}

	return newVhost(d, website)
}

func (r *websiteRepo) makeConfigDirs(configDir string) error {
	for _, scope := range []string{"site", "shared"} {
		if err := os.MkdirAll(filepath.Join(configDir, scope), 0600); err != nil {
			return err
		}
	}

	return nil
}

// writeTypeConfigs 写入站点类型对应的默认片段：404 页面、PHP 缓存或静态 SPA 回退
func (r *websiteRepo) writeTypeConfigs(d webserver.Dialect, vhost webservertypes.Vhost, typ biz.WebsiteType) error {
	if err := vhost.SetConfig("010-error-404.conf", webservertypes.ScopeSite, d.ErrorPageConf()); err != nil {
		return err
	}
	switch typ {
	case biz.WebsiteTypePHP:
		return vhost.SetConfig("010-cache.conf", webservertypes.ScopeSite, d.PHPCacheConf())
	case biz.WebsiteTypeStatic:
		return vhost.SetRawConfig("800-spa.conf", webservertypes.ScopeSite, d.SPAConf())
	default:
		// 反代站点无需额外片段
	}

	return nil
}

// enableStat 共享级片段只有部分服务器需要
func (r *websiteRepo) enableStat(d webserver.Dialect, vhost webservertypes.Vhost, name string) error {
	shared, site := d.StatConf(name)
	if shared != "" {
		if err := vhost.SetConfig("010-stat-format.conf", webservertypes.ScopeShared, shared); err != nil {
			return err
		}
	}
	return vhost.SetConfig("021-stats-log.conf", webservertypes.ScopeSite, site)
}

// checkDomains 域名与监听端口同时重叠才算被占用：nginx 只是警告，Caddy 会整份配置拒载
func (r *websiteRepo) checkDomains(
	d webserver.Dialect,
	exclude string,
	domains []string,
	listens []webservertypes.Listen,
) error {
	ports := r.listenPorts(listens)
	var websites []*biz.Website
	if err := r.db.Find(&websites).Error; err != nil {
		return err
	}
	for _, website := range websites {
		// 切换服务器重建途中，尚未重建的站点没有当前方言的配置，跳过以免拿默认域名误判
		if website.Name == exclude || !io.Exists(filepath.Join(app.Root, "sites", website.Name, "config", d.ConfigFile())) {
			continue
		}
		vhost, err := newVhost(d, website)
		if err != nil || !lo.Some(r.listenPorts(vhost.Listen()), ports) {
			continue
		}
		for _, domain := range vhost.ServerName() {
			if slices.ContainsFunc(domains, func(d string) bool { return strings.EqualFold(d, domain) }) {
				return errors.New(r.t.Get("domain %s is already used by website %s", domain, website.Name))
			}
		}
	}
	return nil
}

// listenPorts 取监听端口，忽略 `80`、`0.0.0.0:80`、`[::]:80` 的地址形式差异
func (r *websiteRepo) listenPorts(listens []webservertypes.Listen) []string {
	return lo.Uniq(lo.Map(listens, func(listen webservertypes.Listen, _ int) string {
		if _, port, err := net.SplitHostPort(listen.Address); err == nil {
			return port
		}
		return listen.Address
	}))
}

// readBasicAuthUsers 读取 htpasswd 文件中的用户列表
func (r *websiteRepo) readBasicAuthUsers(htpasswdPath string) map[string]string {
	if !io.Exists(htpasswdPath) {
		return make(map[string]string)
	}

	file, err := os.Open(htpasswdPath)
	if err != nil {
		return make(map[string]string)
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	users := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// htpasswd 格式: username:{PLAIN}password或直接username:password
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			users[parts[0]] = strings.TrimPrefix(parts[1], "{PLAIN}")
		}
	}
	if scanner.Err() != nil {
		return make(map[string]string)
	}

	return users
}

// writeBasicAuthUsers 将用户凭证写入 htpasswd 文件
func (r *websiteRepo) writeBasicAuthUsers(d webserver.Dialect, htpasswdPath string, users map[string]string) error {
	var lines []string
	for username, password := range users {
		if username == "" || password == "" {
			continue
		}
		lines = append(lines, d.HTPasswdLine(username, password))
	}

	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	return io.Write(htpasswdPath, content, 0644) // 必须 0644，Nginx 在运行中以 www 用户读取
}

// removeBasicAuthFiles 删除站点的所有 htpasswd 文件
func (r *websiteRepo) removeBasicAuthFiles(siteName string) {
	matches, _ := filepath.Glob(filepath.Join(app.Root, "sites", siteName, "htpasswd*"))
	for _, match := range matches {
		_ = io.Remove(match)
	}
}

// getCustomConfigs 获取网站自定义配置列表
func (r *websiteRepo) getCustomConfigs(configDir string) []types.WebsiteCustomConfig {
	var configs []types.WebsiteCustomConfig

	// 从 site 和 shared 目录读取自定义配置
	for _, scope := range []string{"site", "shared"} {
		scopeDir := filepath.Join(configDir, scope)
		entries, err := os.ReadDir(scopeDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			configName, ok := r.customConfigName(name)
			if !ok || configName == "" {
				continue
			}
			// 读取配置内容
			content, err := io.Read(filepath.Join(scopeDir, name))
			if err != nil {
				continue
			}

			configs = append(configs, types.WebsiteCustomConfig{
				Name:    configName,
				Scope:   scope,
				Content: content,
			})
		}
	}

	return configs
}

// saveCustomConfigs 保存网站自定义配置
func (r *websiteRepo) saveCustomConfigs(configDir string, configs []request.WebsiteCustomConfig) error {
	if err := r.clearCustomConfigs(configDir); err != nil {
		return err
	}

	// 分别跟踪 site 和 shared 目录的序号
	siteNum := customConfigStartNum
	sharedNum := customConfigStartNum

	for _, cfg := range configs {
		var num int
		switch cfg.Scope {
		case "site":
			num = siteNum
			siteNum++
		case "shared":
			num = sharedNum
			sharedNum++
		default:
			return fmt.Errorf("invalid config scope: %s", cfg.Scope)
		}

		if num > customConfigEndNum {
			return errors.New(r.t.Get("maximum number of custom configurations reached (limit: %d)", customConfigEndNum-customConfigStartNum+1))
		}

		fileName := fmt.Sprintf("%03d-%s.conf", num, cfg.Name)
		filePath := filepath.Join(configDir, cfg.Scope, fileName)

		if err := io.Write(filePath, cfg.Content, 0600); err != nil {
			return fmt.Errorf("failed to write custom config: %w", err)
		}
	}

	return nil
}

// clearCustomConfigs 清除网站自定义配置文件
func (r *websiteRepo) clearCustomConfigs(configDir string) error {
	for _, scope := range []string{"site", "shared"} {
		scopeDir := filepath.Join(configDir, scope)
		entries, err := os.ReadDir(scopeDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if _, ok := r.customConfigName(name); !ok {
				continue
			}
			filePath := filepath.Join(scopeDir, name)
			if err = os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove custom config: %w", err)
			}
		}
	}

	return nil
}

// customConfigName 自定义配置文件形如 800-name.conf，序号落在自定义配置段内才算
func (r *websiteRepo) customConfigName(fileName string) (string, bool) {
	if !strings.HasSuffix(fileName, ".conf") {
		return "", false
	}
	parts := strings.SplitN(fileName, "-", 2)
	if len(parts) < 2 {
		return "", false
	}
	num, err := strconv.Atoi(parts[0])
	if err != nil || num < customConfigStartNum || num > customConfigEndNum {
		return "", false
	}

	return strings.TrimSuffix(parts[1], ".conf"), true
}

// newVhost 按网站类型构造站点 vhost
func newVhost(d webserver.Dialect, website *biz.Website) (webservertypes.Vhost, error) {
	return d.NewVhost(string(website.Type), filepath.Join(app.Root, "sites", website.Name, "config"))
}

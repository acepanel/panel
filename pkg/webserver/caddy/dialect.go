package caddy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

type Dialect struct{}

func (Dialect) Service() string {
	return "caddy"
}

func (Dialect) ConfigTest() string {
	return ServerRoot + "/caddy validate --config " + MainConf + " 2>&1"
}

func (Dialect) HTMLDir() string {
	return htmlDir
}

func (Dialect) ConfigFile() string {
	return confName
}

func (Dialect) PanelACMEConf() string {
	return panelACMEConf
}

func (Dialect) Features() types.Features {
	return types.Features{Stat: true, DefaultSite: true}
}

// HTTPSListenArgs HTTP/3 由 Caddy 全局启用，监听参数只需标记 ssl
func (Dialect) HTTPSListenArgs() []string {
	return []string{"ssl"}
}

func (Dialect) ErrorPageConf() string {
	return errorPageConf
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

// StatConf JSON 直发面板的统计套接字，soft_start 保证套接字暂不可用时配置仍能加载
func (Dialect) StatConf(name string) (string, string) {
	return "", fmt.Sprintf(statConf, name)
}

// DefaultSiteConf 兜底块固定在主配置里，默认站点靠无主机名地址排在它前面
func (Dialect) DefaultSiteConf() string {
	return ""
}

func (Dialect) WriteDefaultSite(bool) error {
	return nil
}

// HTPasswdLine 保持明文以便面板回读，保存站点时再转成 Caddy 需要的 bcrypt 用户文件
func (Dialect) HTPasswdLine(username, password string) string {
	return username + ":{PLAIN}" + password
}

func (Dialect) RewritesDir() string {
	return "caddy"
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

// WriteSiteChallenge 落盘 token 即可，无需重载
func (Dialect) WriteSiteChallenge(_, path, token string) (bool, error) {
	return false, writeToken(path, token)
}

func (Dialect) RemoveSiteChallenge(_, path, _ string) (bool, error) {
	return false, removeToken(path)
}

// WritePanelChallenge token 目录各站点共用，另记录文件名供清理
func (Dialect) WritePanelChallenge(conf string, _ []string, tokens map[string]string) (bool, error) {
	var names []string
	for path, token := range tokens {
		if err := writeToken(path, token); err != nil {
			return false, err
		}
		names = append(names, filepath.Base(path))
	}
	if err := os.MkdirAll(filepath.Dir(conf), 0700); err != nil {
		return false, err
	}
	return false, os.WriteFile(conf, []byte(strings.Join(names, "\n")), 0600)
}

func (Dialect) RemovePanelChallenge(conf string) (bool, error) {
	raw, err := os.ReadFile(conf)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for name := range strings.SplitSeq(string(raw), "\n") {
		if name != "" {
			_ = removeToken(name)
		}
	}
	return false, os.WriteFile(conf, []byte(""), 0600)
}

func tokenFile(path string) string {
	return filepath.Join(acmeDir, acmeURI, filepath.Base(path))
}

func writeToken(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(tokenFile(path)), 0755); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}
	if err := os.WriteFile(tokenFile(path), []byte(token), 0644); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}
	return nil
}

func removeToken(path string) error {
	if err := os.Remove(tokenFile(path)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

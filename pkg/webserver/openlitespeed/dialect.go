package openlitespeed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// acmeConf mod_acme 配置，无状态 HTTP-01：按账户指纹直接应答 token.thumbprint
const acmeConf = panelConfDir + "/acme.conf"

const acmeProbe = "ace-probe"

// acmeLeaseTTL 租约过期兜底，验证出错时 CleanUp 不会被调用
const acmeLeaseTTL = 2 * time.Minute

// acmeLease mod_acme 一次只认一个指纹，验证进行中不允许切换
var acmeLease struct {
	sync.Mutex
	thumb   string
	active  int
	expires time.Time
}

type Dialect struct{}

func (Dialect) Service() string {
	return "openlitespeed"
}

func (Dialect) ConfigTest() string {
	return ServerRoot + "/bin/openlitespeed -t 2>&1"
}

func (Dialect) HTMLDir() string {
	return htmlDir
}

func (Dialect) ConfigFile() string {
	return vhostConfName
}

func (Dialect) PanelACMEConf() string {
	return acmeConf
}

func (Dialect) Features() types.Features {
	return types.Features{IPv6Listen: true, LSCache: true}
}

func (Dialect) HTTPSListenArgs() []string {
	return []string{"ssl", "quic"}
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

// LSCacheConf 缓存目录按站点隔离，由 OLS 自行创建
func (Dialect) LSCacheConf(name string) string {
	cfg := &conf.Config{}
	m := cfg.AddBlock("module", "cache")
	m.Add("ls_enabled", "1")
	m.Add("storagePath", ServerRoot+"/cachedata/"+name)
	return Export(cfg)
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

func (Dialect) HTPasswdLine(username, password string) string {
	return username + ":" + password
}

// RewritesDir 重写规则与 Apache 语法兼容，复用 Apache 预置
func (Dialect) RewritesDir() string {
	return "apache"
}

func (Dialect) BeforeReload() error {
	return syncPanelConf()
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

// WriteSiteChallenge 只需写入账户指纹，运行中的 OLS 已按该指纹应答时不重载
func (Dialect) WriteSiteChallenge(_, path, keyAuth string) (bool, error) {
	thumb, err := acquireThumbprint(path, keyAuth)
	if err != nil {
		return false, err
	}
	return !acmeLive(thumb), nil
}

func (Dialect) RemoveSiteChallenge(_, _, _ string) (bool, error) {
	releaseThumbprint()
	return false, nil
}

// WritePanelChallenge 同一订单只占用一次指纹
func (Dialect) WritePanelChallenge(_ string, _ []string, tokens map[string]string) (bool, error) {
	for path, keyAuth := range tokens {
		thumb, err := acquireThumbprint(path, keyAuth)
		if err != nil {
			return false, err
		}
		return !acmeLive(thumb), nil
	}
	return false, nil
}

func (Dialect) RemovePanelChallenge(_ string) (bool, error) {
	releaseThumbprint()
	return false, nil
}

// acquireThumbprint 等到允许切换后写入指纹，keyAuth 形如 token.thumbprint
func acquireThumbprint(path, keyAuth string) (string, error) {
	thumb, ok := strings.CutPrefix(keyAuth, filepath.Base(path)+".")
	if !ok || thumb == "" {
		return "", fmt.Errorf("invalid key authorization for %s", path)
	}

	l := &acmeLease
	l.Lock()
	defer l.Unlock()
	for l.active > 0 && l.thumb != thumb && time.Now().Before(l.expires) {
		l.Unlock()
		time.Sleep(time.Second)
		l.Lock()
	}

	cfg := &conf.Config{}
	m := cfg.AddBlock("module", "mod_acme")
	m.Add("ls_enabled", "1")
	m.Add("acmeEnable", "1")
	m.Add("acmeThumbPrint", thumb)
	if err := os.WriteFile(acmeConf, []byte(Export(cfg)), 0600); err != nil {
		return "", fmt.Errorf("failed to write acme config: %w", err)
	}
	if l.thumb != thumb {
		l.thumb, l.active = thumb, 0
	}
	l.active++
	l.expires = time.Now().Add(acmeLeaseTTL)
	return thumb, nil
}

func releaseThumbprint() {
	acmeLease.Lock()
	if acmeLease.active > 0 {
		acmeLease.active--
	}
	acmeLease.Unlock()
}

// acmeLive 探测不通一律视为需要重载
func acmeLive(thumb string) bool {
	client := http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1/.well-known/acme-challenge/"+acmeProbe, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return err == nil && resp.StatusCode == http.StatusOK && string(body) == acmeProbe+"."+thumb
}

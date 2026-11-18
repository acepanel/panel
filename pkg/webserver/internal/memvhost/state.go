// Package memvhost 是 OpenLiteSpeed 与 Caddy 共用的站点内存状态。两者的站点文件都由状态整体生成、加载时解析回来，
// 这里只有字段与平凡的读写方法，加载、生成与保存留在各方言
package memvhost

import (
	"errors"
	"path/filepath"
	"slices"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// State 字段名避开了同名的读取方法
type State struct {
	types.Fragments
	SiteDir  string
	SiteName string

	DocRoot       string
	IndexFiles    []string
	AccessLogFile string
	ErrorLogFile  string
	Includes      []string // 手写的 include、import 行，重新生成时原样带上
	Listens       []types.Listen
	Domains       []string
	TLS           *types.SSLConfig
	Auths         []types.BasicAuth
	PHPVersion    uint
	ProxyRules    []types.Proxy
	UpstreamList  []types.Upstream
	RedirectRules []types.Redirect
}

// New 新站点的默认状态
func New(configDir string) State {
	siteDir := filepath.Dir(configDir)
	return State{
		ConfigDir:     configDir,
		SiteDir:       siteDir,
		SiteName:      filepath.Base(siteDir),
		DocRoot:       filepath.Join(siteDir, "public"),
		IndexFiles:    []string{"index.html"},
		AccessLogFile: filepath.Join(siteDir, "log", "access.log"),
		ErrorLogFile:  filepath.Join(siteDir, "log", "error.log"),
		Listens:       []types.Listen{{Address: "80", Args: []string{}}},
		Domains:       []string{"localhost"},
	}
}

func (s *State) Listen() []types.Listen {
	return s.Listens
}

func (s *State) SetListen(listens []types.Listen) error {
	s.Listens = make([]types.Listen, 0, len(listens))
	for _, l := range listens {
		args := slices.Clone(l.Args)
		if args == nil {
			args = []string{}
		}
		s.Listens = append(s.Listens, types.Listen{Address: l.Address, Args: args})
	}
	return nil
}

func (s *State) ServerName() []string {
	return s.Domains
}

func (s *State) SetServerName(serverName []string) error {
	if len(serverName) == 0 {
		return nil
	}
	s.Domains = slices.Clone(serverName)
	return nil
}

func (s *State) Index() []string {
	if s.IndexFiles == nil {
		return []string{}
	}
	return s.IndexFiles
}

func (s *State) SetIndex(index []string) error {
	s.IndexFiles = slices.Clone(index)
	return nil
}

func (s *State) Root() string {
	return s.DocRoot
}

func (s *State) SetRoot(root string) error {
	s.DocRoot = root
	return nil
}

func (s *State) AccessLog() string {
	return s.AccessLogFile
}

func (s *State) SetAccessLog(accessLog string) error {
	s.AccessLogFile = accessLog
	return nil
}

func (s *State) ErrorLog() string {
	return s.ErrorLogFile
}

func (s *State) SetErrorLog(errorLog string) error {
	s.ErrorLogFile = errorLog
	return nil
}

func (s *State) SSL() bool {
	return s.TLS != nil
}

func (s *State) SSLConfig() *types.SSLConfig {
	if s.TLS == nil {
		return nil
	}
	cfg := *s.TLS
	return &cfg
}

func (s *State) SetSSLConfig(cfg *types.SSLConfig) error {
	if cfg == nil {
		return errors.New("SSL config cannot be nil")
	}
	ssl := *cfg
	if len(ssl.Protocols) == 0 {
		ssl.Protocols = types.DefaultSSLProtocols()
	}
	s.TLS = &ssl
	return nil
}

func (s *State) ClearSSL() error {
	s.TLS = nil
	return nil
}

// RateLimit 两者的限速都不在站点级：OLS 只有服务器级，Caddy 核心没有并发与带宽限制
func (s *State) RateLimit() *types.RateLimit {
	return nil
}

func (s *State) SetRateLimit(*types.RateLimit) error {
	return nil
}

func (s *State) ClearRateLimit() error {
	return nil
}

func (s *State) BasicAuth() []types.BasicAuth {
	return s.Auths
}

func (s *State) SetBasicAuth(auths []types.BasicAuth) error {
	s.Auths = slices.Clone(auths)
	return nil
}

// RealIP 两者都只能在服务器级信任代理头：OLS 见 realip.go，Caddy 在全局 servers 选项里
func (s *State) RealIP() *types.RealIP {
	return nil
}

func (s *State) SetRealIP(*types.RealIP) error {
	return nil
}

func (s *State) ClearRealIP() error {
	return nil
}

func (s *State) Redirects() []types.Redirect {
	return s.RedirectRules
}

func (s *State) SetRedirects(redirects []types.Redirect) error {
	s.RedirectRules = slices.Clone(redirects)
	return nil
}

func (s *State) Proxies() []types.Proxy {
	return s.ProxyRules
}

func (s *State) Upstreams() []types.Upstream {
	return s.UpstreamList
}

func (s *State) SetUpstreams(upstreams []types.Upstream) error {
	s.UpstreamList = slices.Clone(upstreams)
	return nil
}

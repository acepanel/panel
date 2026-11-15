package nginx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

const (
	altSvcHeader  = "Alt-Svc"
	httpsRedirect = "https://$host$request_uri"
)

type staticVhost struct {
	*baseVhost
}

type phpVhost struct {
	*baseVhost
}

type proxyVhost struct {
	*baseVhost
}

// 主文件解析成语法树后就地修改，用户手写的指令得以保留；保存时按 order 表排序输出
type baseVhost struct {
	types.Fragments
	cfg      *conf.Config
	server   *conf.Block // 主 server 块
	siteName string
}

func newStaticVhost(configDir string) (*staticVhost, error) {
	base, err := newBaseVhost(configDir)
	if err != nil {
		return nil, err
	}

	return &staticVhost{baseVhost: base}, nil
}

func newPHPVhost(configDir string) (*phpVhost, error) {
	base, err := newBaseVhost(configDir)
	if err != nil {
		return nil, err
	}

	return &phpVhost{baseVhost: base}, nil
}

func newProxyVhost(configDir string) (*proxyVhost, error) {
	base, err := newBaseVhost(configDir)
	if err != nil {
		return nil, err
	}

	return &proxyVhost{baseVhost: base}, nil
}

func newBaseVhost(configDir string) (*baseVhost, error) {
	if configDir == "" {
		return nil, errors.New("config directory is required")
	}

	v := &baseVhost{
		ConfigDir: configDir,
		siteName:  filepath.Base(filepath.Dir(configDir)),
	}

	configFile := filepath.Join(configDir, "nginx.conf")
	if _, err := os.Stat(configFile); err == nil {
		cfg, err := ParseFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load nginx config: %w", err)
		}
		v.use(cfg)
		return v, nil
	}

	if err := v.reset(); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *phpVhost) PHP() uint {
	return types.PHPVersionFromSocket(v.Config("010-php.conf", types.ScopeSite))
}

func (v *phpVhost) SetPHP(version uint) error {
	if version == 0 {
		return v.RemoveConfig("010-php.conf", types.ScopeSite)
	}

	content := fmt.Sprintf(`location ~ \.php$ {
    try_files $uri =404;
    fastcgi_pass unix:/tmp/php-cgi-%d.sock;
    fastcgi_index index.php;
    fastcgi_param HTTP_HOST $host;
    include fastcgi.conf;
    include pathinfo.conf;
}
`, version)

	return v.SetConfig("010-php.conf", types.ScopeSite, content)
}

// SetEnable 停用时把证书验证之外的请求都重写到停用页，停用页不要认证
func (v *baseVhost) SetEnable(enable bool) error {
	return v.SetEnableWith(enable, fmt.Sprintf(`rewrite ^(?!/\.well-known/acme-challenge/) /stop.html last;
location = /stop.html {
    root %s;
    auth_basic off;
}
`, filepath.Dir(disablePage)))
}

// Default 任一 listen 带 default_server 即为默认站点
func (v *baseVhost) Default() bool {
	return slices.ContainsFunc(v.Listen(), func(l types.Listen) bool {
		return slices.Contains(l.Args, "default_server")
	})
}

// SetDefault 为全部 listen 增删 default_server
func (v *baseVhost) SetDefault(enable bool) error {
	listens := v.Listen()
	for i := range listens {
		listens[i].Args = slices.DeleteFunc(listens[i].Args, func(arg string) bool { return arg == "default_server" })
		if enable {
			listens[i].Args = append(listens[i].Args, "default_server")
		}
	}
	return v.SetListen(listens)
}

// Listen 同一地址的多条 listen 合并为一条，参数去重
func (v *baseVhost) Listen() []types.Listen {
	var result []types.Listen
	index := make(map[string]int)
	for _, d := range v.server.GetAll("listen") {
		args := d.Values()
		if len(args) == 0 {
			continue
		}
		address := args[0]
		if i, ok := index[address]; ok {
			for _, arg := range args[1:] {
				if !slices.Contains(result[i].Args, arg) {
					result[i].Args = append(result[i].Args, arg)
				}
			}
			continue
		}
		index[address] = len(result)
		result = append(result, types.Listen{Address: address, Args: append([]string{}, args[1:]...)})
	}

	return result
}

func (v *baseVhost) SetListen(listens []types.Listen) error {
	v.server.Remove("listen")
	for _, l := range listens {
		hasSSL := slices.Contains(l.Args, "ssl")
		hasQUIC := slices.Contains(l.Args, "quic")

		// nginx 不允许 ssl 与 quic 同在一条 listen，拆成两行
		if hasSSL && hasQUIC {
			sslArgs := slices.DeleteFunc(slices.Clone(l.Args), func(arg string) bool { return arg == "quic" })
			quicArgs := slices.DeleteFunc(slices.Clone(l.Args), func(arg string) bool { return arg != "quic" && arg != "default_server" })
			v.server.Add("listen", append([]string{l.Address}, sslArgs...)...)
			v.server.Add("listen", append([]string{l.Address}, quicArgs...)...)
			continue
		}
		v.server.Add("listen", append([]string{l.Address}, l.Args...)...)
	}

	return nil
}

func (v *baseVhost) ServerName() []string {
	return v.server.Get("server_name").Values()
}

func (v *baseVhost) SetServerName(serverName []string) error {
	v.server.Set("server_name", serverName...)
	return nil
}

func (v *baseVhost) Index() []string {
	return v.server.Get("index").Values()
}

func (v *baseVhost) SetIndex(index []string) error {
	v.server.Set("index", index...)
	return nil
}

func (v *baseVhost) Root() string {
	return v.server.Value("root")
}

func (v *baseVhost) SetRoot(root string) error {
	v.server.Set("root", root)
	return nil
}

func (v *baseVhost) AccessLog() string {
	return v.fragmentValue("020-access-log.conf", "access_log")
}

func (v *baseVhost) SetAccessLog(accessLog string) error {
	if accessLog == "" {
		return v.RemoveConfig("020-access-log.conf", types.ScopeSite)
	}
	return v.SetConfig("020-access-log.conf", types.ScopeSite, "access_log "+accessLog+";\n")
}

func (v *baseVhost) ErrorLog() string {
	return v.fragmentValue("020-error-log.conf", "error_log")
}

func (v *baseVhost) SetErrorLog(errorLog string) error {
	if errorLog == "" {
		return v.RemoveConfig("020-error-log.conf", types.ScopeSite)
	}
	return v.SetConfig("020-error-log.conf", types.ScopeSite, "error_log "+errorLog+";\n")
}

func (v *baseVhost) Save() error {
	if err := os.WriteFile(filepath.Join(v.ConfigDir, "nginx.conf"), []byte(Render(v.cfg)), 0600); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}
	return nil
}

func (v *baseVhost) SSL() bool {
	return v.server.Value("ssl_certificate") != ""
}

func (v *baseVhost) SSLConfig() *types.SSLConfig {
	if !v.SSL() {
		return nil
	}

	cfg := &types.SSLConfig{
		Protocols: v.server.Get("ssl_protocols").Values(),
		OCSP:      v.server.Value("ssl_stapling") == "on",
	}
	for _, d := range v.server.GetAll("add_header") {
		switch d.Arg(0) {
		case types.HSTSHeader:
			cfg.HSTS = true
		case altSvcHeader:
			cfg.AltSvc = d.Arg(1)
		}
	}
	for _, d := range v.server.GetAll("if") {
		cfg.HTTPRedirect = cfg.HTTPRedirect || hasReturnTo(d, httpsRedirect)
	}

	return cfg
}

func (v *baseVhost) SetSSLConfig(cfg *types.SSLConfig) error {
	if cfg == nil {
		return errors.New("SSL config cannot be nil")
	}

	if err := v.ClearSSL(); err != nil {
		return err
	}
	if len(cfg.Protocols) == 0 {
		cfg.Protocols = types.DefaultSSLProtocols()
	}

	v.server.Add("ssl_certificate", cfg.Cert)
	v.server.Add("ssl_certificate_key", cfg.Key)
	v.server.Add("ssl_protocols", cfg.Protocols...)
	if cfg.HSTS {
		v.server.Add("add_header", types.HSTSHeader, types.HSTSValue)
	}
	if cfg.OCSP {
		v.server.Add("ssl_stapling", "on")
		v.server.Add("ssl_stapling_verify", "on")
	}
	if cfg.AltSvc != "" {
		v.server.Add("add_header", altSvcHeader, cfg.AltSvc)
	}
	v.setHTTPSRedirect(cfg.HTTPRedirect)

	return nil
}

func (v *baseVhost) ClearSSL() error {
	for _, name := range []string{
		"ssl_certificate", "ssl_certificate_key", "ssl_session_timeout", "ssl_session_cache", "ssl_protocols",
		"ssl_ciphers", "ssl_prefer_server_ciphers", "ssl_early_data", "ssl_stapling", "ssl_stapling_verify",
	} {
		v.server.Remove(name)
	}
	v.server.RemoveFunc("add_header", sslHeader)
	v.setHTTPSRedirect(false)

	return nil
}

func (v *baseVhost) RateLimit() *types.RateLimit {
	limit := &types.RateLimit{}
	if rate := v.server.Value("limit_rate"); rate != "" {
		limit.Rate, _ = strconv.Atoi(strings.TrimRight(rate, "kK"))
	}
	for _, d := range v.server.GetAll("limit_conn") {
		val, _ := strconv.Atoi(d.Arg(1))
		switch d.Arg(0) {
		case "perserver":
			limit.PerServer = val
		case "perip":
			limit.PerIP = val
		}
	}

	if limit.PerServer == 0 && limit.PerIP == 0 && limit.Rate == 0 {
		return nil
	}
	return limit
}

func (v *baseVhost) SetRateLimit(limit *types.RateLimit) error {
	_ = v.ClearRateLimit()
	if limit.Rate > 0 {
		v.server.Add("limit_rate", strconv.Itoa(limit.Rate)+"k")
	}
	if limit.PerServer > 0 {
		v.server.Add("limit_conn", "perserver", strconv.Itoa(limit.PerServer))
	}
	if limit.PerIP > 0 {
		v.server.Add("limit_conn", "perip", strconv.Itoa(limit.PerIP))
	}
	return nil
}

func (v *baseVhost) ClearRateLimit() error {
	v.server.Remove("limit_rate")
	v.server.Remove("limit_conn")
	return nil
}

func (v *baseVhost) BasicAuth() []types.BasicAuth {
	// 新版配置：解析 shared 片段中的 map
	if content := v.Config(authConfName, types.ScopeShared); content != "" {
		return parseAuthMaps(content)
	}

	// 兼容旧版 server 级整站配置
	file := v.server.Value("auth_basic_user_file")
	if file == "" || strings.HasPrefix(file, "$") {
		return nil
	}
	return []types.BasicAuth{{Path: "/", UserFile: file}}
}

func (v *baseVhost) SetBasicAuth(auths []types.BasicAuth) error {
	if err := v.clearBasicAuth(); err != nil {
		return err
	}
	if len(auths) == 0 {
		return nil
	}

	// map 变量在 http 上下文按 $uri 求值，server 级引用后继承入所有 location（含 PHP、反向代理）
	if err := v.SetConfig(authConfName, types.ScopeShared, generateAuthMaps(v.siteName, auths)); err != nil {
		return err
	}

	realmVar, fileVar := authVarNames(v.siteName)
	v.server.Add("auth_basic", realmVar)
	v.server.Add("auth_basic_user_file", fileVar)
	return nil
}

func (v *baseVhost) RealIP() *types.RealIP {
	realIP := &types.RealIP{
		Header:    v.server.Value("real_ip_header"),
		Recursive: v.server.Value("real_ip_recursive") == "on",
	}
	for _, d := range v.server.GetAll("set_real_ip_from") {
		if ip := d.Arg(0); ip != "" {
			realIP.From = append(realIP.From, ip)
		}
	}

	if len(realIP.From) == 0 && realIP.Header == "" {
		return nil
	}
	return realIP
}

func (v *baseVhost) SetRealIP(realIP *types.RealIP) error {
	_ = v.ClearRealIP()
	if realIP == nil {
		return nil
	}

	for _, ip := range realIP.From {
		if ip != "" {
			v.server.Add("set_real_ip_from", ip)
		}
	}
	if realIP.Header != "" {
		v.server.Add("real_ip_header", realIP.Header)
	}
	if realIP.Recursive {
		v.server.Add("real_ip_recursive", "on")
	}
	return nil
}

func (v *baseVhost) ClearRealIP() error {
	v.server.Remove("set_real_ip_from")
	v.server.Remove("real_ip_header")
	v.server.Remove("real_ip_recursive")
	return nil
}

func (v *baseVhost) Redirects() []types.Redirect {
	return parseRedirectFiles(filepath.Join(v.ConfigDir, "site"))
}

func (v *baseVhost) SetRedirects(redirects []types.Redirect) error {
	return writeRedirectFiles(filepath.Join(v.ConfigDir, "site"), redirects)
}

func (v *baseVhost) Proxies() []types.Proxy {
	return parseProxyFiles(filepath.Join(v.ConfigDir, "site"))
}

func (v *baseVhost) SetProxies(proxies []types.Proxy) error {
	return writeProxyFiles(filepath.Join(v.ConfigDir, "site"), proxies)
}

func (v *baseVhost) Upstreams() []types.Upstream {
	return parseUpstreamFiles(filepath.Join(v.ConfigDir, "shared"))
}

func (v *baseVhost) SetUpstreams(upstreams []types.Upstream) error {
	return writeUpstreamFiles(filepath.Join(v.ConfigDir, "shared"), upstreams)
}

// use 切换到新的语法树，没有 server 块时补一个
func (v *baseVhost) use(cfg *conf.Config) {
	v.cfg = cfg
	server := cfg.GetBlock("server")
	if server == nil {
		server = cfg.AddBlock("server")
	}
	v.server = server.Block
}

func (v *baseVhost) reset() error {
	cfg, err := Parse(strings.ReplaceAll(defaultConf, "/opt/ace/sites/default", "/opt/ace/sites/"+v.siteName))
	if err != nil {
		return fmt.Errorf("failed to load default config: %w", err)
	}
	v.use(cfg)
	return nil
}

func (v *baseVhost) fragmentValue(name, directive string) string {
	cfg, err := Parse(v.Config(name, types.ScopeSite))
	if err != nil {
		return ""
	}
	return cfg.Value(directive)
}

// setHTTPSRedirect 用 if ($scheme = http) 跳转普通 HTTP，用 error_page 497 跳转发到 HTTPS 端口的明文请求
func (v *baseVhost) setHTTPSRedirect(enable bool) {
	const https497Target = "https://$host:$server_port$request_uri"

	var found bool
	v.server.RemoveFunc("if", func(d *conf.Directive) bool {
		if !enable {
			if slices.Equal(d.Values(), []string{"($scheme", "=", "http)"}) {
				return true
			}
			d.RemoveFunc("return", func(r *conf.Directive) bool { return slices.Contains(r.Values(), httpsRedirect) })
			return false
		}
		found = found || hasReturnTo(d, httpsRedirect)
		return false
	})
	if enable && !found {
		v.server.AddBlock("if", "($scheme", "=", "http)").Add("return", "308", httpsRedirect)
	}

	found = false
	v.server.RemoveFunc("error_page", func(d *conf.Directive) bool {
		is497 := slices.Contains(d.Values(), "497") && slices.Contains(d.Values(), https497Target)
		found = found || is497
		return is497 && !enable
	})
	if enable && !found {
		v.server.Add("error_page", "497", "=308", https497Target)
	}
}

func (v *baseVhost) clearBasicAuth() error {
	v.server.Remove("auth_basic")
	v.server.Remove("auth_basic_user_file")
	return v.RemoveConfig(authConfName, types.ScopeShared)
}

func hasReturnTo(d *conf.Directive, target string) bool {
	return slices.ContainsFunc(d.GetAll("return"), func(r *conf.Directive) bool {
		return slices.Contains(r.Values(), target)
	})
}

func sslHeader(d *conf.Directive) bool {
	return d.Arg(0) == types.HSTSHeader || d.Arg(0) == altSvcHeader
}

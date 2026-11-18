package apache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// userFileSuffix 加密用户文件的后缀，与面板可回读的明文 htpasswd 并存
const userFileSuffix = ".apache"

// siteAuthPattern 整站认证要放过证书验证路径，否则带认证的站点签不了证书
const siteAuthPattern = `^/(?!\.well-known/acme-challenge/)`

type staticVhost struct {
	*baseVhost
}

type phpVhost struct {
	*baseVhost
}

type proxyVhost struct {
	*baseVhost
}

type baseVhost struct {
	types.Fragments
	config     *conf.Config
	vhost      *conf.Directive // 主 VirtualHost 块，写盘时按端口拆成明文与 TLS 两个块
	httpsPorts map[string]bool // 走 TLS 的端口
	siteName   string
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
		ConfigDir:  configDir,
		httpsPorts: make(map[string]bool),
		siteName:   filepath.Base(filepath.Dir(configDir)),
	}

	var config *conf.Config
	configFile := filepath.Join(configDir, configName)
	if _, err := os.Stat(configFile); err == nil {
		config, err = ParseFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse apache config: %w", err)
		}
	}

	if config == nil {
		var err error
		config, err = ParseString(v.defaultConf())
		if err != nil {
			return nil, fmt.Errorf("failed to parse default config: %w", err)
		}
	}

	v.config = config
	v.loadVhosts()

	return v, nil
}

func (v *phpVhost) PHP() uint {
	cfg, err := ParseString(v.Config("010-php.conf", types.ScopeSite))
	if err != nil {
		return 0
	}
	d := cfg.FindOne("FilesMatch.SetHandler")
	if d == nil || len(d.Args) == 0 {
		return 0
	}
	return types.PHPVersionFromSocket(d.Args[0].Value)
}

func (v *phpVhost) SetPHP(version uint) error {
	if version == 0 {
		return v.RemoveConfig("010-php.conf", types.ScopeSite)
	}

	handler := fmt.Sprintf("proxy:unix:/tmp/php-cgi-%d.sock|fcgi://localhost/", version)
	cfg := &conf.Config{}
	cfg.Append(conf.Blk("FilesMatch", `\.php$`).Append(
		&conf.Directive{Name: "SetHandler", Args: []conf.Arg{dquote(handler)}},
	))
	return v.SetConfig("010-php.conf", "site", Export(cfg)+"\n")
}

func (v *phpVhost) Rewrite() string {
	return types.Htaccess(v.Root()).Read()
}

func (v *phpVhost) SetRewrite(content string) error {
	return types.Htaccess(v.Root()).Write(content)
}

// SetEnable 停用时把证书验证之外的请求都重写到停用页。停用页不要认证，也不受反代规则的 IP 限制：
// <If> 在所有 <Location> 之后合并，嵌在 <Directory> 里的又排在站点自己的 <If> 之后，这里的放行最后生效
func (v *baseVhost) SetEnable(enable bool) error {
	return v.SetEnableWith(enable, fmt.Sprintf(`RewriteEngine On
RewriteCond %%{REQUEST_URI} !%s
RewriteRule ^ %s [L]
<Directory %s>
    <If "true">
        Require all granted
    </If>
</Directory>
`, acmePattern, disablePage, htmlDir))
}

// Default 默认站点切换只在 nginx 与 Caddy 支持
func (v *baseVhost) Default() bool {
	return false
}

func (v *baseVhost) SetDefault(bool) error {
	return nil
}

func (v *baseVhost) Listen() []types.Listen {
	var result []types.Listen
	// Apache 的监听写在 <VirtualHost *:80> 标签参数中，是否走 TLS 取自加载时记下的 httpsPorts
	for _, addr := range v.vhost.Values() {
		args := []string{}
		if v.httpsPorts[portOf(addr)] {
			args = append(args, "ssl")
		}
		result = append(result, types.Listen{Address: addr, Args: args})
	}
	return result
}

func (v *baseVhost) SetListen(listens []types.Listen) error {
	var args []string
	clear(v.httpsPorts)
	for _, l := range listens {
		addr := l.Address
		// 只有端口号时补 *: 前缀
		if !strings.Contains(addr, ":") {
			addr = "*:" + addr
		}
		args = append(args, addr)
		if slices.Contains(l.Args, "ssl") {
			v.httpsPorts[portOf(addr)] = true
		}
	}
	v.vhost.SetArgs(args...)
	return nil
}

func (v *baseVhost) ServerName() []string {
	names := make([]string, 0)
	if name := v.vhost.Value("ServerName"); name != "" {
		names = append(names, name)
	}
	for _, alias := range v.vhost.GetAll("ServerAlias") {
		names = append(names, alias.Values()...)
	}
	return names
}

func (v *baseVhost) SetServerName(serverName []string) error {
	if len(serverName) == 0 {
		return nil
	}
	v.vhost.Set("ServerName", serverName[0])
	v.vhost.Remove("ServerAlias")
	if len(serverName) > 1 {
		v.vhost.Add("ServerAlias", serverName[1:]...)
	}
	return nil
}

func (v *baseVhost) Index() []string {
	if values := v.vhost.Get("DirectoryIndex").Values(); values != nil {
		return values
	}
	return []string{}
}

func (v *baseVhost) SetIndex(index []string) error {
	if len(index) == 0 {
		v.vhost.Remove("DirectoryIndex")
		return nil
	}
	v.vhost.Set("DirectoryIndex", index...)
	return nil
}

func (v *baseVhost) Root() string {
	return v.vhost.Value("DocumentRoot")
}

func (v *baseVhost) SetRoot(root string) error {
	v.vhost.Set("DocumentRoot", root)

	// 同步更新 Directory 块路径
	if dirBlock := v.vhost.GetBlock("Directory"); dirBlock != nil {
		dirBlock.SetArgs(root)
	} else {
		v.vhost.AddBlock("Directory", root).Append(
			conf.Dir("Options", "-Indexes", "+FollowSymLinks"),
			conf.Dir("AllowOverride", "All"),
			conf.Dir("Require", "all", "granted"),
		)
	}
	return nil
}

func (v *baseVhost) AccessLog() string {
	cfg, err := ParseString(v.Config("020-access-log.conf", types.ScopeSite))
	if err != nil {
		return ""
	}
	if d := cfg.FindOne("CustomLog"); d != nil && len(d.Args) > 0 {
		return d.Args[0].Value
	}
	return ""
}

func (v *baseVhost) SetAccessLog(accessLog string) error {
	if accessLog == "" {
		return v.RemoveConfig("020-access-log.conf", types.ScopeSite)
	}
	cfg := &conf.Config{}
	cfg.Append(conf.Dir("CustomLog", accessLog, "combined"))
	return v.SetConfig("020-access-log.conf", "site", Export(cfg)+"\n")
}

func (v *baseVhost) ErrorLog() string {
	cfg, err := ParseString(v.Config("020-error-log.conf", types.ScopeSite))
	if err != nil {
		return ""
	}
	if d := cfg.FindOne("ErrorLog"); d != nil && len(d.Args) > 0 {
		return d.Args[0].Value
	}
	return ""
}

func (v *baseVhost) SetErrorLog(errorLog string) error {
	if errorLog == "" {
		return v.RemoveConfig("020-error-log.conf", types.ScopeSite)
	}
	cfg := &conf.Config{}
	cfg.Append(conf.Dir("ErrorLog", errorLog))
	return v.SetConfig("020-error-log.conf", "site", Export(cfg)+"\n")
}

func (v *baseVhost) Save() error {
	configFile := filepath.Join(v.ConfigDir, configName)
	if err := os.WriteFile(configFile, []byte(Render(v.expand())+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}
	return nil
}

func (v *baseVhost) SSL() bool {
	return v.vhost.Has("SSLEngine") && strings.EqualFold(v.vhost.Value("SSLEngine"), "on")
}

func (v *baseVhost) SSLConfig() *types.SSLConfig {
	if !v.SSL() {
		return nil
	}

	config := &types.SSLConfig{
		Cert: v.vhost.Value("SSLCertificateFile"),
		Key:  v.vhost.Value("SSLCertificateKeyFile"),
	}
	config.Protocols = parseSSLProtocols(v.vhost.Get("SSLProtocol").Values())
	config.OCSP = strings.EqualFold(v.vhost.Value("SSLUseStapling"), "on")

	if slices.ContainsFunc(v.vhost.GetAll("Header"), isHSTSHeader) {
		config.HSTS = true
	}
	for _, r := range v.vhost.GetAll("RewriteRule") {
		if argsContain(r.Args, "https://") {
			config.HTTPRedirect = true
			break
		}
	}

	return config
}

func (v *baseVhost) SetSSLConfig(cfg *types.SSLConfig) error {
	if cfg == nil {
		return errors.New("SSL config cannot be nil")
	}

	v.vhost.Set("SSLEngine", "on")
	if cfg.Cert != "" {
		v.vhost.Set("SSLCertificateFile", cfg.Cert)
	}
	if cfg.Key != "" {
		v.vhost.Set("SSLCertificateKeyFile", cfg.Key)
	}

	v.vhost.Set("SSLProtocol", sslProtocolArgs(cfg.Protocols)...)

	// 先移除旧的再按需添加，保留其他 Header；只加不减会让开关关不掉，且重复保存时不断累加
	v.vhost.RemoveFunc("Header", isHSTSHeader)
	if cfg.HSTS {
		v.vhost.Add("Header", "always", "set", types.HSTSHeader, types.HSTSValue)
	}

	// 依赖全局的 SSLStaplingCache，由安装脚本写入
	if cfg.OCSP {
		v.vhost.Set("SSLUseStapling", "on")
	} else {
		v.vhost.Remove("SSLUseStapling")
	}

	v.removeHTTPRedirect()
	if cfg.HTTPRedirect {
		v.vhost.Set("RewriteEngine", "on")
		v.vhost.Add("RewriteCond", "%{HTTPS}", "off")
		v.vhost.Add("RewriteRule", "^(.*)$", "https://%{HTTP_HOST}%{REQUEST_URI}", "[R=308,L]")
	}

	// 监听没标 ssl 时按 443 兜底，否则整站会落进明文块
	if len(v.httpsPorts) == 0 {
		v.httpsPorts["443"] = true
		if !v.hasPort("443") {
			v.vhost.AppendArg("*:443")
		}
	}

	return nil
}

func (v *baseVhost) ClearSSL() error {
	for _, name := range []string{"SSLEngine", "SSLCertificateFile", "SSLCertificateKeyFile", "SSLProtocol", "SSLCipherSuite", "SSLUseStapling"} {
		v.vhost.Remove(name)
	}
	v.vhost.RemoveFunc("Header", isHSTSHeader)
	v.removeHTTPRedirect()
	clear(v.httpsPorts)

	var newArgs []string
	for _, addr := range v.vhost.Values() {
		if portOf(addr) != "443" {
			newArgs = append(newArgs, addr)
		}
	}
	if len(newArgs) == 0 {
		newArgs = []string{"*:80"}
	}
	v.vhost.SetArgs(newArgs...)
	return nil
}

func (v *baseVhost) RateLimit() *types.RateLimit {
	if !slices.ContainsFunc(v.vhost.GetAll("SetOutputFilter"), isRateLimitFilter) {
		return nil
	}

	rateLimit := &types.RateLimit{}
	for _, d := range v.vhost.GetAll("SetEnv") {
		if isRateLimitEnv(d) {
			rateLimit.Rate, _ = strconv.Atoi(d.Arg(1))
		}
	}
	return rateLimit
}

func (v *baseVhost) SetRateLimit(limit *types.RateLimit) error {
	_ = v.ClearRateLimit()
	// Apache mod_ratelimit 只支持流量限制，不支持并发连接限制
	if limit.Rate > 0 {
		v.vhost.Add("SetOutputFilter", "RATE_LIMIT")
		v.vhost.Add("SetEnv", "rate-limit", strconv.Itoa(limit.Rate))
	}
	return nil
}

// ClearRateLimit 只动限速自己的两条，用户手写的 SetEnv、SetOutputFilter 原样保留
func (v *baseVhost) ClearRateLimit() error {
	v.vhost.RemoveFunc("SetOutputFilter", isRateLimitFilter)
	v.vhost.RemoveFunc("SetEnv", isRateLimitEnv)
	return nil
}

func (v *baseVhost) BasicAuth() []types.BasicAuth {
	var auths []types.BasicAuth
	for _, block := range v.vhost.All() {
		if !isAuthBlock(block) {
			continue
		}
		path := block.Arg(0)
		if path == siteAuthPattern {
			path = "/"
		}
		auths = append(auths, types.BasicAuth{Path: path, UserFile: strings.TrimSuffix(block.Value("AuthUserFile"), userFileSuffix)})
	}
	if len(auths) > 0 {
		return auths
	}

	// 兼容旧版 vhost 级整站配置
	if strings.EqualFold(v.vhost.Value("AuthType"), "Basic") {
		return []types.BasicAuth{{Path: "/", UserFile: v.vhost.Value("AuthUserFile")}}
	}
	return nil
}

func (v *baseVhost) SetBasicAuth(auths []types.BasicAuth) error {
	v.clearBasicAuth()
	// 容器块后声明者覆盖先声明者，按路径长度升序写出使更精确的目录规则生效；
	// 且它在合并顺序上晚于 Directory 块，认证不会被 Require all granted 覆盖
	auths = slices.SortedStableFunc(slices.Values(auths), func(a, b types.BasicAuth) int {
		return len(a.Path) - len(b.Path)
	})
	for _, auth := range auths {
		// 缺文件时照写规则，Apache 找不到 AuthUserFile 会拒绝访问，比放行安全
		if _, err := types.WriteUserFile(auth.UserFile, userFileSuffix, ":"); err != nil {
			return err
		}
		block := v.vhost.AddBlock("Location", auth.Path)
		if auth.Path == "/" {
			block.Name = "LocationMatch"
			block.SetArgs(siteAuthPattern)
		}
		block.Append(
			conf.Dir("AuthType", "Basic"),
			conf.Dir("AuthName", "Restricted"),
			conf.Dir("AuthUserFile", auth.UserFile+userFileSuffix),
			conf.Dir("Require", "valid-user"),
		)
	}
	return nil
}

func (v *baseVhost) RealIP() *types.RealIP {
	header := v.vhost.Value("RemoteIPHeader")
	if header == "" {
		return nil
	}

	var from []string
	for _, d := range v.vhost.GetAll("RemoteIPTrustedProxy") {
		if len(d.Args) > 0 {
			from = append(from, d.Args[0].Value)
		}
	}

	return &types.RealIP{From: from, Header: header}
}

func (v *baseVhost) SetRealIP(realIP *types.RealIP) error {
	v.vhost.Remove("RemoteIPHeader")
	v.vhost.Remove("RemoteIPTrustedProxy")

	if realIP == nil || (len(realIP.From) == 0 && realIP.Header == "") {
		return nil
	}

	if realIP.Header != "" {
		v.vhost.Set("RemoteIPHeader", realIP.Header)
	}
	for _, ip := range realIP.From {
		if ip != "" {
			v.vhost.Add("RemoteIPTrustedProxy", ip)
		}
	}
	return nil
}

func (v *baseVhost) ClearRealIP() error {
	v.vhost.Remove("RemoteIPHeader")
	v.vhost.Remove("RemoteIPTrustedProxy")
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
	return writeProxyFiles(filepath.Join(v.ConfigDir, "site"), proxies, v.upstreamNames())
}

func (v *baseVhost) Upstreams() []types.Upstream {
	return parseBalancerFiles(filepath.Join(v.ConfigDir, "shared"))
}

func (v *baseVhost) SetUpstreams(upstreams []types.Upstream) error {
	return writeBalancerFiles(filepath.Join(v.ConfigDir, "shared"), upstreams)
}

func (v *baseVhost) defaultConf() string {
	return strings.ReplaceAll(defaultVhostConf, "/opt/ace/sites/default", "/opt/ace/sites/"+v.siteName)
}

// loadVhosts 把写盘时拆开的明文与 TLS 两个 VirtualHost 合回单一逻辑视图
func (v *baseVhost) loadVhosts() {
	clear(v.httpsPorts)
	vhosts := v.config.Blocks("VirtualHost")
	if len(vhosts) == 0 {
		v.vhost = v.config.AddBlock("VirtualHost", "*:80")
		return
	}

	main := vhosts[0]
	for _, b := range vhosts {
		if strings.EqualFold(b.Value("SSLEngine"), "on") {
			main = b
			for _, addr := range b.Values() {
				v.httpsPorts[portOf(addr)] = true
			}
			break
		}
	}

	var addrs []string
	for _, b := range vhosts {
		if b != main {
			addrs = append(addrs, b.Values()...)
		}
	}
	if len(addrs) > 0 {
		main.SetArgs(append(addrs, main.Values()...)...)
		v.config.RemoveFunc("VirtualHost", func(d *conf.Directive) bool { return d != main })
	}
	v.vhost = main
}

// expand TLS 端口与明文端口各占一个 VirtualHost。SSLEngine 按 VirtualHost 生效，
// 同一个块里混用两种端口会让明文端口直接断连，连强制跳转都执行不到
func (v *baseVhost) expand() *conf.Config {
	plainAddrs, tlsAddrs := []string{}, []string{}
	for _, addr := range v.vhost.Values() {
		if v.httpsPorts[portOf(addr)] {
			tlsAddrs = append(tlsAddrs, addr)
		} else {
			plainAddrs = append(plainAddrs, addr)
		}
	}
	if len(plainAddrs) == 0 || len(tlsAddrs) == 0 {
		return v.config
	}

	plain := v.vhost.Clone()
	plain.SetArgs(plainAddrs...)
	plain.Filter(func(n conf.Node) bool {
		d, ok := n.(*conf.Directive)
		return !ok || !strings.HasPrefix(d.Name, "SSL")
	})
	plain.RemoveFunc("Header", isHSTSHeader)

	config := &conf.Config{Block: *v.config.Clone()}
	config.Blocks("VirtualHost")[0].SetArgs(tlsAddrs...)
	config.Append(&conf.Blank{}, plain)

	return config
}

func (v *baseVhost) removeHTTPRedirect() {
	v.vhost.RemoveFunc("RewriteCond", func(d *conf.Directive) bool {
		return d.Arg(0) == "%{HTTPS}" && strings.EqualFold(d.Arg(1), "off")
	})
	v.vhost.RemoveFunc("RewriteRule", func(d *conf.Directive) bool {
		return argsContain(d.Args, "https://%{HTTP_HOST}")
	})
}

func (v *baseVhost) hasPort(port string) bool {
	return slices.ContainsFunc(v.vhost.Values(), func(addr string) bool {
		return portOf(addr) == port
	})
}

func (v *baseVhost) clearBasicAuth() {
	// 清除旧版 vhost 级指令
	for _, name := range []string{"AuthType", "AuthName", "AuthUserFile"} {
		v.vhost.Remove(name)
	}
	v.vhost.RemoveFunc("Require", func(d *conf.Directive) bool {
		return len(d.Args) == 1 && d.Args[0].Value == "valid-user"
	})
	v.vhost.Filter(func(n conf.Node) bool {
		d, ok := n.(*conf.Directive)
		return !ok || !isAuthBlock(d)
	})
}

// upstreamNames 已写入的上游名，代理目标指向其中之一时要换成 balancer 地址
func (v *baseVhost) upstreamNames() []string {
	return balancerNames(filepath.Join(v.ConfigDir, "shared"))
}

// sslProtocolArgs 面板的协议列表转 Apache 语法。Apache 只认带 +/- 前缀的增减写法，
// 直接列出多个协议名会以 "Illegal protocol" 拒载整份配置
func sslProtocolArgs(protocols []string) []string {
	if len(protocols) == 0 {
		protocols = types.DefaultSSLProtocols()
	}
	return append([]string{"-all"}, lo.Map(protocols, func(p string, _ int) string {
		return "+" + strings.TrimPrefix(p, "+")
	})...)
}

// parseSSLProtocols 从 Apache 语法还原协议列表，只取显式开启的项
func parseSSLProtocols(args []string) []string {
	return lo.FilterMap(args, func(a string, _ int) (string, bool) {
		return strings.CutPrefix(a, "+")
	})
}

func isHSTSHeader(d *conf.Directive) bool {
	return argsContain(d.Args, types.HSTSHeader)
}

func isRateLimitFilter(d *conf.Directive) bool {
	return d.Arg(0) == "RATE_LIMIT"
}

func isRateLimitEnv(d *conf.Directive) bool {
	return d.Arg(0) == "rate-limit"
}

func isAuthBlock(d *conf.Directive) bool {
	return d.Block != nil && len(d.Args) > 0 && strings.EqualFold(d.Value("AuthType"), "Basic") &&
		(strings.EqualFold(d.Name, "Location") || strings.EqualFold(d.Name, "LocationMatch"))
}

func argsContain(args []conf.Arg, sub string) bool {
	for _, a := range args {
		if strings.Contains(a.Value, sub) {
			return true
		}
	}
	return false
}

// portOf 提取监听地址的端口部分，如 *:443 -> 443
func portOf(addr string) string {
	if _, after, ok := strings.CutLast(addr, ":"); ok {
		return after
	}
	return ""
}

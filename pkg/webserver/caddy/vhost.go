package caddy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/internal/memvhost"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

const (
	httpMatcher = "@ace_http"
	acmeMatcher = "@ace_acme"
	stopMatcher = "@ace_stop"
)

// userFileSuffix 供 basic_auth import 的用户文件后缀，与面板可回读的明文 htpasswd 并存
const userFileSuffix = ".caddy"

// accessLogFormat nginx combined 格式，由 transform-encoder 插件渲染，空字段输出 -
const accessLogFormat = `{request>remote_ip} - {user_id} [{ts}] "{request>method} {request>uri} {request>proto}" {status} {size} "{request>headers>Referer>[0]}" "{request>headers>User-Agent>[0]}"`

const accessLogTimeFormat = "02/Jan/2006:15:04:05 -0700"

var caddyProtocols = map[string]string{"TLSv1.2": "tls1.2", "TLSv1.3": "tls1.3"}

type staticVhost struct {
	*baseVhost
}

type phpVhost struct {
	*baseVhost
}

type proxyVhost struct {
	*baseVhost
}

// baseVhost 站点文件由内存状态整体生成，加载时解析回来
type baseVhost struct {
	memvhost.State
	safeName  string
	isDefault bool // 站点块额外带无主机名地址
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

	v := &baseVhost{State: memvhost.New(configDir)}
	v.safeName = types.SafeName(v.SiteName)

	path := filepath.Join(configDir, confName)
	if _, err := os.Stat(path); err == nil {
		cfg, err := ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse caddy config: %w", err)
		}
		v.load(cfg)
	}

	return v, nil
}

func (v *phpVhost) PHP() uint {
	return v.PHPVersion
}

func (v *phpVhost) SetPHP(version uint) error {
	v.PHPVersion = version
	return nil
}

// SetEnable 停用片段只是标记，只含注释，被 import 无副作用
func (v *baseVhost) SetEnable(enable bool) error {
	return v.SetEnableWith(enable, "")
}

func (v *baseVhost) Default() bool {
	return v.isDefault
}

func (v *baseVhost) SetDefault(enable bool) error {
	v.isDefault = enable
	return nil
}

// SetListen 绑定 IP 对整个站点块生效，监听地址要么都带 IP 要么都不带
func (v *baseVhost) SetListen(listens []types.Listen) error {
	var withHost, withoutHost int
	for _, l := range listens {
		if host, _ := splitHostPort(l.Address); host != "" {
			withHost++
		} else {
			withoutHost++
		}
	}
	if withHost > 0 && withoutHost > 0 {
		return errors.New("caddy binds addresses per site: listens must either all specify an IP or none")
	}
	// 所有站点共用一份主配置，非法端口会让整份配置拒载，必须在写盘前挡住
	for _, l := range listens {
		_, port := splitHostPort(l.Address)
		if value, err := strconv.ParseUint(port, 10, 16); err != nil || value == 0 {
			return fmt.Errorf("invalid listen address %q: caddy requires a port", l.Address)
		}
	}

	return v.State.SetListen(listens)
}

// ErrorLog Caddy 没有站点级错误日志，统一指向全局错误日志
func (v *baseVhost) ErrorLog() string {
	return ErrorLogPath
}

func (v *baseVhost) SetErrorLog(string) error {
	return nil
}

func (v *baseVhost) Save() error {
	users, err := v.writeUserFiles()
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(v.ConfigDir, confName), []byte(Export(v.build(users))), 0600); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}
	return nil
}

func (v *baseVhost) SetProxies(proxies []types.Proxy) error {
	for _, p := range proxies {
		if _, err := customNodes(p); err != nil {
			return err
		}
	}
	v.ProxyRules = slices.Clone(proxies)
	return nil
}

func (v *baseVhost) load(cfg *conf.Config) {
	v.loadUpstreams(cfg)
	sites := sites(cfg)
	if len(sites) == 0 {
		return
	}
	v.loadListen(sites)
	v.loadSSL(sites)

	// 站点主体在片段里，手写的最简配置则直接取站点块
	body := sites[0].Block
	if snippet := cfg.Get("(" + v.siteSnippet() + ")"); snippet != nil {
		body = snippet.Block
	}
	if d := body.Get("root"); d != nil {
		v.DocRoot = d.Arg(len(d.Values()) - 1)
	}
	v.IndexFiles = []string{}
	if fs := body.Get("file_server"); fs != nil {
		if idx := fs.Get("index"); idx != nil {
			v.IndexFiles = idx.Values()
		}
	}
	v.AccessLogFile = ""
	if l := body.Get("log"); l != nil {
		if out := l.Get("output"); out != nil && out.Arg(0) == "file" {
			v.AccessLogFile = out.Arg(1)
		}
	}
	for _, d := range body.GetAll("import") {
		if !strings.HasPrefix(d.Arg(0), v.ConfigDir+"/") {
			v.Includes = append(v.Includes, d.Arg(0))
		}
	}
	if d := body.Get("php_fastcgi"); d != nil {
		v.PHPVersion = types.PHPVersionFromSocket(d.Arg(0))
	}
	v.loadAuths(body)
	v.loadProxies(body)
	v.loadRedirects(body)
}

// loadListen 无 scheme 时 80 为 HTTP，其它端口带主机名或块内有 tls 即 HTTPS，与 Caddy 的推断一致
func (v *baseVhost) loadListen(sites []*conf.Directive) {
	var hosts, ports, binds []string
	ssl := map[string]bool{}
	var hostless bool
	for _, site := range sites {
		hasTLS := site.Get("tls") != nil
		for _, key := range addresses(site) {
			scheme, rest, found := strings.Cut(key, "://")
			if !found {
				scheme, rest = "", key
			}
			host, port := splitHostPort(rest)
			if port == "" {
				port = map[bool]string{true: "80", false: "443"}[scheme == "http"]
			}
			secure := scheme == "https" || (scheme == "" && port != "80" && (host != "" || hasTLS))
			if host == "" {
				hostless = true
			} else if !slices.Contains(hosts, host) {
				hosts = append(hosts, host)
			}
			if !slices.Contains(ports, port) {
				ports = append(ports, port)
			}
			ssl[port] = ssl[port] || secure
		}
		if d := site.Get("bind"); d != nil {
			for _, bind := range d.Values() {
				if !slices.Contains(binds, bind) {
					binds = append(binds, bind)
				}
			}
		}
	}
	if hosts == nil {
		hosts = []string{}
	}
	v.Domains = hosts
	// 有域名又带无主机名地址即默认站点；完全无域名的站点（如 phpMyAdmin）不算
	v.isDefault = hostless && len(hosts) > 0

	if len(binds) == 0 {
		binds = []string{""}
	}
	v.Listens = []types.Listen{}
	for _, port := range ports {
		args := []string{}
		if ssl[port] {
			args = []string{"ssl"}
		}
		for _, bind := range binds {
			address := port
			if bind != "" {
				address = net.JoinHostPort(bind, port)
			}
			v.Listens = append(v.Listens, types.Listen{Address: address, Args: slices.Clone(args)})
		}
	}
}

func (v *baseVhost) loadSSL(sites []*conf.Directive) {
	for _, site := range sites {
		tls := site.Get("tls")
		if tls == nil {
			continue
		}
		v.TLS = &types.SSLConfig{
			Cert:      tls.Arg(0),
			Key:       tls.Arg(1),
			Protocols: protocolsFromCaddy(nil),
			OCSP:      true,
		}
		if p := tls.Get("protocols"); p != nil {
			v.TLS.Protocols = protocolsFromCaddy(p.Values())
		}
		for _, d := range site.GetAll("header") {
			if slices.Contains(d.Values(), types.HSTSHeader) {
				v.TLS.HSTS = true
			}
		}
	}
	if v.TLS == nil {
		return
	}
	for _, site := range sites {
		for _, d := range site.GetAll("redir") {
			if d.Arg(0) == httpMatcher {
				v.TLS.HTTPRedirect = true
			}
		}
	}
}

func (v *baseVhost) loadAuths(body *conf.Block) {
	for _, d := range body.GetAll("basic_auth") {
		m := body.Get(d.Arg(0))
		if m == nil {
			continue
		}
		pattern := m.Arg(1)
		if m.Block != nil {
			if p := m.Get("path"); p != nil {
				pattern = p.Arg(0)
			}
		}
		imp := d.Get("import")
		if imp == nil {
			continue
		}
		v.Auths = append(v.Auths, types.BasicAuth{
			Path:     "/" + strings.Trim(strings.TrimSuffix(pattern, "*"), "/"),
			UserFile: strings.TrimSuffix(imp.Arg(0), ".caddy"),
		})
	}
}

// writeUserFiles 明文 htpasswd 另转一份 Caddy 能用的；返回各文件是否有用户
func (v *baseVhost) writeUserFiles() (map[string]bool, error) {
	users := make(map[string]bool, len(v.Auths))
	for _, auth := range v.Auths {
		hasUsers, err := types.WriteUserFile(auth.UserFile, userFileSuffix, " ")
		if err != nil {
			return nil, err
		}
		users[auth.UserFile] = hasUsers
	}
	return users, nil
}

// build 主体写成片段由 HTTP 与 HTTPS 两个站点块引用，同一块混用两种 scheme 会被合并进一个 server 而报错
func (v *baseVhost) build(users map[string]bool) *conf.Config {
	cfg := &conf.Config{}
	cfg.Append(conf.Cmt("Auto-generated by AcePanel. DO NOT EDIT MANUALLY!"))
	// 空通配每次重载都记一条警告，有文件时才引用
	if matches, _ := filepath.Glob(filepath.Join(v.ConfigDir, "shared", "*.conf")); len(matches) > 0 {
		cfg.Add("import", filepath.Join(v.ConfigDir, "shared", "*.conf"))
	}
	v.buildUpstreams(cfg)

	body := cfg.AddBlock("(" + v.siteSnippet() + ")")
	v.buildHead(body)
	body.Append(&conf.Blank{})
	body.Add("import", filepath.Join(v.ConfigDir, "site", "*.conf"))
	for _, path := range v.Includes {
		body.Add("import", path)
	}

	// 重定向排在片段之后：多个 handle_errors 后写的生效，与 nginx 的 error_page 覆盖语义一致
	v.buildRedirects(body.Block)
	v.buildAuths(body.Block, users)
	v.buildProxies(body.Block)

	body.Append(&conf.Blank{})
	if v.PHPVersion > 0 {
		body.Add("php_fastcgi", phpSocket(v.PHPVersion))
	}
	if len(v.IndexFiles) > 0 {
		body.AddBlock("file_server").Add("index", v.IndexFiles...)
	} else {
		body.Add("file_server")
	}

	// 停用时站点块改引只留验证与停用页的片段：redir 与 basic_auth 的执行顺序都排在 handle 之前，写进去就会先于停用页生效。
	// 主体片段照常生成但不被引用，站点设置靠它在重新启用时还原
	snippet := v.siteSnippet()
	if !v.Enable() {
		snippet += "_stop"
		stop := cfg.AddBlock("(" + snippet + ")")
		v.buildHead(stop)
		stop.Append(&conf.Comment{Text: " ace:stop"})
		stop.Add(stopMatcher, "expression", "true")
		h := stop.AddBlock("handle", stopMatcher)
		h.Add("rewrite", "*", "/stop.html")
		h.AddBlock("file_server").Add("root", htmlDir)
	}

	httpKeys, httpsKeys := v.siteKeys()
	binds := v.bindHosts()
	if len(httpKeys) > 0 {
		site := addSite(cfg, httpKeys...)
		if len(binds) > 0 {
			site.Add("bind", binds...)
		}
		if v.TLS != nil && v.TLS.HTTPRedirect && len(httpsKeys) > 0 {
			site.Add(httpMatcher, "not", "path", acmeURI+"*")
			site.Add("redir", httpMatcher, "https://{host}{uri}", "308")
		}
		site.Add("import", snippet)
	}
	if len(httpsKeys) > 0 {
		site := addSite(cfg, httpsKeys...)
		if len(binds) > 0 {
			site.Add("bind", binds...)
		}
		v.buildTLS(site)
		if v.TLS != nil && v.TLS.HSTS {
			site.Add("header", types.HSTSHeader, types.HSTSValue)
		}
		site.Add("import", snippet)
	}

	return cfg
}

// buildHead 日志、根目录与 ACME 验证，启用与停用的片段都以它开头
func (v *baseVhost) buildHead(body *conf.Directive) {
	if v.AccessLogFile != "" {
		l := body.AddBlock("log")
		l.Add("output", "file", v.AccessLogFile)
		f := l.AddBlock("format", "transform", accessLogFormat)
		f.Add("time_format", accessLogTimeFormat)
	}
	body.Add("root", "*", v.DocRoot)
	body.Add("encode", "br", "zstd", "gzip")

	// 按原始 URI 匹配以免被伪静态改写；root 只给 file_server 用，不影响错误页的站点根目录
	body.Append(&conf.Blank{})
	body.Add(acmeMatcher, "expression", `{http.request.orig_uri.path}.startsWith("`+acmeURI+`")`)
	acme := body.AddBlock("handle", acmeMatcher)
	acme.Add("rewrite", "*", "{http.request.orig_uri.path}")
	acme.AddBlock("file_server").Add("root", acmeDir)
}

// buildAuths 整站认证要放过验证路径；没有用户的规则退化为 401，与 nginx 的空 htpasswd 一致
func (v *baseVhost) buildAuths(body *conf.Block, users map[string]bool) {
	for i, auth := range v.Auths {
		name := v.authMatcher(i)
		pattern := "/" + strings.Trim(auth.Path, "/") + "*"
		if auth.Path == "/" {
			pattern = "/*"
			m := body.AddBlock(name)
			m.Add("path", pattern)
			m.Add("not", "path", acmeURI+"*")
		} else {
			body.Add(name, "path", pattern)
		}
		// 没有用户时直接 401，与 nginx 读到空 htpasswd 的行为一致，不能放行
		if !users[auth.UserFile] {
			body.Add("error", name, "401")
			continue
		}
		body.AddBlock("basic_auth", name).Add("import", caddyUserFile(auth.UserFile))
	}
}

// siteKeys 无域名或默认站点时带无主机名地址，Caddy 会把它排在带域名的块之后、主配置兜底块之前
func (v *baseVhost) siteKeys() ([]string, []string) {
	var httpKeys, httpsKeys []string
	for _, l := range v.Listens {
		_, port := splitHostPort(l.Address)
		if port == "" {
			port = l.Address
		}
		keys, scheme := &httpKeys, "http"
		if slices.Contains(l.Args, "ssl") {
			keys, scheme = &httpsKeys, "https"
		}
		for _, host := range v.Domains {
			*keys = append(*keys, scheme+"://"+host+":"+port)
		}
		if len(v.Domains) == 0 || v.isDefault {
			*keys = append(*keys, scheme+"://:"+port)
		}
	}
	return slices.Compact(httpKeys), slices.Compact(httpsKeys)
}

func (v *baseVhost) bindHosts() []string {
	var hosts []string
	for _, l := range v.Listens {
		// bind 要的是裸 IP，不能带方括号
		if host, _ := splitHostPort(l.Address); host != "" {
			if host = tools.UnwrapIPv6(host); !slices.Contains(hosts, host) {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts
}

func (v *baseVhost) buildTLS(site *conf.Directive) {
	if v.TLS == nil {
		return
	}
	protocols := protocolsToCaddy(v.TLS.Protocols)
	if len(protocols) == 0 {
		site.Add("tls", v.TLS.Cert, v.TLS.Key)
		return
	}
	site.AddBlock("tls", v.TLS.Cert, v.TLS.Key).Add("protocols", protocols...)
}

func (v *baseVhost) siteSnippet() string {
	return "ace_site_" + v.safeName
}

func (v *baseVhost) authMatcher(i int) string {
	return fmt.Sprintf("@ace_auth_%d", i)
}

func caddyUserFile(userFile string) string {
	return userFile + userFileSuffix
}

func phpSocket(version uint) string {
	return fmt.Sprintf("unix//tmp/php-cgi-%d.sock", version)
}

// splitHostPort 无法拆分时整体视为端口；IPv6 字面量保留方括号，面板的域名与监听地址都是带括号的形式
func splitHostPort(address string) (string, string) {
	if !strings.Contains(address, ":") {
		return "", address
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", address
	}
	return strings.TrimSuffix(address, ":"+port), port
}

// protocolsToCaddy Caddy 只支持 TLS 1.2 与 1.3，两者都选时省略，与默认一致
func protocolsToCaddy(protocols []string) []string {
	var has12, has13 bool
	for _, p := range protocols {
		switch caddyProtocols[p] {
		case "tls1.2":
			has12 = true
		case "tls1.3":
			has13 = true
		}
	}
	switch {
	case has12 && !has13:
		return []string{"tls1.2", "tls1.2"}
	case has13 && !has12:
		return []string{"tls1.3"}
	}
	return nil
}

func protocolsFromCaddy(args []string) []string {
	switch {
	case len(args) == 0:
		return types.DefaultSSLProtocols()
	case len(args) == 2 && args[0] == "tls1.2" && args[1] == "tls1.2":
		return []string{"TLSv1.2"}
	case args[0] == "tls1.3":
		return []string{"TLSv1.3"}
	}
	return types.DefaultSSLProtocols()
}

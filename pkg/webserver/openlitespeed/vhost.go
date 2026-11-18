package openlitespeed

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/internal/memvhost"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

var rewriteDirectivePattern = regexp.MustCompile(`(?i)^rewrite(rule|cond|base|engine|options|map)\b`)

// sslProtocol 位掩码：SSLv3=1 TLSv1.0=2 TLSv1.1=4 TLSv1.2=8 TLSv1.3=16
var protocolBitTable = []struct {
	name string
	bit  int
}{
	{"TLSv1", 2},
	{"TLSv1.1", 4},
	{"TLSv1.2", 8},
	{"TLSv1.3", 16},
}

type staticVhost struct {
	*baseVhost
}

type phpVhost struct {
	*baseVhost
}

type proxyVhost struct {
	*baseVhost
}

// vhconf 由内存状态整体生成，加载时解析回来；监听与域名属于主配置的 listener，单独记在 listen 文件
type baseVhost struct {
	memvhost.State
	safeName string
}

type fragmentType int

const (
	fragmentEmpty   fragmentType = iota // 无有效指令
	fragmentRewrite                     // 全部为 Apache 风格重写规则，需放入 rewrite 块
	fragmentConfig                      // 普通 vhost 级配置
)

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
	v.safeName = safeName(v.SiteName)

	cfg, err := parseIfExists(filepath.Join(configDir, vhostConfName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse openlitespeed config: %w", err)
	}
	if cfg != nil {
		v.load(cfg)
	}
	listen, err := parseIfExists(filepath.Join(configDir, listenConfName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse listen config: %w", err)
	}
	if listen != nil {
		v.loadListen(listen)
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

// SetEnable 停用片段只是标记，停用规则在生成 vhconf 时按标记写入
func (v *baseVhost) SetEnable(enable bool) error {
	return v.SetEnableWith(enable, "")
}

// Default 监听器按域名映射站点，通配域名承接未匹配的请求
func (v *baseVhost) Default() bool {
	return slices.Contains(v.Domains, "*")
}

func (v *baseVhost) SetDefault(enable bool) error {
	v.Domains = slices.DeleteFunc(v.Domains, func(domain string) bool { return domain == "*" })
	if enable {
		v.Domains = append(v.Domains, "*")
	}
	return nil
}

func (v *baseVhost) Save() error {
	if err := os.WriteFile(filepath.Join(v.ConfigDir, vhostConfName), []byte(Export(v.build())), 0600); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(v.ConfigDir, listenConfName), []byte(Export(v.buildListen())), 0600); err != nil {
		return fmt.Errorf("failed to save listen file: %w", err)
	}

	return syncPanelConf()
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
	if root := cfg.Value("docRoot"); root != "" {
		v.DocRoot = root
	}
	if idx := cfg.GetBlock("index"); idx != nil {
		v.IndexFiles = splitList(idx.Value("indexFiles"))
	}
	v.AccessLogFile = ""
	if b := cfg.GetBlock("accesslog"); b != nil {
		v.AccessLogFile = b.Arg(0)
	}
	v.ErrorLogFile = ""
	if b := cfg.GetBlock("errorlog"); b != nil {
		v.ErrorLogFile = b.Arg(0)
	}
	for _, d := range cfg.GetAll("include") {
		if !strings.HasPrefix(d.Arg(0), v.ConfigDir+"/") {
			v.Includes = append(v.Includes, d.Arg(0))
		}
	}
	v.loadSSL(cfg)
	v.loadPHP(cfg)
	v.loadUpstreams(cfg)
	v.loadProxies(cfg)
	v.loadAuths(cfg)
	v.loadRedirects(cfg)
}

func (v *baseVhost) loadListen(cfg *conf.Config) {
	v.Listens = []types.Listen{}
	for _, d := range cfg.GetAll("listen") {
		fields := strings.Fields(d.Arg(0))
		if len(fields) == 0 {
			continue
		}
		args := fields[1:]
		if args == nil {
			args = []string{}
		}
		v.Listens = append(v.Listens, types.Listen{Address: fields[0], Args: args})
	}
	v.Domains = strings.Fields(cfg.Value("domain"))
}

func (v *baseVhost) loadSSL(cfg *conf.Config) {
	b := cfg.GetBlock("vhssl")
	if b == nil {
		return
	}
	v.TLS = &types.SSLConfig{
		Cert:      b.Value("certFile"),
		Key:       b.Value("keyFile"),
		Protocols: protocolsFromBits(b.Value("sslProtocol")),
		OCSP:      b.Value("enableStapling") == "1",
	}
	for _, ctx := range cfg.Blocks("context") {
		if strings.Contains(ctx.Value("extraHeaders"), types.HSTSHeader) {
			v.TLS.HSTS = true
		}
	}
	if rw := cfg.GetBlock("rewrite"); rw != nil {
		for _, d := range rw.GetAll("RewriteRule") {
			if strings.Contains(d.Arg(0), "https://%{HTTP_HOST}") {
				v.TLS.HTTPRedirect = true
			}
		}
	}
}

func (v *baseVhost) loadPHP(cfg *conf.Config) {
	sh := cfg.GetBlock("scripthandler")
	if sh == nil {
		return
	}
	for _, d := range sh.GetAll("include") {
		if m := phpHandlerPattern.FindStringSubmatch(d.Arg(0)); m != nil {
			version, _ := strconv.Atoi(m[1])
			v.PHPVersion = uint(version)
		}
	}
}

// loadAuths 一条认证规则会挂到它覆盖的每个上下文上，按 realm 去重后照 realm 的顺序还原
func (v *baseVhost) loadAuths(cfg *conf.Config) {
	paths := make(map[string]string)
	for _, ctx := range cfg.Blocks("context") {
		if path := ctx.Meta("auth"); path != "" {
			paths[ctx.Value("realm")] = path
		}
	}
	for _, realm := range cfg.Blocks("realm") {
		path, ok := paths[realm.Arg(0)]
		if !ok {
			continue
		}
		var userFile string
		if db := realm.GetBlock("userDB"); db != nil {
			userFile = db.Value("location")
		}
		v.Auths = append(v.Auths, types.BasicAuth{Path: path, UserFile: userFile})
	}
}

// build 由内存状态生成完整 vhconf
func (v *baseVhost) build() *conf.Config {
	cfg := &conf.Config{}
	cfg.Append(conf.Cmt("Auto-generated by AcePanel. DO NOT EDIT MANUALLY!"))
	cfg.Add("docRoot", v.DocRoot)

	idx := cfg.AddBlock("index", "")
	idx.Add("useServer", "0")
	if len(v.IndexFiles) > 0 {
		idx.Add("indexFiles", strings.Join(v.IndexFiles, ", "))
	}

	if v.ErrorLogFile != "" {
		b := cfg.AddBlock("errorlog", v.ErrorLogFile)
		b.Add("useServer", "0")
		b.Add("logLevel", "WARN")
		b.Add("rollingSize", "10M")
	}
	if v.AccessLogFile != "" {
		b := cfg.AddBlock("accesslog", v.AccessLogFile)
		b.Add("useServer", "0")
		b.Add("logFormat", accessLogFormat)
		b.Add("rollingSize", "10M")
		b.Add("keepDays", "30")
	}

	v.buildPHP(cfg)
	v.buildSSL(cfg)
	v.buildRealms(cfg)
	v.buildUpstreams(cfg)
	v.buildProxies(cfg)
	v.buildRootContext(cfg)
	v.buildAuthContexts(cfg)

	if !v.Enable() {
		stop := cfg.AddBlock("context", stopURI)
		stop.Add("location", htmlDir+"/")
		stop.Add("allowBrowse", "1")
	}

	rewriteFragments := v.buildIncludes(cfg)
	// 同一状态码的 errorpage 后写的生效，404 重定向要排在默认 404 页面片段之后
	v.buildRedirects(cfg)
	v.buildRewrite(cfg, rewriteFragments)

	return cfg
}

// buildPHP 只 include 服务器级处理器行，外部应用由 syncPHP 维护
func (v *baseVhost) buildPHP(cfg *conf.Config) {
	if v.PHPVersion == 0 {
		return
	}
	cfg.AddBlock("scripthandler", "").Add("include", phpHandlerFile(v.PHPVersion))
}

func (v *baseVhost) buildSSL(cfg *conf.Config) {
	if v.TLS == nil {
		return
	}
	quic := slices.ContainsFunc(v.Listens, func(l types.Listen) bool {
		return slices.Contains(l.Args, "quic")
	})
	b := cfg.AddBlock("vhssl", "")
	b.Add("keyFile", v.TLS.Key)
	b.Add("certFile", v.TLS.Cert)
	b.Add("certChain", "1")
	b.Add("sslProtocol", strconv.Itoa(protocolBits(v.TLS.Protocols)))
	b.Add("sslSessionCache", "1")
	b.Add("enableQuic", map[bool]string{true: "1", false: "0"}[quic])
	if v.TLS.OCSP {
		b.Add("enableStapling", "1")
	}
}

func (v *baseVhost) buildRealms(cfg *conf.Config) {
	for i, auth := range v.Auths {
		db := cfg.AddBlock("realm", v.realmName(i)).AddBlock("userDB", "")
		db.Add("location", auth.UserFile)
		db.Add("maxCacheSize", "200")
		db.Add("cacheTimeout", "60")
	}
}

// buildRootContext 根路径已被反向代理占用时不生成
func (v *baseVhost) buildRootContext(cfg *conf.Config) {
	if cfg.GetBlock("context", "/") != nil {
		return
	}
	ctx := cfg.AddBlock("context", "/")
	ctx.Add("location", "$DOC_ROOT/")
	ctx.Add("allowBrowse", "1")
	rw := ctx.AddBlock("rewrite", "")
	rw.Add("enable", "1")
	v.protect(ctx, "/")
	setHeaders(ctx, v.contextHeaders())
}

// buildAuthContexts 为还没有同路径上下文的认证路径生成上下文。
// 落在代理规则里面的要照搬那条规则，生成静态上下文的话该路径就不再走代理而是去磁盘找文件
func (v *baseVhost) buildAuthContexts(cfg *conf.Config) {
	for _, auth := range v.Auths {
		path := authPath(auth)
		uri := strings.TrimSuffix(path, "/") + "/"
		if cfg.GetBlock("context", uri) != nil {
			continue
		}
		if p, f, ok := v.forwardFor(path); ok {
			custom, _ := customNodes(p)
			v.proxyContext(cfg, uri, path, p, f).AppendCustom(custom)
			continue
		}
		ctx := cfg.AddBlock("context", uri)
		ctx.Add("location", "$DOC_ROOT"+uri)
		ctx.Add("allowBrowse", "1")
		v.protect(ctx, path)
		setHeaders(ctx, v.contextHeaders())
	}
}

// buildIncludes 返回需放进 rewrite 块的重写片段
func (v *baseVhost) buildIncludes(cfg *conf.Config) []string {
	for _, path := range v.Includes {
		cfg.Add("include", path)
	}

	var rewrites []string
	for _, scope := range []types.ConfigScope{types.ScopeShared, types.ScopeSite} {
		dir := filepath.Join(v.ConfigDir, string(scope))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			switch fragmentKind(string(content)) {
			case fragmentRewrite:
				rewrites = append(rewrites, path)
			case fragmentConfig:
				cfg.Add("include", path)
			case fragmentEmpty: // 没有有效指令，不引用
			}
		}
	}

	return rewrites
}

func (v *baseVhost) buildRewrite(cfg *conf.Config, fragments []string) {
	rw := cfg.AddBlock("rewrite", "")
	rw.Add("enable", "1")
	rw.Add("autoLoadHtaccess", "1")

	// OLS 核心对 /.well-known/acme-challenge/ 强制跳过 rewrite，无需排除验证路径
	if !v.Enable() {
		rw.Add("RewriteCond", "%{REQUEST_URI} !^"+stopURI)
		rw.Add("RewriteRule", "^ "+stopURI+"stop.html [L]")
	}
	if v.TLS != nil && v.TLS.HTTPRedirect {
		rw.Add("RewriteCond", "%{HTTPS} !on")
		rw.Add("RewriteRule", "^(.*)$ https://%{HTTP_HOST}%{REQUEST_URI} [R=308,L]")
	}
	v.addRedirectRules(rw)
	// vhost 级 rewrite 先于上下文匹配，伪静态与 SPA 回退会把子路径反代的请求改写去根上下文找文件，
	// 命中代理规则的请求在此跳过后续规则；根规则不拦，整站反代沿用重写片段的既有行为
	for _, p := range v.ProxyRules {
		if rule := proxyPassthrough(p.Location); rule != "" {
			rw.Add("RewriteRule", rule)
		}
	}
	for _, path := range fragments {
		rw.Add("include", path)
	}
}

func (v *baseVhost) buildListen() *conf.Config {
	cfg := &conf.Config{}
	cfg.Append(conf.Cmt("Auto-generated by AcePanel. DO NOT EDIT MANUALLY!"))
	for _, l := range v.Listens {
		cfg.Add("listen", strings.TrimSpace(l.Address+" "+strings.Join(l.Args, " ")))
	}
	cfg.Add("domain", strings.Join(v.Domains, " "))
	return cfg
}

// contextHeaders OLS 只在上下文级处理 extraHeaders，前缀上下文会继承上级的而正则上下文不会，每个上下文都写一遍
func (v *baseVhost) contextHeaders() []string {
	if v.TLS != nil && v.TLS.HSTS {
		return []string{"Header set " + types.HSTSHeader + " " + types.HSTSValue}
	}
	return nil
}

// protect OLS 的认证挂在上下文上，按最长前缀只命中一个上下文，所以每个上下文都要带上覆盖它的那条认证规则
func (v *baseVhost) protect(ctx *conf.Directive, path string) {
	best := -1
	for i, auth := range v.Auths {
		if covers(authPath(auth), path) && (best < 0 || len(authPath(auth)) > len(authPath(v.Auths[best]))) {
			best = i
		}
	}
	if best >= 0 {
		ctx.Add("realm", v.realmName(best))
		ctx.AddMeta("auth", v.Auths[best].Path)
	}
}

func (v *baseVhost) realmName(i int) string {
	return fmt.Sprintf("%s_auth_%d", v.safeName, i)
}

func setHeaders(ctx *conf.Directive, lines []string) {
	if len(lines) > 0 {
		ctx.Append(&conf.Directive{Name: "extraHeaders", Args: []conf.Arg{{Value: strings.Join(lines, "\n"), Quote: conf.QuoteHeredoc}}})
	}
}

func authPath(auth types.BasicAuth) string {
	return "/" + strings.Trim(auth.Path, "/")
}

func fragmentKind(content string) fragmentType {
	kind := fragmentEmpty
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !rewriteDirectivePattern.MatchString(line) {
			return fragmentConfig
		}
		kind = fragmentRewrite
	}
	return kind
}

// safeName 将站点名转换为 OpenLiteSpeed 标识符安全形式
func safeName(name string) string {
	return strings.Map(func(char rune) rune {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' {
			return char
		}
		return '_'
	}, name)
}

// parseIfExists 解析存在的配置文件，不存在返回 nil
func parseIfExists(path string) (*conf.Config, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil //nolint:nilerr
	}
	return ParseFile(path)
}

func splitList(value string) []string {
	out := make([]string, 0)
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func protocolBits(protocols []string) int {
	var bits int
	for _, p := range protocols {
		for _, item := range protocolBitTable {
			if strings.EqualFold(p, item.name) || (item.name == "TLSv1" && strings.EqualFold(p, "TLSv1.0")) {
				bits |= item.bit
			}
		}
	}
	if bits == 0 {
		bits = 24
	}
	return bits
}

func protocolsFromBits(value string) []string {
	bits, _ := strconv.Atoi(value)
	if bits == 0 {
		bits = 24
	}
	var out []string
	for _, item := range protocolBitTable {
		if bits&item.bit != 0 {
			out = append(out, item.name)
		}
	}
	return out
}

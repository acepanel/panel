package apache

import (
	"fmt"
	"hash/crc32"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 全部代理规则写进一个片段文件，按 nginx 的 location 优先级排列。
// ProxyPassMatch 按书写顺序首个命中生效，排好序后匹配结果就与 nginx 一致，不依赖用户添加的先后。
// 每条规则自己的设置放在 <If>/<ElseIf> 链上的一节里，链与 ProxyPassMatch 用同一组正则、同一个顺序，
// 一个请求只会落进命中它的那条规则的那一节，规则之间互不泄漏

const (
	// proxyIndexMeta 规则在用户列表里的序号，文件里按优先级排列，回读靠它还原顺序
	proxyIndexMeta = "proxy"
	acmeAhead      = `/\.well-known/acme-challenge/`
	acmePattern    = "^" + acmeAhead
)

var proxyFilePattern = regexp.MustCompile(`^(\d{3})-proxy\.conf$`)

var bodyLimitPattern = regexp.MustCompile(`^%\{HTTP:Content-Length\} -gt (\d+)$`)

// builtinHeaders mod_proxy 自带 X-Forwarded-For/Host/Server，这两个要自己补；用户给了同名的头就用用户的值
var builtinHeaders = []struct{ name, value string }{
	{"X-Real-IP", "$remote_addr"},
	{"X-Forwarded-Proto", "$scheme"},
}

var ttlUnits = map[byte]int{'s': 1, 'm': 60, 'h': 3600, 'd': 86400, 'w': 604800, 'M': 2592000, 'y': 31536000}

// route 一条规则在 Apache 下的匹配与目标
type route struct {
	pattern string // 匹配请求路径的正则，ProxyPassMatch 与 <If> 共用
	ahead   string // 同一个匹配写成从路径开头起的前瞻断言体，用来拼只在规则胜出时命中的正则
	target  string // ProxyPassMatch 的目标
	worker  string // 目标带路径时去掉反向引用的那一截，要另外定义成 worker
	origin  string // 后端地址，到端口为止
	front   string // ProxyPassReverse 的前端路径
	back    string // ProxyPassReverse 的后端地址
}

func parseProxyFiles(siteDir string) []types.Proxy {
	type indexed struct {
		index int
		proxy types.Proxy
	}
	var found []indexed
	for _, file := range conf.ListFiles(siteDir, proxyFilePattern, proxyStartNum, proxyEndNum) {
		cfg, err := ParseFile(file)
		if err != nil {
			continue
		}
		for _, blk := range cfg.Blocks("IfModule") {
			index, parse := len(found), parseLegacyProxy
			if meta := blk.Meta(proxyIndexMeta); meta != "" {
				index, _ = strconv.Atoi(meta)
				parse = parseProxy
			} else if blk.Meta("location") == "" && !blk.Has("ProxyPass") {
				continue
			}
			found = append(found, indexed{index: index, proxy: parse(blk)})
		}
	}
	slices.SortStableFunc(found, func(a, b indexed) int { return a.index - b.index })

	proxies := make([]types.Proxy, 0, len(found))
	for _, item := range found {
		proxies = append(proxies, item.proxy)
	}
	return proxies
}

// newProxy 配置里没写的项按 mod_proxy 的实际行为回填：不保留也不改写 Host 时发上游主机名，对上游固定用 HTTP/1.1
func newProxy(blk *conf.Directive) types.Proxy {
	return types.Proxy{
		Location:    blk.Meta("location"),
		Pass:        blk.Meta("pass"),
		Host:        "$proxy_host",
		HTTPVersion: "1.1",
		Resolver:    []string{},
		Headers:     make(map[string]string),
		Replaces:    make(map[string]string),
	}
}

func parseProxy(blk *conf.Directive) types.Proxy {
	proxy := newProxy(blk)
	pass := blk.Get("ProxyPassMatch")
	proxy.Timeout = parseTimeout(pass)
	proxy.Buffering = !slices.Contains(pass.ArgsFrom(2), "flushpackets=on")
	for _, backend := range blk.Blocks("Proxy") {
		if ssl := parseSSLBackend(backend); ssl != nil {
			proxy.SSLBackend = ssl
		}
	}
	proxy.Cache = parseCache(blk.GetBlock("LocationMatch"))
	if section := blk.GetBlock("ElseIf"); section != nil {
		parseSection(&proxy, section)
	}
	return proxy
}

// parseLegacyProxy 旧版一条规则一个文件：Host、请求头与后端 TLS 写在 vhost 级，按路径生效的几项在 <Location> 里，
// 重新保存后换成新写法
func parseLegacyProxy(blk *conf.Directive) types.Proxy {
	proxy := newProxy(blk)
	pass := blk.Get("ProxyPass")
	if pass == nil {
		pass = blk.Get("ProxyPassMatch")
	}
	if proxy.Location == "" {
		proxy.Location, proxy.Pass = pass.Arg(0), pass.Arg(1)
	}
	proxy.Timeout = parseTimeout(pass)
	proxy.Buffering = blk.Has("ProxyIOBufferSize")
	proxy.SSLBackend = parseSSLBackend(blk)
	proxy.ClientMaxBodySize, _ = strconv.ParseInt(blk.Value("LimitRequestBody"), 10, 64)

	for _, d := range blk.GetAll("RequestHeader") {
		if d.Arg(1) == "Host" {
			proxy.Host = d.Arg(2)
		}
	}
	if strings.EqualFold(blk.Value("ProxyPreserveHost"), "on") {
		proxy.Host = "$host"
	}
	for _, sub := range blk.Blocks("IfModule") {
		parseRequestHeaders(&proxy, sub)
		parseSubstitutes(&proxy, sub)
	}
	section := blk.GetBlock("Location")
	if section == nil {
		section = blk.GetBlock("LocationMatch")
	}
	if section != nil {
		parseSection(&proxy, section)
	}
	return proxy
}

// parseSection 回读一条规则自己那一节里的设置
func parseSection(proxy *types.Proxy, section *conf.Directive) {
	section, custom := section.SplitCustom()
	proxy.Custom = strings.TrimSpace(Export(&conf.Config{Nodes: custom}))
	proxy.AccessControl = parseAccessControl(section)
	proxy.ResponseHeaders = parseResponseHeaders(section)
	if strings.EqualFold(section.Value("ProxyPreserveHost"), "on") {
		proxy.Host = "$host"
	}
	parseRequestHeaders(proxy, section)
	if sub := section.GetBlock("IfModule", "mod_substitute.c"); sub != nil {
		parseSubstitutes(proxy, sub)
	}
	if limit := section.GetBlock("If"); limit != nil {
		if m := bodyLimitPattern.FindStringSubmatch(limit.Arg(0)); m != nil {
			proxy.ClientMaxBodySize, _ = strconv.ParseInt(m[1], 10, 64)
		}
	}
	if d := section.Get("LimitRequestBody"); d != nil {
		proxy.ClientMaxBodySize, _ = strconv.ParseInt(d.Arg(0), 10, 64)
	}
}

// parseRequestHeaders 与内置值相同的内置头不算用户的自定义头
func parseRequestHeaders(proxy *types.Proxy, blk *conf.Directive) {
	for _, d := range blk.GetAll("RequestHeader") {
		vals := d.Values()
		if len(vals) < 3 || !strings.EqualFold(vals[0], "set") {
			continue
		}
		name, value := vals[1], types.ParseApacheHeaderValue(vals[2])
		builtin := slices.ContainsFunc(builtinHeaders, func(h struct{ name, value string }) bool {
			return h.name == name && h.value == value
		})
		switch {
		case name == "Host":
			proxy.Host = value
		case !builtin:
			proxy.Headers[name] = value
		}
	}
}

func parseSubstitutes(proxy *types.Proxy, blk *conf.Directive) {
	for _, d := range blk.GetAll("Substitute") {
		if from, to, ok := parseSubstitute(d.Arg(0)); ok {
			proxy.Replaces[from] = to
		}
	}
}

// parseSubstitute 解析 Substitute 规则 s|from|to|flags，分隔符取规则第二个字符
func parseSubstitute(rule string) (from, to string, ok bool) {
	if len(rule) < 2 || rule[0] != 's' {
		return "", "", false
	}
	parts := strings.Split(rule[2:], string(rule[1]))
	if len(parts) >= 2 {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func parseTimeout(pass *conf.Directive) *types.TimeoutConfig {
	timeout := &types.TimeoutConfig{}
	for _, a := range pass.ArgsFrom(2) {
		key, value, _ := strings.Cut(a, "=")
		seconds, _ := strconv.Atoi(value)
		switch key {
		case "connectiontimeout":
			timeout.Connect = time.Duration(seconds) * time.Second
		case "timeout":
			timeout.Read = time.Duration(seconds) * time.Second
		}
	}
	if *timeout == (types.TimeoutConfig{}) {
		return nil
	}
	return timeout
}

func parseSSLBackend(blk *conf.Directive) *types.SSLBackendConfig {
	if blk == nil || !strings.EqualFold(blk.Value("SSLProxyVerify"), "require") {
		return nil
	}
	depth, _ := strconv.Atoi(blk.Value("SSLProxyVerifyDepth"))
	return &types.SSLBackendConfig{
		Verify:             true,
		TrustedCertificate: blk.Value("SSLProxyCACertificateFile"),
		VerifyDepth:        depth,
	}
}

func parseCache(loc *conf.Directive) *types.CacheConfig {
	// cacheCodes 后端没给有效期时 mod_cache 只缓存这两种状态的响应（另有少见的 203、300），回读时有效期记在它们名下
	const cacheCodes = "200 301"

	if loc == nil || !loc.Has("CacheEnable") {
		return nil
	}
	cache := &types.CacheConfig{Valid: map[string]string{}, NoCacheConditions: []string{}, UseStale: []string{}, Methods: []string{}}
	if seconds, err := strconv.Atoi(loc.Value("CacheDefaultExpire")); err == nil {
		cache.Valid[cacheCodes] = formatTTL(seconds)
	}
	return cache
}

func parseAccessControl(blk *conf.Directive) *types.AccessControlConfig {
	ac := &types.AccessControlConfig{Allow: []string{}, Deny: []string{}}
	collect := func(b *conf.Directive) {
		for _, d := range b.GetAll("Require") {
			switch {
			case d.Arg(0) == "not" && d.Arg(1) == "ip":
				ac.Deny = append(ac.Deny, d.ArgsFrom(2)...)
			case d.Arg(0) == "ip":
				ac.Allow = append(ac.Allow, d.ArgsFrom(1)...)
			case d.Arg(0) == "all" && d.Arg(1) == "denied":
				ac.Deny = append(ac.Deny, "all")
			}
		}
	}
	collect(blk)
	for _, all := range blk.Blocks("RequireAll") {
		collect(all)
	}
	if len(ac.Allow) == 0 && len(ac.Deny) == 0 {
		return nil
	}
	return ac
}

func parseResponseHeaders(blk *conf.Directive) *types.ResponseHeaderConfig {
	headers := &types.ResponseHeaderConfig{Hide: []string{}, Add: make(map[string]string)}
	for _, d := range blk.GetAll("Header") {
		args := d.Values()
		if len(args) > 0 && strings.EqualFold(args[0], "always") {
			args = args[1:]
		}
		switch {
		case len(args) >= 2 && strings.EqualFold(args[0], "unset"):
			headers.Hide = append(headers.Hide, args[1])
		case len(args) >= 3 && strings.EqualFold(args[0], "set"):
			headers.Add[args[1]] = types.ParseApacheHeaderValue(args[2])
		}
	}
	if len(headers.Hide) == 0 && len(headers.Add) == 0 {
		return nil
	}
	return headers
}

func writeProxyFiles(siteDir string, proxies []types.Proxy, upstreams []string) error {
	if len(proxies) > proxyEndNum-proxyStartNum+1 {
		return fmt.Errorf("proxy rules exceed limit (%d)", proxyEndNum-proxyStartNum+1)
	}
	// 先全部生成，自定义指令或正则有问题时不动已有文件
	cfg := &conf.Config{}
	cfg.Append(conf.Cmt("Auto-generated by AcePanel. DO NOT EDIT MANUALLY!"))
	cfg.Append(acmeNodes()...)
	// 只在本条规则胜出时命中的正则：排在前面的规则都没命中、自己命中。每写完一条就把它并进排除项
	won := "^(?!" + acmeAhead + ")"
	for _, i := range types.ProxyOrder(proxies) {
		r := newRoute(proxies[i].Location, balancerPass(proxies[i].Pass, upstreams))
		nodes, err := proxyNodes(proxies[i], i, r, won+"(?="+r.ahead+")")
		if err != nil {
			return err
		}
		cfg.Append(nodes...)
		won += "(?!" + r.ahead + ")"
	}
	if err := conf.ClearFiles(siteDir, proxyFilePattern, proxyStartNum, proxyEndNum); err != nil {
		return err
	}
	if len(proxies) == 0 {
		return nil
	}

	path := filepath.Join(siteDir, fmt.Sprintf("%03d-proxy.conf", proxyStartNum))
	if err := os.WriteFile(path, []byte(Export(cfg)+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to write proxy config: %w", err)
	}
	return nil
}

// acmeNodes 证书验证路径不进反代，也不套任何一条规则的设置，否则整站白名单之类会把验证请求挡掉。
// 这一节同时是 <If> 链的起点，后面每条规则都接 <ElseIf>
func acmeNodes() []conf.Node {
	expr, _ := matchExpr(acmePattern)
	return []conf.Node{conf.Blk("IfModule", "mod_proxy.c").Append(
		conf.Dir("ProxyPassMatch", acmePattern, "!"),
		conf.Blk("If", expr),
	)}
}

// matchExpr <If> 的条件。表达式里的正则不认转义的定界符，只能挑一个正则里没出现过的
func matchExpr(pattern string) (string, error) {
	// requestURI 请求路径，与 ProxyPassMatch 匹配的是同一个值
	const requestURI = "%{REQUEST_URI}"

	for _, del := range "#,;:!%_-|" {
		if !strings.ContainsRune(pattern, del) {
			return requestURI + " =~ m" + string(del) + pattern + string(del), nil
		}
	}
	return "", fmt.Errorf("unsupported location pattern: %s", pattern)
}

// newRoute nginx 的 proxy_pass 不带路径时原样转发 URI，带路径时把匹配到的部分换成该路径。
// ProxyPassMatch 的目标里没有反向引用时会把整个 URI 接在后面，正好是前一种；后一种用捕获组接上余下的部分，
// 精确匹配没有余下的部分，也要留一个空捕获组，否则 URI 又会被接上去。
// 正则 location 在 nginx 里不允许 proxy_pass 带路径，这里同样只取到端口
func newRoute(location, pass string) route {
	socket, origin, path := splitPass(pass)
	kind, rest := types.ParseLocation(location)
	r := route{target: socket + origin, origin: origin, front: "/", back: origin + "/"}
	if kind == types.LocationRegex || kind == types.LocationRegexAny {
		r.pattern = rest
		if kind == types.LocationRegexAny {
			// PCRE 的内联标志，ProxyPassMatch 本身区分大小写
			r.pattern = "(?i)" + rest
		}
		// 用户的正则不一定从开头匹配，顶层还可能有 |，整个包起来放到任意位置去找
		r.ahead = ".*(?:" + r.pattern + ")"
		return r
	}

	prefix := types.NormalizePath(rest)
	quoted, end, tail := regexp.QuoteMeta(prefix), "", "(.*)$"
	if kind == types.LocationExact {
		end, tail = "$", "()$"
	}
	r.pattern, r.ahead = "^"+quoted+end, quoted+end
	if path != "" {
		r.worker = r.target + path
		r.pattern, r.target, r.front, r.back = "^"+quoted+tail, r.worker+"$1", prefix, origin+path
	}
	return r
}

// splitPass 拆出套接字前缀、后端地址与路径。nginx 的 unix 套接字写法是 http://unix:/套接字[:/路径]，
// Apache 要写成 unix:/套接字|http://主机名，其中主机名只用来给 worker 起名：
// 同名的 worker 会共用先定义的那个套接字，所以按套接字路径各编一个
func splitPass(pass string) (socket, origin, path string) {
	if scheme, rest, ok := strings.Cut(pass, "://unix:"); ok {
		sock, path, _ := strings.Cut(rest, ":")
		return "unix:" + sock + "|", scheme + "://" + socketHost(sock), path
	}
	u, err := url.Parse(pass)
	if err != nil || u.Host == "" {
		return "", pass, ""
	}
	return "", u.Scheme + "://" + u.Host, u.Path
}

func socketHost(sock string) string {
	return fmt.Sprintf("uds-%08x", crc32.ChecksumIEEE([]byte(sock)))
}

// balancerPass 上游名换成 Apache 的 balancer 地址，照搬 nginx 的 http://<上游名> 会被当真实主机解析
func balancerPass(pass string, upstreams []string) string {
	u, err := url.Parse(pass)
	if err != nil || !slices.Contains(upstreams, u.Hostname()) {
		return pass
	}
	return "balancer://" + u.Hostname() + u.Path
}

// proxyNodes won 是只在这条规则胜出时命中的正则
func proxyNodes(proxy types.Proxy, index int, r route, won string) ([]conf.Node, error) {
	// 片段要整体落在这条规则的那一节里，严格解析，孤立或未闭合的标签会破坏外层结构
	custom, err := parse(proxy.Custom, parseOptions{})
	if err != nil {
		return nil, fmt.Errorf("invalid custom directives of proxy %s: %w", proxy.Location, err)
	}
	expr, err := matchExpr(r.pattern)
	if err != nil {
		return nil, err
	}

	blk := conf.Blk("IfModule", "mod_proxy.c")
	blk.AddMeta(proxyIndexMeta, strconv.Itoa(index))
	// 匹配类型与目标路径在 Apache 语法里不可逆，留下原值供回读
	blk.AddMeta("location", proxy.Location)
	blk.AddMeta("pass", proxy.Pass)

	var params []string
	if !strings.HasPrefix(r.target, "balancer://") {
		params = workerParams(proxy)
	}
	blk.Append(conf.Dir("ProxyPassMatch", append([]string{r.pattern, r.target}, params...)...), conf.Dir("ProxyPassReverse", r.front, r.back))
	// 目标里带 $1 的 worker，httpd 是拿它的名字与实际地址比长度来找的，余下部分不足两个字符就找不到：
	// 连接参数不生效，套接字后端更是直接连不上。给同一截地址再定义一个 worker 兜住
	if r.worker != "" && params != nil {
		blk.Append(conf.Blk("Proxy", r.worker).Append(conf.Dir("ProxySet", params...)))
	}
	if ssl := sslProxyNode(proxy, r.origin); ssl != nil {
		blk.Append(ssl)
	}
	if proxy.Cache != nil {
		cache, err := cacheNode(proxy, won)
		if err != nil {
			return nil, err
		}
		blk.Append(cache)
	}
	section := conf.Blk("ElseIf", expr).Append(sectionNodes(proxy)...)
	section.AppendCustom(custom.Nodes)
	blk.Append(section)

	return []conf.Node{conf.Cmt(fmt.Sprintf("Reverse proxy: %s -> %s", proxy.Location, proxy.Pass)), blk}, nil
}

// workerParams 连接参数只能写在直连后端上，balancer 目标的要写在 BalancerMember 上
func workerParams(p types.Proxy) []string {
	var params []string
	if p.Timeout != nil {
		if p.Timeout.Connect > 0 {
			params = append(params, "connectiontimeout="+strconv.Itoa(int(p.Timeout.Connect.Seconds())))
		}
		if p.Timeout.Read > 0 {
			params = append(params, "timeout="+strconv.Itoa(int(p.Timeout.Read.Seconds())))
		}
	}
	// 不缓冲即每收到一块就发给客户端
	if !p.Buffering {
		params = append(params, "flushpackets=on")
	}
	// 2.4.47 起协议升级由 mod_proxy_http 接管，但要显式开启，否则 WebSocket 被当普通请求
	return append(params, "upgrade=websocket")
}

// sslProxyNode 后端 TLS 的指令不能写在 <If> 里，按后端地址圈定在 <Proxy> 里，不同后端各用各的校验设置
func sslProxyNode(proxy types.Proxy, origin string) *conf.Directive {
	if !strings.HasPrefix(origin, "https://") {
		return nil
	}

	blk := conf.Blk("Proxy", origin+"/").Append(conf.Dir("SSLProxyEngine", "On"))
	if proxy.SSLBackend == nil || !proxy.SSLBackend.Verify {
		return blk.Append(
			conf.Dir("SSLProxyVerify", "none"),
			conf.Dir("SSLProxyCheckPeerCN", "off"),
			conf.Dir("SSLProxyCheckPeerName", "off"),
		)
	}
	blk.Append(conf.Dir("SSLProxyVerify", "require"))
	if proxy.SSLBackend.VerifyDepth > 0 {
		blk.Append(conf.Dir("SSLProxyVerifyDepth", strconv.Itoa(proxy.SSLBackend.VerifyDepth)))
	}
	if proxy.SSLBackend.TrustedCertificate != "" {
		blk.Append(conf.Dir("SSLProxyCACertificateFile", proxy.SSLBackend.TrustedCertificate))
	}
	return blk
}

// cacheNode 缓存开关不能写在 <If> 里，放进只在这条规则胜出时命中的 <LocationMatch>；按路径启用依赖全局的
// CacheQuickHandler off，与 CacheRoot 一起由安装脚本写入。mod_cache 按 HTTP 语义缓存：后端自己给了有效期就以它为准，
// 带查询串的地址也只在这种情况下才缓存，其余可缓存的响应用规则里的有效期
func cacheNode(p types.Proxy, won string) (*conf.Directive, error) {
	loc := conf.Blk("LocationMatch", won).Append(
		conf.Dir("CacheEnable", "disk"),
		// 没有 Last-Modified 与 ETag 的响应默认不缓存
		conf.Dir("CacheIgnoreNoLastMod", "On"),
		// 默认会在后端出错时拿过期的缓存顶上
		conf.Dir("CacheStaleOnError", "Off"),
		// 默认连 Set-Cookie 一起缓存再发给别的访客，带它的响应标成私有就不进缓存
		conf.Dir("Header", "merge", "Cache-Control", "private", "expr=-n %{resp:Set-Cookie}"),
	)
	ttl := cacheTTL(p.Cache.Valid)
	// 没填时不能落到 httpd 默认的一小时，与 nginx 方言一样按 10 分钟
	if ttl == "" {
		ttl = "10m"
	}
	seconds, ok := parseTTL(ttl)
	if !ok {
		return nil, fmt.Errorf("invalid cache duration of proxy %s: %s", p.Location, ttl)
	}
	// 三项设成同一个值，按 Last-Modified 估出来的有效期也落在它上面
	for _, name := range []string{"CacheDefaultExpire", "CacheMinExpire", "CacheMaxExpire"} {
		loc.Append(conf.Dir(name, strconv.Itoa(seconds)))
	}
	return loc, nil
}

// cacheTTL Apache 只有一个有效期，取对 200 生效的那一项
func cacheTTL(valid map[string]string) string {
	for _, codes := range slices.Sorted(maps.Keys(valid)) {
		if fields := strings.Fields(codes); slices.Contains(fields, "200") || slices.Contains(fields, "any") {
			return valid[codes]
		}
	}
	return ""
}

func sectionNodes(p types.Proxy) []conf.Node {
	var nodes []conf.Node
	for _, h := range builtinHeaders {
		value := h.value
		if custom, ok := p.Headers[h.name]; ok {
			value = custom
		}
		nodes = append(nodes, conf.Dir("RequestHeader", "set", h.name, types.ApacheHeaderValue(value)))
	}
	host := strings.TrimSpace(p.Host)
	// 套接字后端的主机名是编出来的，nginx 在这种情况下发的是 localhost
	if (host == "" || host == "$proxy_host") && strings.Contains(p.Pass, "://unix:") {
		host = "localhost"
	}
	// mod_proxy 默认发上游主机名并无视请求里的 Host，要发别的值得先保留它，再由 mod_headers 改写
	switch host {
	case "", "$proxy_host":
	case "$host":
		nodes = append(nodes, conf.Dir("ProxyPreserveHost", "On"))
	default:
		nodes = append(nodes, conf.Dir("ProxyPreserveHost", "On"), conf.Dir("RequestHeader", "set", "Host", types.ApacheHeaderValue(host)))
	}
	for _, name := range slices.Sorted(maps.Keys(p.Headers)) {
		if !slices.ContainsFunc(builtinHeaders, func(h struct{ name, value string }) bool { return h.name == name }) {
			nodes = append(nodes, conf.Dir("RequestHeader", "set", name, types.ApacheHeaderValue(p.Headers[name])))
		}
	}
	if p.AccessControl != nil {
		nodes = append(nodes, accessControlNodes(p.AccessControl)...)
	}
	// 不带 always：后端发回来的头不在 always 那张表里，unset 删不掉；那张表管的是本机生成的重定向与错误响应，这些头也不该加上去
	if p.ResponseHeaders != nil {
		for _, name := range p.ResponseHeaders.Hide {
			nodes = append(nodes, conf.Dir("Header", "unset", name))
		}
		for _, name := range slices.Sorted(maps.Keys(p.ResponseHeaders.Add)) {
			nodes = append(nodes, conf.Dir("Header", "set", name, types.ApacheHeaderValue(p.ResponseHeaders.Add[name])))
		}
	}
	// 反代请求不受 LimitRequestBody 约束，只能按 Content-Length 拦。分块传输事先不知道大小，一律要求带长度，否则换成分块就绕过了上限；
	// HTTP/2 没带长度的请求体到这里也是分块
	if p.ClientMaxBodySize > 0 {
		nodes = append(nodes,
			conf.Blk("If", "%{HTTP:Content-Length} -gt "+strconv.FormatInt(p.ClientMaxBodySize, 10)).Append(conf.Dir("Redirect", "413")),
			conf.Blk("ElseIf", "-n %{HTTP:Transfer-Encoding}").Append(conf.Dir("Redirect", "411")),
		)
	}
	if len(p.Replaces) > 0 {
		nodes = append(nodes, substituteNode(p.Replaces))
	}
	return nodes
}

// accessControlNodes IP 访问控制。Require ip 本身就是白名单；只有拒绝名单时要包在 RequireAll 里才是"默认放行、排除名单"的语义。
// <If> 默认会顶掉基本认证那一节的 Require，要显式改成两者都满足
func accessControlNodes(ac *types.AccessControlConfig) []conf.Node {
	allow, deny := ac.Lists()
	merge := conf.Dir("AuthMerging", "And")
	switch {
	case slices.Contains(deny, "all"):
		return []conf.Node{merge, conf.Dir("Require", "all", "denied")}
	case len(deny) == 0 && len(allow) == 0:
		return nil
	case len(deny) == 0:
		return []conf.Node{merge, conf.Dir("Require", append([]string{"ip"}, allow...)...)}
	}

	granted := conf.Dir("Require", "all", "granted")
	if len(allow) > 0 {
		granted = conf.Dir("Require", append([]string{"ip"}, allow...)...)
	}
	return []conf.Node{merge, conf.Blk("RequireAll").Append(
		granted,
		conf.Dir("Require", append([]string{"not", "ip"}, deny...)...),
	)}
}

// substituteNode 后端按客户端的 Accept-Encoding 压缩后就替换不动了，转发前去掉这个头
func substituteNode(replaces map[string]string) *conf.Directive {
	sub := conf.Blk("IfModule", "mod_substitute.c").Append(
		conf.Dir("RequestHeader", "unset", "Accept-Encoding"),
		conf.Dir("AddOutputFilterByType", "SUBSTITUTE", "text/html", "text/plain", "text/xml"),
	)
	for _, from := range slices.Sorted(maps.Keys(replaces)) {
		// 用 | 作为分隔符以支持含 / 的内容，强制双引号
		sub.Append(&conf.Directive{Name: "Substitute", Args: []conf.Arg{dquote(fmt.Sprintf("s|%s|%s|n", from, replaces[from]))}})
	}
	return sub
}

// parseTTL nginx 的时长写法转成秒，不带单位按秒
func parseTTL(s string) (int, bool) {
	s = strings.TrimSpace(s)
	unit := 1
	if s != "" && ttlUnits[s[len(s)-1]] > 0 {
		unit, s = ttlUnits[s[len(s)-1]], s[:len(s)-1]
	}
	n, err := strconv.Atoi(s)
	return n * unit, err == nil && n > 0
}

func formatTTL(seconds int) string {
	for _, suffix := range []byte("dhm") {
		if seconds%ttlUnits[suffix] == 0 {
			return strconv.Itoa(seconds/ttlUnits[suffix]) + string(suffix)
		}
	}
	return strconv.Itoa(seconds) + "s"
}

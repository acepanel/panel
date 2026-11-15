package openlitespeed

import (
	"fmt"
	"maps"
	"net"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 上游与代理均以 vhost 级外部应用加上下文的形式写入 vhconf，无法从配置推导的字段以 ace: 注释回写

// initTimeout 实测管不到等后端响应的时间，当不了读超时，固定用默认值
const initTimeout = "60"

// allAddresses 访问控制里表示全部地址的写法
const allAddresses = "ALL"

var (
	headerOpPattern = regexp.MustCompile(`^(RequestHeader|Header)\s+(set|unset)\s+(\S+)\s*(.*)$`)
	// forwardRulePattern 取上下文里转发规则的外部应用名与 Host
	forwardRulePattern = regexp.MustCompile(`^\S+\s+https?://([^/%\s]+)\S*\s+\[P,L,NE(?:,E=Proxy-Host:([^\]]+))?]$`)
)

// forward 一条代理规则把请求交给后端的方式
type forward struct {
	handler string // 外部应用名
	address string // 直连后端的地址，走上游时为空
	pass    string // 实际生效的代理目标
	host    string // 发给后端的 Host，空为透传访客的
	prefix  string // 规则匹配的路径前缀
	path    string // 目标里的路径，换掉请求里匹配到的前缀，空为原样转发
}

func (v *baseVhost) loadUpstreams(cfg *conf.Config) {
	prefix := v.safeName + "_up_"
	for _, lb := range cfg.Blocks("extprocessor") {
		if !strings.EqualFold(lb.Value("type"), "loadbalancer") {
			continue
		}
		up := types.Upstream{
			Name:     strings.TrimPrefix(lb.Arg(0), prefix),
			Servers:  make(map[string]string),
			Resolver: []string{},
		}
		for worker := range strings.SplitSeq(lb.Value("workers"), ",") {
			_, name, _ := strings.Cut(strings.TrimSpace(worker), "::")
			if member := cfg.GetBlock("extprocessor", name); member != nil {
				up.Servers[addrFromOLS(strings.TrimPrefix(member.Value("address"), "https://"))] = ""
			}
		}
		v.UpstreamList = append(v.UpstreamList, up)
	}
}

// loadProxies 带 ace:location 的上下文才是代理规则，认证路径另起的上下文不带
func (v *baseVhost) loadProxies(cfg *conf.Config) {
	for _, raw := range cfg.Blocks("context") {
		ctx, custom := raw.SplitCustom()
		location := ctx.Meta("location")
		if location == "" {
			continue
		}
		// 对上游固定用 HTTP/1.1
		p := types.Proxy{
			Location:    location,
			Pass:        ctx.Meta("pass"),
			HTTPVersion: "1.1",
			Resolver:    []string{},
			Headers:     make(map[string]string),
			Replaces:    make(map[string]string),
			Custom:      strings.TrimSpace(Export(&conf.Config{Nodes: custom})),
		}
		handler, host := ctx.Value("handler"), ""
		if rw := ctx.GetBlock("rewrite"); rw != nil {
			if m := forwardRulePattern.FindStringSubmatch(rw.Value("RewriteRule")); m != nil {
				handler, host = m[1], m[2]
			}
		}
		if ext := cfg.GetBlock("extprocessor", handler); ext != nil {
			p.Buffering = ext.Value("respBuffer") == "1"
		}

		if d := ctx.Get("extraHeaders"); d != nil {
			for line := range strings.SplitSeq(d.Arg(0), "\n") {
				m := headerOpPattern.FindStringSubmatch(strings.TrimSpace(line))
				if m == nil || m[3] == types.HSTSHeader {
					continue
				}
				switch {
				// 旧版用 RequestHeader 改 Host
				case m[1] == "RequestHeader" && m[3] == "Host":
					host = strings.TrimPrefix(m[4], "expr=")
				case m[1] == "RequestHeader":
					p.Headers[m[3]] = m[4]
				case m[2] == "set":
					if p.ResponseHeaders == nil {
						p.ResponseHeaders = &types.ResponseHeaderConfig{Add: make(map[string]string)}
					}
					p.ResponseHeaders.Add[m[3]] = m[4]
				default:
					if p.ResponseHeaders == nil {
						p.ResponseHeaders = &types.ResponseHeaderConfig{Add: make(map[string]string)}
					}
					p.ResponseHeaders.Hide = append(p.ResponseHeaders.Hide, m[3])
				}
			}
		}
		p.Host = nginxHost(host, p.Pass)

		if ac := ctx.GetBlock("accessControl"); ac != nil {
			p.AccessControl = &types.AccessControlConfig{Allow: splitList(ac.Value("allow"))}
			for _, ip := range splitList(ac.Value("deny")) {
				switch {
				case ip != allAddresses:
					p.AccessControl.Deny = append(p.AccessControl.Deny, ip)
				// 允许列表后面那条全部拒绝是写入时补的
				case len(p.AccessControl.Allow) == 0:
					p.AccessControl.Deny = append(p.AccessControl.Deny, "all")
				}
			}
		}

		v.ProxyRules = append(v.ProxyRules, p)
	}
}

func (v *baseVhost) buildUpstreams(cfg *conf.Config) {
	for _, up := range v.UpstreamList {
		lbName := v.upstreamName(up.Name)
		servers := make([]string, 0, len(up.Servers))
		for addr := range up.Servers {
			servers = append(servers, addr)
		}
		sort.Strings(servers)
		// 成员走不走 TLS 写在各自的地址里，取决于代理规则的目标是不是 https://上游名
		secure := slices.ContainsFunc(v.ProxyRules, func(p types.Proxy) bool {
			u, err := url.Parse(p.Pass)
			return err == nil && u.Scheme == "https" && u.Hostname() == up.Name
		})

		var workers []string
		for i, addr := range servers {
			name := fmt.Sprintf("%s_%d", lbName, i)
			address := olsAddress(addr)
			if secure && !strings.HasPrefix(address, "uds:") {
				address = "https://" + address
			}
			addProxyApp(cfg.AddBlock("extprocessor", name), address, false)
			workers = append(workers, "proxy::"+name)
		}

		lb := cfg.AddBlock("extprocessor", lbName)
		lb.Add("type", "loadbalancer")
		lb.Add("workers", strings.Join(workers, ", "))
	}
}

func (v *baseVhost) buildProxies(cfg *conf.Config) {
	for i, p := range v.ProxyRules {
		f := v.forwardOf(p, i)
		if f.address != "" {
			addProxyApp(cfg.AddBlock("extprocessor", f.handler), f.address, p.Buffering)
		}
		ctx := v.proxyContext(cfg, locationToURI(p.Location), rulePath(p.Location), p, f)
		ctx.AddMeta("location", p.Location)
		ctx.AddMeta("pass", f.pass)
		custom, _ := customNodes(p)
		ctx.AppendCustom(custom)
	}
}

func (v *baseVhost) forwardOf(p types.Proxy, i int) forward {
	f := forward{pass: p.Pass}
	f.handler, f.address = v.proxyHandler(p, i)
	if f.address != "" {
		f.host = proxyHost(p)
	}
	u, err := url.Parse(p.Pass)
	if err != nil || u.Path == "" {
		return f
	}
	// 负载均衡只会原样转发，正则规则没有可换掉的前缀，目标里的路径都生效不了
	kind, prefix := types.ParseLocation(p.Location)
	if f.address == "" || kind == types.LocationRegex || kind == types.LocationRegexAny {
		u.Path, u.RawPath = "", ""
		f.pass = u.String()
		return f
	}
	// 换成自己等于没换，按原样转发处理，WebSocket 才接得上
	if f.prefix, f.path = types.NormalizePath(prefix), u.EscapedPath(); f.path == f.prefix {
		f.path = ""
	}

	return f
}

// proxyHandler 解析代理目标：命中上游时返回上游名，否则返回新建外部应用名与后端地址
func (v *baseVhost) proxyHandler(p types.Proxy, i int) (string, string) {
	u, err := url.Parse(p.Pass)
	if err != nil || u.Host == "" {
		return fmt.Sprintf("%s_proxy_%d", v.safeName, i), p.Pass
	}
	for _, up := range v.UpstreamList {
		if up.Name == u.Hostname() {
			return v.upstreamName(up.Name), ""
		}
	}

	host, port := u.Hostname(), u.Port()
	address := net.JoinHostPort(host, port)
	if u.Scheme == "https" {
		if port == "" {
			address = net.JoinHostPort(host, "443")
		}
		address = "https://" + address
	} else if port == "" {
		address = net.JoinHostPort(host, "80")
	}

	return fmt.Sprintf("%s_proxy_%d", v.safeName, i), address
}

// proxyContext 在 uri 上生成代理规则的上下文，path 是它覆盖的路径。
// 落在规则里面的认证路径要另起上下文，转发、头部与访问控制都得照搬，否则那段路径就脱离了规则
func (v *baseVhost) proxyContext(cfg *conf.Config, uri, path string, p types.Proxy, f forward) *conf.Directive {
	ctx := cfg.AddBlock("context", uri)
	f.attach(ctx, uri)

	headers := v.contextHeaders()
	// 负载均衡上下文不执行 RequestHeader
	if f.address != "" {
		for _, name := range literalHeaders(p.Headers) {
			headers = append(headers, fmt.Sprintf("RequestHeader set %s %s", name, p.Headers[name]))
		}
	}
	if p.ResponseHeaders != nil {
		for _, name := range literalHeaders(p.ResponseHeaders.Add) {
			headers = append(headers, fmt.Sprintf("Header set %s %s", name, p.ResponseHeaders.Add[name]))
		}
		for _, name := range p.ResponseHeaders.Hide {
			headers = append(headers, "Header unset "+name)
		}
	}
	setHeaders(ctx, headers)

	if p.AccessControl != nil {
		allow, deny := p.AccessControl.Lists()
		// 只填允许列表时其余地址照样放行，补上全部拒绝才是白名单
		denyAll := len(allow) > 0 || slices.Contains(deny, "all")
		deny = slices.DeleteFunc(deny, func(ip string) bool { return ip == "all" })
		if denyAll {
			deny = append(deny, allAddresses)
		}
		if len(allow) > 0 || len(deny) > 0 {
			ac := ctx.AddBlock("accessControl", "")
			if len(allow) > 0 {
				ac.Add("allow", strings.Join(allow, ", "))
			}
			if len(deny) > 0 {
				ac.Add("deny", strings.Join(deny, ", "))
			}
		}
	}

	v.protect(ctx, path)
	f.websocket(cfg, uri)

	return ctx
}

// forwardFor 覆盖该路径的前缀代理规则里最长的那条及其转发方式
func (v *baseVhost) forwardFor(path string) (types.Proxy, forward, bool) {
	best, match := -1, -1
	for i, p := range v.ProxyRules {
		if kind, _ := types.ParseLocation(p.Location); kind != types.LocationPrefix {
			continue
		}
		if prefix := rulePath(p.Location); covers(prefix, path) && len(prefix) > best {
			best, match = len(prefix), i
		}
	}
	if match < 0 {
		return types.Proxy{}, forward{}, false
	}

	return v.ProxyRules[match], v.forwardOf(v.ProxyRules[match], match), true
}

func (v *baseVhost) upstreamName(name string) string {
	return fmt.Sprintf("%s_up_%s", v.safeName, safeName(name))
}

// attach 让上下文把请求交给后端。上游与不改 Host、路径的直连后端由处理器上下文原样转发；
// 要改的只能靠上下文里的重写规则：用 RequestHeader 改 Host 会多发一个 Host 头，代理上下文不改写 URI，
// 而 vhost 级的 [P] 规则会绕过上下文的认证、访问控制与头部操作
func (f forward) attach(ctx *conf.Directive, uri string) {
	switch {
	case f.address == "":
		ctx.Add("type", "loadbalancer")
		ctx.Add("handler", f.handler)
	case f.host == "" && f.path == "":
		ctx.Add("type", "proxy")
		ctx.Add("handler", f.handler)
	default:
		ctx.Add("location", "$DOC_ROOT/")
		ctx.Add("allowBrowse", "0")
		rw := ctx.AddBlock("rewrite", "")
		rw.Add("enable", "1")
		rw.Add("RewriteRule", f.rule(uri))
	}
}

// rule 目标的主机位是外部应用名。不带路径时用 ORG_REQ_URI 把请求行原样转出去，
// 带路径时同 nginx 换掉匹配到的前缀；上下文内的规则匹配的是去掉上下文 uri 之后的部分
func (f forward) rule(uri string) string {
	scheme := "http"
	if strings.HasPrefix(f.address, "https://") {
		scheme = "https"
	}
	pattern, target := "^", "%{ORG_REQ_URI}"
	if f.path != "" {
		target = f.path
		if !strings.HasPrefix(uri, "exp:") {
			var rest string
			if strings.HasPrefix(uri, f.prefix) {
				rest = uri[len(f.prefix):]
			}
			pattern, target = "^(.*)$", f.path+rest+"$1"
		}
	}
	flags := "P,L,NE"
	if f.host != "" {
		flags += ",E=Proxy-Host:" + f.host
	}

	return fmt.Sprintf("%s %s://%s%s [%s]", pattern, scheme, f.handler, target, flags)
}

// websocket 升级请求不走代理处理器，由同 uri 的 websocket 块原样接到后端；路径被改写的规则对不上，不生成
func (f forward) websocket(cfg *conf.Config, uri string) {
	if f.address != "" && f.path == "" && !strings.HasPrefix(f.address, "https://") && !strings.HasPrefix(uri, "exp:") {
		cfg.AddBlock("websocket", uri).Add("address", f.address)
	}
}

// literalHeaders extraHeaders 里的值按字面量发出，带 nginx 变量的表达不了，不写
func literalHeaders(headers map[string]string) []string {
	return slices.DeleteFunc(slices.Sorted(maps.Keys(headers)), func(name string) bool {
		return types.ApacheVars.ToNative(headers[name]) != headers[name]
	})
}

// customNodes 解析代理的自定义指令，SetProxies 时已校验过语法
func customNodes(p types.Proxy) ([]conf.Node, error) {
	cfg, err := Parse(p.Custom)
	if err != nil {
		return nil, fmt.Errorf("invalid custom directives of proxy %s: %w", p.Location, err)
	}
	return cfg.Nodes, nil
}

func addProxyApp(ext *conf.Directive, address string, buffering bool) {
	ext.Add("type", "proxy")
	ext.Add("address", address)
	ext.Add("maxConns", "100")
	ext.Add("initTimeout", initTimeout)
	ext.Add("retryTimeout", "0")
	ext.Add("persistConn", "1")
	ext.Add("pcKeepAliveTimeout", "60")
	ext.Add("respBuffer", map[bool]string{true: "1", false: "0"}[buffering])
}

// proxyHost 发给后端的 Host，空为透传访客的。OLS 原样转发整个 Host 头，$host 与 $http_host 在这里没有区别；
// 其余变量换成重写规则的写法
func proxyHost(p types.Proxy) string {
	switch host := strings.TrimSpace(p.Host); host {
	case "$host", "$http_host":
		return ""
	case "", "$proxy_host":
		return upstreamHost(p.Pass)
	default:
		return types.ApacheVars.ToNative(host)
	}
}

// nginxHost 把写进配置的 Host 还原成面板的写法，与从 pass 推导的默认值相同时回读成 $proxy_host，保证往返幂等
func nginxHost(host, pass string) string {
	switch host {
	case "":
		return "$host"
	case upstreamHost(pass):
		return "$proxy_host"
	default:
		return types.ApacheVars.ToNginx(host)
	}
}

// upstreamHost 同 nginx 的 $proxy_host：目标里写了端口就带上
func upstreamHost(pass string) string {
	u, err := url.Parse(pass)
	if err != nil {
		return ""
	}
	return u.Host
}

// locationToURI 前缀匹配转为以 / 结尾的 uri 以覆盖子路径；精确匹配与正则转为 exp:，不带 / 的前缀 uri 连子路径也会匹配
func locationToURI(location string) string {
	kind, rest := types.ParseLocation(location)
	switch kind {
	case types.LocationRegexAny:
		return "exp:(?i)" + rest
	case types.LocationRegex:
		return "exp:" + rest
	case types.LocationExact:
		return "exp:^" + regexp.QuoteMeta(types.NormalizePath(rest)) + "$"
	case types.LocationPrefix:
	}
	if rest = strings.Trim(rest, "/"); rest == "" {
		return "/"
	}
	return "/" + rest + "/"
}

// rulePath 规则覆盖的路径，用来找覆盖它的认证规则。正则规则匹配到哪里不可知，按根路径算
func rulePath(location string) string {
	kind, rest := types.ParseLocation(location)
	if kind == types.LocationRegex || kind == types.LocationRegexAny {
		return "/"
	}
	return "/" + strings.Trim(rest, "/")
}

// covers 前缀 prefix 是否覆盖 path，两者都不带结尾的 /
func covers(prefix, path string) bool {
	return prefix == "/" || path == prefix || strings.HasPrefix(path, prefix+"/")
}

// proxyPassthrough 命中代理规则的请求跳过后续 rewrite 的规则，模式与上下文 uri 一致；根路径返回空
func proxyPassthrough(location string) string {
	uri := locationToURI(location)
	if uri == "/" {
		return ""
	}
	if exp, ok := strings.CutPrefix(uri, "exp:"); ok {
		return exp + " - [L]"
	}
	return "^" + regexp.QuoteMeta(uri) + " - [L]"
}

// olsAddress 将 nginx 风格的上游地址转换为 OpenLiteSpeed 地址
func olsAddress(addr string) string {
	if path, ok := strings.CutPrefix(addr, "unix:"); ok {
		return "uds:/" + path
	}
	return addr
}

// addrFromOLS 将 OpenLiteSpeed 地址还原为 nginx 风格
func addrFromOLS(addr string) string {
	if path, ok := strings.CutPrefix(strings.ToLower(addr[:min(len(addr), 5)]), "uds:/"); ok {
		return "unix:" + path + addr[5:]
	}
	return addr
}

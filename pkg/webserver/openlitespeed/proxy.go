package openlitespeed

import (
	"fmt"
	"maps"
	"net"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 上游与代理均以 vhost 级外部应用加上下文的形式写入 vhconf，无法从配置推导的字段以 ace: 注释回写

const defaultInitTimeout = 60

func (v *baseVhost) upstreamName(name string) string {
	return fmt.Sprintf("%s_up_%s", v.safeName, safeName(name))
}

func (v *baseVhost) buildUpstreams(cfg *conf.Config) {
	for _, up := range v.upstreams {
		lbName := v.upstreamName(up.Name)
		servers := make([]string, 0, len(up.Servers))
		for addr := range up.Servers {
			servers = append(servers, addr)
		}
		sort.Strings(servers)

		var workers []string
		for i, addr := range servers {
			name := fmt.Sprintf("%s_%d", lbName, i)
			ext := cfg.AddBlock("extprocessor", name)
			addProxyApp(ext, olsAddress(addr), defaultInitTimeout, false)
			if options := strings.TrimSpace(up.Servers[addr]); options != "" {
				ext.AddMeta("options", options)
			}
			workers = append(workers, "proxy::"+name)
		}

		lb := cfg.AddBlock("extprocessor", lbName)
		lb.Add("type", "loadbalancer")
		lb.Add("workers", strings.Join(workers, ", "))
		// OLS 的 loadbalancer 没有算法选项，记下来供换回其它服务器时使用
		if algo := types.NormalizeAlgo(up.Algo); algo != "" {
			lb.AddMeta("algo", algo)
		}
		if up.Keepalive > 0 {
			lb.AddMeta("keepalive", strconv.Itoa(up.Keepalive))
		}
	}
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
			Algo:     types.NormalizeAlgo(lb.Meta("algo")),
			Resolver: []string{},
		}
		up.Keepalive, _ = strconv.Atoi(lb.Meta("keepalive"))
		for worker := range strings.SplitSeq(lb.Value("workers"), ",") {
			_, name, _ := strings.Cut(strings.TrimSpace(worker), "::")
			if member := cfg.GetBlock("extprocessor", name); member != nil {
				up.Servers[addrFromOLS(member.Value("address"))] = member.Meta("options")
			}
		}
		v.upstreams = append(v.upstreams, up)
	}
}

// buildProxies 认证路径与代理路径相同时合并进代理上下文，返回已合并的认证序号
func (v *baseVhost) buildProxies(cfg *conf.Config) map[int]bool {
	consumed := make(map[int]bool)
	for i, p := range v.proxies {
		handler, address := v.proxyHandler(p, i)
		if address != "" {
			timeout := defaultInitTimeout
			if p.Timeout != nil && p.Timeout.Read > 0 {
				timeout = int(p.Timeout.Read / time.Second)
			}
			addProxyApp(cfg.AddBlock("extprocessor", handler), address, timeout, p.Buffering)
		}

		uri := locationToURI(p.Location)
		ctx := cfg.AddBlock("context", uri)
		ctx.Add("type", "proxy")
		ctx.Add("handler", handler)
		ctx.AddMeta("location", p.Location)
		ctx.AddMeta("pass", p.Pass)
		if p.SNI != "" {
			ctx.AddMeta("sni", p.SNI)
		}

		headers := v.contextHeaders()
		if host := proxyHost(p); host != "" {
			headers = append(headers, "RequestHeader set Host "+host)
		}
		for _, name := range slices.Sorted(maps.Keys(p.Headers)) {
			headers = append(headers, fmt.Sprintf("RequestHeader set %s %s", name, headerValue(p.Headers[name])))
		}
		if p.ResponseHeaders != nil {
			for _, name := range slices.Sorted(maps.Keys(p.ResponseHeaders.Add)) {
				headers = append(headers, fmt.Sprintf("Header set %s %s", name, headerValue(p.ResponseHeaders.Add[name])))
			}
			for _, name := range p.ResponseHeaders.Hide {
				headers = append(headers, "Header unset "+name)
			}
		}
		setHeaders(ctx, headers)

		if p.AccessControl != nil && (len(p.AccessControl.Allow) > 0 || len(p.AccessControl.Deny) > 0) {
			ac := ctx.AddBlock("accessControl", "")
			if len(p.AccessControl.Allow) > 0 {
				ac.Add("allow", strings.Join(p.AccessControl.Allow, ", "))
			}
			if len(p.AccessControl.Deny) > 0 {
				ac.Add("deny", strings.Join(p.AccessControl.Deny, ", "))
			}
		}

		if path := authPathOf(p.Location); path != "" {
			for j, auth := range v.auths {
				if auth.Path == path && !consumed[j] {
					ctx.Add("realm", v.realmName(j))
					ctx.AddMeta("auth", auth.Path)
					consumed[j] = true
					break
				}
			}
		}
		custom, _ := customNodes(p)
		ctx.AppendCustom(custom)

		// 直连 HTTP 后端时同时代理 WebSocket
		if address != "" && !strings.HasPrefix(address, "https://") && !strings.HasPrefix(uri, "exp:") {
			cfg.AddBlock("websocket", uri).Add("address", address)
		}
	}

	return consumed
}

// customNodes 解析代理的自定义指令，SetProxies 时已校验过语法
func customNodes(p types.Proxy) ([]conf.Node, error) {
	cfg, err := Parse(p.Custom)
	if err != nil {
		return nil, fmt.Errorf("invalid custom directives of proxy %s: %w", p.Location, err)
	}
	return cfg.Nodes, nil
}

func addProxyApp(ext *conf.Directive, address string, timeout int, buffering bool) {
	ext.Add("type", "proxy")
	ext.Add("address", address)
	ext.Add("maxConns", "100")
	ext.Add("initTimeout", strconv.Itoa(timeout))
	ext.Add("retryTimeout", "0")
	ext.Add("persistConn", "1")
	ext.Add("pcKeepAliveTimeout", "60")
	ext.Add("respBuffer", map[bool]string{true: "1", false: "0"}[buffering])
}

func proxyHost(p types.Proxy) string {
	switch host := strings.TrimSpace(p.Host); host {
	case "$host":
		return ""
	case "", "$proxy_host":
		return upstreamHost(p.Pass)
	default:
		return host
	}
}

// headerValue 含变量的头值要写成 expr 形式，OLS 的 extraHeaders 走 Apache 指令语法，照搬 nginx 变量会当字面量发给上游
func headerValue(value string) string {
	native := types.ApacheVars.ToNative(value)
	if native == value {
		return value
	}
	return "expr=" + native
}

func parseHeaderValue(value string) string {
	expr, ok := strings.CutPrefix(value, "expr=")
	if !ok {
		return value
	}
	return types.ApacheVars.ToNginx(expr)
}

func upstreamHost(pass string) string {
	u, err := url.Parse(pass)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// proxyHandler 解析代理目标：命中上游时返回上游名，否则返回新建外部应用名与后端地址
func (v *baseVhost) proxyHandler(p types.Proxy, i int) (string, string) {
	u, err := url.Parse(p.Pass)
	if err != nil || u.Host == "" {
		return fmt.Sprintf("%s_proxy_%d", v.safeName, i), p.Pass
	}
	for _, up := range v.upstreams {
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

var headerOpPattern = regexp.MustCompile(`^(RequestHeader|Header)\s+(set|unset)\s+(\S+)\s*(.*)$`)

func (v *baseVhost) loadProxies(cfg *conf.Config) {
	for _, raw := range cfg.Blocks("context") {
		ctx, custom := raw.SplitCustom()
		if !strings.EqualFold(ctx.Value("type"), "proxy") {
			continue
		}
		p := types.Proxy{
			Location: ctx.Meta("location"),
			Pass:     ctx.Meta("pass"),
			SNI:      ctx.Meta("sni"),
			Resolver: []string{},
			Headers:  make(map[string]string),
			Replaces: make(map[string]string),
			Custom:   strings.TrimSpace(Export(&conf.Config{Block: conf.Block{Nodes: custom}})),
		}
		if p.Location == "" {
			p.Location = uriToLocation(ctx.Arg(0))
		}
		if ext := cfg.GetBlock("extprocessor", ctx.Value("handler")); ext != nil {
			p.Buffering = ext.Value("respBuffer") == "1"
			if p.Pass == "" {
				p.Pass = addrFromOLS(ext.Value("address"))
			}
			if timeout, _ := strconv.Atoi(ext.Value("initTimeout")); timeout > 0 && timeout != defaultInitTimeout {
				d := time.Duration(timeout) * time.Second
				p.Timeout = &types.TimeoutConfig{Connect: d, Read: d, Send: d}
			}
		}

		if d := ctx.Get("extraHeaders"); d != nil {
			for line := range strings.SplitSeq(d.Arg(0), "\n") {
				m := headerOpPattern.FindStringSubmatch(strings.TrimSpace(line))
				if m == nil || m[3] == hstsHeader {
					continue
				}
				switch {
				case m[1] == "RequestHeader" && m[3] == "Host":
					// 与从 pass 推导的默认值相同时回读成 nginx 的变量名，保证往返幂等
					if m[4] == upstreamHost(p.Pass) {
						p.Host = "$proxy_host"
					} else {
						p.Host = m[4]
					}
				case m[1] == "RequestHeader":
					p.Headers[m[3]] = parseHeaderValue(m[4])
				case m[2] == "set":
					if p.ResponseHeaders == nil {
						p.ResponseHeaders = &types.ResponseHeaderConfig{Add: make(map[string]string)}
					}
					p.ResponseHeaders.Add[m[3]] = parseHeaderValue(m[4])
				default:
					if p.ResponseHeaders == nil {
						p.ResponseHeaders = &types.ResponseHeaderConfig{Add: make(map[string]string)}
					}
					p.ResponseHeaders.Hide = append(p.ResponseHeaders.Hide, m[3])
				}
			}
		}

		if ac := ctx.GetBlock("accessControl"); ac != nil {
			p.AccessControl = &types.AccessControlConfig{
				Allow: splitList(ac.Value("allow")),
				Deny:  splitList(ac.Value("deny")),
			}
		}

		v.proxies = append(v.proxies, p)
	}
}

// locationToURI 前缀匹配转为以 / 结尾的 uri 以覆盖子路径，精确匹配不带 /，正则转为 exp:
func locationToURI(location string) string {
	location = strings.TrimSpace(location)
	switch {
	case strings.HasPrefix(location, "~*"):
		return "exp:(?i)" + strings.TrimSpace(location[2:])
	case strings.HasPrefix(location, "~"):
		return "exp:" + strings.TrimSpace(location[1:])
	case strings.HasPrefix(location, "="):
		return "/" + strings.Trim(strings.TrimSpace(location[1:]), "/")
	}
	location = strings.Trim(strings.TrimSpace(strings.TrimPrefix(location, "^~")), "/")
	if location == "" {
		return "/"
	}
	return "/" + location + "/"
}

// uriToLocation 无元数据时由上下文 uri 还原 location
func uriToLocation(uri string) string {
	if exp, ok := strings.CutPrefix(uri, "exp:"); ok {
		return "~ " + exp
	}
	return uri
}

// authPathOf 取前缀匹配 location 对应的认证路径，正则匹配返回空
func authPathOf(location string) string {
	location = strings.TrimSpace(location)
	if strings.HasPrefix(location, "~") {
		return ""
	}
	location = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(location, "^~"), "="))
	return "/" + strings.Trim(location, "/ ")
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

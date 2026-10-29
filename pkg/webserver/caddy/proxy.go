package caddy

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

// 上游写成顶层片段供 reverse_proxy 块内 import；代理按 nginx 的 location 优先级排入一个 route 块，
// 匹配器序号即用户定义顺序；无法从指令推导的字段以 ace: 注释回写

func (v *baseVhost) upstreamSnippet(name string) string {
	return fmt.Sprintf("ace_upstream_%s_%s", v.safeName, safeName(name))
}

var weightPattern = regexp.MustCompile(`\bweight=(\d+)`)

func (v *baseVhost) buildUpstreams(cfg *conf.Config) {
	for _, up := range v.upstreams {
		s := cfg.AddBlock("(" + v.upstreamSnippet(up.Name) + ")")
		s.AddMeta("upstream", up.Name)
		servers := slices.Sorted(maps.Keys(up.Servers))
		to := make([]string, 0, len(servers))
		weights := make([]string, 0, len(servers))
		weighted := false
		for _, addr := range servers {
			to = append(to, caddyAddress(addr))
			weight := "1"
			if m := weightPattern.FindStringSubmatch(up.Servers[addr]); m != nil {
				weight = m[1]
				weighted = weighted || weight != "1"
			}
			weights = append(weights, weight)
		}
		s.Add("to", to...)
		switch algo := types.NormalizeAlgo(up.Algo); {
		case algo != "":
			s.Add("lb_policy", algo)
		case weighted:
			s.Add("lb_policy", append([]string{"weighted_round_robin"}, weights...)...)
		default:
			s.Add("lb_policy", "round_robin")
		}
	}
}

func (v *baseVhost) loadUpstreams(cfg *conf.Config) {
	for _, s := range cfg.All() {
		if s.Block == nil || !strings.HasPrefix(s.Name, "(ace_upstream_"+v.safeName+"_") {
			continue
		}
		up := types.Upstream{
			Name:     s.Meta("upstream"),
			Servers:  make(map[string]string),
			Resolver: []string{},
		}
		var servers []string
		if to := s.Get("to"); to != nil {
			for _, addr := range to.Values() {
				servers = append(servers, nginxAddress(addr))
				up.Servers[nginxAddress(addr)] = ""
			}
		}
		if lb := s.Get("lb_policy"); lb != nil {
			switch lb.Arg(0) {
			case "round_robin":
			case "weighted_round_robin":
				for i, weight := range lb.ArgsFrom(1) {
					if i < len(servers) && weight != "1" {
						up.Servers[servers[i]] = "weight=" + weight
					}
				}
			default:
				up.Algo = types.NormalizeAlgo(lb.Arg(0))
			}
		}
		v.upstreams = append(v.upstreams, up)
	}
}

// proxyOrder 按 nginx 的 location 优先级排序：精确匹配、^~ 前缀（长者优先）、正则（书写顺序）、普通前缀（长者优先）
func proxyOrder(proxies []types.Proxy) []int {
	rank := func(location string) (int, int) {
		location = strings.TrimSpace(location)
		switch {
		case strings.HasPrefix(location, "="):
			return 0, 0
		case strings.HasPrefix(location, "^~"):
			return 1, -len(strings.TrimSpace(location[2:]))
		case strings.HasPrefix(location, "~"):
			return 2, 0
		}
		return 3, -len(location)
	}
	order := make([]int, len(proxies))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		ra, la := rank(proxies[order[a]].Location)
		rb, lb := rank(proxies[order[b]].Location)
		if ra != rb {
			return ra < rb
		}
		return la < lb
	})
	return order
}

func (v *baseVhost) buildProxies(body *conf.Block) {
	if len(v.proxies) == 0 {
		return
	}
	body.Append(&conf.Blank{})
	route := body.AddBlock("route")
	for _, i := range proxyOrder(v.proxies) {
		p := v.proxies[i]
		name := fmt.Sprintf("@ace_proxy_%d", i)
		route.Add(name, locationMatcher(p.Location)...)
		h := route.AddBlock("handle", name)
		h.AddMeta("location", p.Location)
		h.AddMeta("pass", p.Pass)

		if p.ClientMaxBodySize > 0 {
			h.AddBlock("request_body").Add("max_size", strconv.FormatInt(p.ClientMaxBodySize, 10))
		}
		if p.AccessControl != nil {
			allow, deny := accessLists(p.AccessControl)
			if len(deny) > 0 {
				name := fmt.Sprintf("@ace_deny_%d", i)
				h.Add(name, append([]string{"remote_ip"}, deny...)...)
				h.Add("respond", name, "403")
			}
			if len(allow) > 0 {
				name := fmt.Sprintf("@ace_allow_%d", i)
				h.Add(name, append([]string{"not", "remote_ip"}, allow...)...)
				h.Add("respond", name, "403")
			}
		}
		// proxy_pass 带路径时把匹配到的前缀替换为该路径，正则 location 不支持
		if prefix, path := passPrefixRewrite(p); path != "" {
			h.Add("uri", "path_regexp", "^"+regexp.QuoteMeta(prefix), path)
		}
		for _, from := range slices.Sorted(maps.Keys(p.Replaces)) {
			h.Add("replace", from, p.Replaces[from])
		}

		rp := h.AddBlock("reverse_proxy")
		snippet, address := v.proxyTarget(p.Pass)
		if snippet != "" {
			rp.Add("import", snippet)
		} else {
			rp.AppendArg(address)
		}
		switch host := strings.TrimSpace(p.Host); host {
		case "$host":
		case "", "$proxy_host":
			// 与 nginx 的默认值 $proxy_host 一致，发上游主机名而不是访客 Host
			rp.Add("header_up", "Host", "{upstream_hostport}")
		default:
			rp.Add("header_up", "Host", nginxVariables.ToNative(host))
		}
		// X-Forwarded-* 由 Caddy 自动附加，X-Real-IP 与 nginx 方言一样默认补上
		if _, ok := p.Headers[realIPHeader]; !ok {
			rp.Add("header_up", realIPHeader, realIPValue)
		}
		for _, name := range slices.Sorted(maps.Keys(p.Headers)) {
			// nginx 习惯手写的 X-Forwarded-For 链由 Caddy 自动维护，写死反而会丢掉上游链路
			if strings.EqualFold(name, "X-Forwarded-For") && p.Headers[name] == "$proxy_add_x_forwarded_for" {
				continue
			}
			rp.Add("header_up", name, nginxVariables.ToNative(p.Headers[name]))
		}
		if p.ResponseHeaders != nil {
			for _, name := range slices.Sorted(maps.Keys(p.ResponseHeaders.Add)) {
				rp.Add("header_down", name, nginxVariables.ToNative(p.ResponseHeaders.Add[name]))
			}
			for _, name := range p.ResponseHeaders.Hide {
				rp.Add("header_down", "-"+name)
			}
		}
		if !p.Buffering {
			rp.Add("flush_interval", "-1")
		}
		if p.Retry != nil {
			if p.Retry.Tries > 0 {
				rp.Add("lb_retries", strconv.Itoa(p.Retry.Tries))
			}
			if p.Retry.Timeout > 0 {
				rp.Add("lb_try_duration", p.Retry.Timeout.String())
			}
		}
		v.buildTransport(rp, p, strings.HasPrefix(strings.ToLower(p.Pass), "https://"))
		if rp.Len() == 0 {
			rp.Block = nil
		}
		custom, _ := customNodes(p)
		h.AppendCustom(custom)
	}
}

// customNodes 解析代理的自定义指令，SetProxies 时已校验过语法
func customNodes(p types.Proxy) ([]conf.Node, error) {
	cfg, err := Parse(p.Custom)
	if err != nil {
		return nil, fmt.Errorf("invalid custom directives of proxy %s: %w", p.Location, err)
	}
	return cfg.Nodes, nil
}

// buildTransport 后端 TLS、SNI、协议版本与超时都在 transport 块，nginx 默认不校验后端证书，这里保持一致
func (v *baseVhost) buildTransport(rp *conf.Directive, p types.Proxy, https bool) {
	t := conf.Blk("transport", "http")
	if https || p.SNI != "" {
		t.Add("tls")
		if p.SNI != "" {
			t.Add("tls_server_name", p.SNI)
		}
		if p.SSLBackend == nil || !p.SSLBackend.Verify {
			t.Add("tls_insecure_skip_verify")
		}
		if p.SSLBackend != nil && p.SSLBackend.TrustedCertificate != "" {
			t.Add("tls_trust_pool", "file", p.SSLBackend.TrustedCertificate)
		}
	}
	if p.HTTPVersion == "2" {
		t.Add("versions", "h2c", "2")
	}
	if p.Timeout != nil {
		if p.Timeout.Connect > 0 {
			t.Add("dial_timeout", p.Timeout.Connect.String())
		}
		if p.Timeout.Read > 0 {
			t.Add("response_header_timeout", p.Timeout.Read.String())
		}
	}
	if t.Len() > 0 {
		rp.Append(t)
	}
}

// proxyTarget 命中上游返回片段名，否则返回 Caddy 形式的后端地址
func (v *baseVhost) proxyTarget(pass string) (string, string) {
	u, err := url.Parse(pass)
	if err != nil || u.Host == "" {
		return "", pass
	}
	for _, up := range v.upstreams {
		if up.Name == u.Hostname() {
			return v.upstreamSnippet(up.Name), ""
		}
	}
	// nginx 的 unix 套接字写法 http://unix:/path.sock，路径落在 Path 里
	if u.Host == "unix:" {
		return "", "unix/" + u.Path
	}
	port := u.Port()
	if port == "" {
		port = map[bool]string{true: "443", false: "80"}[u.Scheme == "https"]
	}
	return "", net.JoinHostPort(u.Hostname(), port)
}

func passPrefixRewrite(p types.Proxy) (string, string) {
	location := strings.TrimSpace(p.Location)
	if strings.HasPrefix(location, "~") {
		return "", ""
	}
	u, err := url.Parse(p.Pass)
	if err != nil || u.Path == "" || u.Host == "unix:" {
		return "", ""
	}
	prefix := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(location, "^~"), "="))
	return prefix, u.Path
}

func (v *baseVhost) loadProxies(body *conf.Block) {
	type indexed struct {
		index int
		proxy types.Proxy
	}
	var found []indexed
	for _, route := range body.GetAll("route") {
		for _, h := range route.GetAll("handle") {
			index, ok := strings.CutPrefix(h.Arg(0), "@ace_proxy_")
			if !ok {
				continue
			}
			i, _ := strconv.Atoi(index)
			found = append(found, indexed{index: i, proxy: v.loadProxy(h)})
		}
	}
	sort.SliceStable(found, func(a, b int) bool { return found[a].index < found[b].index })
	for _, item := range found {
		v.proxies = append(v.proxies, item.proxy)
	}
}

func (v *baseVhost) loadProxy(h *conf.Directive) types.Proxy {
	h, custom := h.SplitCustom()
	p := types.Proxy{
		Location:  h.Meta("location"),
		Pass:      h.Meta("pass"),
		Buffering: true,
		Resolver:  []string{},
		Headers:   make(map[string]string),
		Replaces:  make(map[string]string),
		Custom:    strings.TrimSpace(Export(&conf.Config{Block: conf.Block{Nodes: custom}})),
	}
	if body := h.Get("request_body"); body != nil {
		p.ClientMaxBodySize, _ = strconv.ParseInt(body.Get("max_size").Arg(0), 10, 64)
	}
	for _, d := range h.GetAll("respond") {
		m := h.Get(d.Arg(0))
		if m == nil {
			continue
		}
		if p.AccessControl == nil {
			p.AccessControl = &types.AccessControlConfig{}
		}
		if m.Arg(0) == "not" {
			p.AccessControl.Allow = append(p.AccessControl.Allow, m.ArgsFrom(2)...)
		} else if deny := m.ArgsFrom(1); slices.Equal(deny, denyAll) {
			p.AccessControl.Deny = append(p.AccessControl.Deny, "all")
		} else {
			p.AccessControl.Deny = append(p.AccessControl.Deny, deny...)
		}
	}
	for _, d := range h.GetAll("replace") {
		p.Replaces[d.Arg(0)] = d.Arg(1)
	}

	rp := h.Get("reverse_proxy")
	if rp == nil {
		return p
	}
	for _, d := range rp.GetAll("header_up") {
		switch {
		case d.Arg(0) == "Host":
			// 回读成 nginx 的变量名，切换服务器时这个值会原样交给别的方言
			p.Host = strings.Replace(d.Arg(1), "{upstream_hostport}", "$proxy_host", 1)
		case d.Arg(0) == realIPHeader && d.Arg(1) == realIPValue:
		default:
			p.Headers[d.Arg(0)] = nginxVariables.ToNginx(d.Arg(1))
		}
	}
	for _, d := range rp.GetAll("header_down") {
		if p.ResponseHeaders == nil {
			p.ResponseHeaders = &types.ResponseHeaderConfig{Add: make(map[string]string)}
		}
		if name, hide := strings.CutPrefix(d.Arg(0), "-"); hide {
			p.ResponseHeaders.Hide = append(p.ResponseHeaders.Hide, name)
		} else {
			p.ResponseHeaders.Add[d.Arg(0)] = nginxVariables.ToNginx(d.Arg(1))
		}
	}
	if d := rp.Get("flush_interval"); d != nil && d.Arg(0) == "-1" {
		p.Buffering = false
	}
	retries, _ := strconv.Atoi(rp.Get("lb_retries").Arg(0))
	tryDuration, _ := time.ParseDuration(rp.Get("lb_try_duration").Arg(0))
	if retries > 0 || tryDuration > 0 {
		p.Retry = &types.RetryConfig{Tries: retries, Timeout: tryDuration}
	}

	t := rp.Get("transport")
	if t == nil {
		return p
	}
	p.SNI = t.Get("tls_server_name").Arg(0)
	if t.Get("tls") != nil {
		skip := t.Get("tls_insecure_skip_verify") != nil
		ca := t.Get("tls_trust_pool").Arg(1)
		if !skip || ca != "" {
			p.SSLBackend = &types.SSLBackendConfig{Verify: !skip, TrustedCertificate: ca}
		}
	}
	if versions := t.Get("versions"); versions != nil && slices.Contains(versions.Values(), "h2c") {
		p.HTTPVersion = "2"
	}
	connect, _ := time.ParseDuration(t.Get("dial_timeout").Arg(0))
	read, _ := time.ParseDuration(t.Get("response_header_timeout").Arg(0))
	if connect > 0 || read > 0 {
		p.Timeout = &types.TimeoutConfig{Connect: connect, Read: read}
	}
	return p
}

const (
	realIPHeader = "X-Real-IP"
	realIPValue  = "{remote_host}"
)

var denyAll = []string{"0.0.0.0/0", "::/0"}

// accessLists nginx 习惯的 all：允许列表里等于不限制，有允许列表时 deny all 已隐含，否则展开为全部网段
func accessLists(ac *types.AccessControlConfig) ([]string, []string) {
	var allow, deny []string
	for _, ip := range ac.Allow {
		if !strings.EqualFold(ip, "all") {
			allow = append(allow, ip)
		}
	}
	for _, ip := range ac.Deny {
		switch {
		case !strings.EqualFold(ip, "all"):
			deny = append(deny, ip)
		case len(allow) == 0:
			deny = append(deny, denyAll...)
		}
	}
	return allow, deny
}

func locationMatcher(location string) []string {
	location = strings.TrimSpace(location)
	switch {
	case strings.HasPrefix(location, "~*"):
		return []string{"path_regexp", "(?i)" + strings.TrimSpace(location[2:])}
	case strings.HasPrefix(location, "~"):
		return []string{"path_regexp", strings.TrimSpace(location[1:])}
	case strings.HasPrefix(location, "="):
		return []string{"path", strings.TrimSpace(location[1:])}
	}
	return []string{"path", strings.TrimSpace(strings.TrimPrefix(location, "^~")) + "*"}
}

func caddyAddress(addr string) string {
	if path, ok := strings.CutPrefix(addr, "unix:"); ok {
		return "unix/" + path
	}
	return addr
}

func nginxAddress(addr string) string {
	if path, ok := strings.CutPrefix(addr, "unix/"); ok {
		return "unix:" + path
	}
	return addr
}

// 表首命中即回读时的规范原像，所以 $host 要排在 $http_host 之前
var nginxVariables = types.NewVarMap([][2]string{
	{"$proxy_add_x_forwarded_for", "{remote_host}"},
	{"$remote_addr", "{remote_host}"},
	{"$host", "{host}"},
	{"$http_host", "{host}"},
	{"$scheme", "{scheme}"},
	{"$request_uri", "{uri}"},
	{"$server_port", "{port}"},
})

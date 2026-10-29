package nginx

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

var proxyFilePattern = regexp.MustCompile(`^(\d{3})-proxy\.conf$`)

// parseDuration 解析 nginx 时长，如 "5s" "5m" "5h"，无单位按秒
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	unit := time.Second
	switch s[len(s)-1] {
	case 'm':
		unit, s = time.Minute, s[:len(s)-1]
	case 'h':
		unit, s = time.Hour, s[:len(s)-1]
	case 's':
		s = s[:len(s)-1]
	}
	value, _ := strconv.Atoi(s)
	return time.Duration(value) * unit
}

// formatDuration 用最大整除单位表示时长
func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	switch {
	case seconds > 0 && seconds%3600 == 0:
		return fmt.Sprintf("%dh", seconds/3600)
	case seconds > 0 && seconds%60 == 0:
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

// parseSize 解析 nginx 大小，如 "10m" "512k" "1g"
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	unit := int64(1)
	switch s[len(s)-1] {
	case 'k', 'K':
		unit, s = 1024, s[:len(s)-1]
	case 'm', 'M':
		unit, s = 1024*1024, s[:len(s)-1]
	case 'g', 'G':
		unit, s = 1024*1024*1024, s[:len(s)-1]
	}
	value, _ := strconv.ParseInt(s, 10, 64)
	return value * unit
}

func formatSize(bytes int64) string {
	for _, u := range []struct {
		size   int64
		suffix string
	}{{1024 * 1024 * 1024, "g"}, {1024 * 1024, "m"}, {1024, "k"}} {
		if bytes > 0 && bytes%u.size == 0 {
			return strconv.FormatInt(bytes/u.size, 10) + u.suffix
		}
	}
	return strconv.FormatInt(bytes, 10)
}

func parseProxyFiles(siteDir string) []types.Proxy {
	var proxies []types.Proxy
	for _, file := range listFiles(siteDir, proxyFilePattern, ProxyStartNum, ProxyEndNum) {
		cfg, err := ParseFile(file)
		if err != nil {
			continue
		}
		if loc := cfg.GetBlock("location"); loc != nil {
			proxies = append(proxies, parseProxy(loc))
		}
	}
	return proxies
}

// standardHeaders 生成时固定写入的请求头，回读时不算作自定义头
var standardHeaders = map[string]bool{
	"Host": true, "X-Real-IP": true, "X-Forwarded-For": true, "X-Forwarded-Proto": true,
	"Upgrade": true, "Connection": true, "Early-Data": true, "Accept-Encoding": true,
}

func parseProxy(loc *conf.Directive) types.Proxy {
	loc, custom := loc.SplitCustom()
	p := types.Proxy{
		Location:          strings.Join(loc.Values(), " "),
		Pass:              loc.Value("proxy_pass"),
		SNI:               loc.Value("proxy_ssl_name"),
		Buffering:         loc.Value("proxy_buffering") == "on",
		HTTPVersion:       loc.Value("proxy_http_version"),
		ResolverTimeout:   parseDuration(loc.Value("resolver_timeout")),
		ClientMaxBodySize: parseSize(loc.Value("client_max_body_size")),
		Resolver:          []string{},
		Headers:           make(map[string]string),
		Replaces:          make(map[string]string),
		Custom:            strings.TrimSpace(Export(&conf.Config{Block: conf.Block{Nodes: custom}})),
	}
	if resolver := loc.Get("resolver").Values(); resolver != nil {
		p.Resolver = resolver
	}
	for _, d := range loc.GetAll("proxy_set_header") {
		name, value := d.Arg(0), strings.Join(d.ArgsFrom(1), " ")
		switch {
		case name == "Host":
			p.Host = value
		case !standardHeaders[name]:
			p.Headers[name] = value
		}
	}
	for _, d := range loc.GetAll("sub_filter") {
		p.Replaces[d.Arg(0)] = d.Arg(1)
	}
	if loc.Has("proxy_cache") && loc.Value("proxy_cache") != "off" {
		p.Cache = parseProxyCache(loc)
	}
	if loc.Has("proxy_connect_timeout") || loc.Has("proxy_read_timeout") || loc.Has("proxy_send_timeout") {
		p.Timeout = &types.TimeoutConfig{
			Connect: parseDuration(loc.Value("proxy_connect_timeout")),
			Read:    parseDuration(loc.Value("proxy_read_timeout")),
			Send:    parseDuration(loc.Value("proxy_send_timeout")),
		}
	}
	if loc.Has("proxy_next_upstream") || loc.Has("proxy_next_upstream_tries") || loc.Has("proxy_next_upstream_timeout") {
		p.Retry = &types.RetryConfig{
			Conditions: loc.Get("proxy_next_upstream").Values(),
			Tries:      atoi(loc.Value("proxy_next_upstream_tries")),
			Timeout:    parseDuration(loc.Value("proxy_next_upstream_timeout")),
		}
	}
	if loc.Has("proxy_ssl_verify") || loc.Has("proxy_ssl_trusted_certificate") || loc.Has("proxy_ssl_verify_depth") {
		p.SSLBackend = &types.SSLBackendConfig{
			Verify:             loc.Value("proxy_ssl_verify") == "on",
			TrustedCertificate: loc.Value("proxy_ssl_trusted_certificate"),
			VerifyDepth:        atoi(loc.Value("proxy_ssl_verify_depth")),
		}
	}
	if loc.Has("proxy_hide_header") || loc.Has("add_header") {
		p.ResponseHeaders = &types.ResponseHeaderConfig{Add: make(map[string]string)}
		for _, d := range loc.GetAll("proxy_hide_header") {
			p.ResponseHeaders.Hide = append(p.ResponseHeaders.Hide, d.Arg(0))
		}
		for _, d := range loc.GetAll("add_header") {
			p.ResponseHeaders.Add[d.Arg(0)] = d.Arg(1)
		}
	}
	if loc.Has("allow") || loc.Has("deny") {
		p.AccessControl = &types.AccessControlConfig{}
		for _, d := range loc.GetAll("allow") {
			p.AccessControl.Allow = append(p.AccessControl.Allow, d.Arg(0))
		}
		for _, d := range loc.GetAll("deny") {
			p.AccessControl.Deny = append(p.AccessControl.Deny, d.Arg(0))
		}
	}
	return p
}

func parseProxyCache(loc *conf.Directive) *types.CacheConfig {
	cache := &types.CacheConfig{
		Valid:             make(map[string]string),
		NoCacheConditions: []string{},
		UseStale:          []string{},
		Methods:           []string{},
		BackgroundUpdate:  loc.Value("proxy_cache_background_update") == "on",
		Lock:              loc.Value("proxy_cache_lock") == "on",
		MinUses:           atoi(loc.Value("proxy_cache_min_uses")),
		Key:               loc.Value("proxy_cache_key"),
	}
	// proxy_cache_valid 最后一个参数是时长，其余为状态码；仅时长则归入 any
	for _, d := range loc.GetAll("proxy_cache_valid") {
		parts := d.Values()
		switch {
		case len(parts) >= 2:
			cache.Valid[strings.Join(parts[:len(parts)-1], " ")] = parts[len(parts)-1]
		case len(parts) == 1:
			cache.Valid["any"] = parts[0]
		}
	}
	// proxy_cache_bypass 与 proxy_no_cache 参数一致，取其一
	if v := loc.Get("proxy_cache_bypass").Values(); v != nil {
		cache.NoCacheConditions = v
	}
	if v := loc.Get("proxy_cache_use_stale").Values(); v != nil {
		cache.UseStale = v
	}
	if v := loc.Get("proxy_cache_methods").Values(); v != nil {
		cache.Methods = v
	}
	return cache
}

func writeProxyFiles(siteDir string, proxies []types.Proxy) error {
	if len(proxies) > ProxyEndNum-ProxyStartNum+1 {
		return fmt.Errorf("proxy rules exceed limit (%d)", ProxyEndNum-ProxyStartNum+1)
	}
	// 先全部生成，自定义指令有语法错误时不动已有文件
	nodes := make([]*conf.Directive, len(proxies))
	for i, proxy := range proxies {
		loc, err := proxyNode(proxy)
		if err != nil {
			return err
		}
		nodes[i] = loc
	}
	if err := clearFiles(siteDir, proxyFilePattern, ProxyStartNum, ProxyEndNum); err != nil {
		return err
	}
	for i, proxy := range proxies {
		path := filepath.Join(siteDir, fmt.Sprintf("%03d-proxy.conf", ProxyStartNum+i))
		if err := writeFragment(path, conf.Cmt(fmt.Sprintf("Reverse proxy: %s -> %s", proxy.Location, proxy.Pass)), nodes[i]); err != nil {
			return err
		}
	}
	return nil
}

func clearProxyFiles(siteDir string) error {
	return clearFiles(siteDir, proxyFilePattern, ProxyStartNum, ProxyEndNum)
}

func proxyNode(p types.Proxy) (*conf.Directive, error) {
	custom, err := Parse(p.Custom)
	if err != nil {
		return nil, fmt.Errorf("invalid custom directives of proxy %s: %w", p.Location, err)
	}
	loc := conf.Blk("location", strings.Fields(p.Location)...)

	if p.AccessControl != nil {
		for _, ip := range p.AccessControl.Allow {
			loc.Add("allow", ip)
		}
		for _, ip := range p.AccessControl.Deny {
			loc.Add("deny", ip)
		}
	}
	if p.ClientMaxBodySize > 0 {
		loc.Add("client_max_body_size", formatSize(p.ClientMaxBodySize))
	}
	if len(p.Resolver) > 0 {
		loc.Add("resolver", p.Resolver...)
		if p.ResolverTimeout > 0 {
			loc.Add("resolver_timeout", formatDuration(p.ResolverTimeout))
		}
	}

	loc.Add("proxy_pass", p.Pass)
	httpVersion := p.HTTPVersion
	if httpVersion == "" {
		httpVersion = "1.1"
	}
	loc.Add("proxy_http_version", httpVersion)
	host := p.Host
	if host == "" {
		host = "$proxy_host"
	}
	loc.Add("proxy_set_header", "Host", host)
	loc.Add("proxy_set_header", "X-Real-IP", "$remote_addr")
	loc.Add("proxy_set_header", "X-Forwarded-For", "$proxy_add_x_forwarded_for")
	loc.Add("proxy_set_header", "X-Forwarded-Proto", "$scheme")
	loc.Add("proxy_set_header", "Upgrade", "$http_upgrade")
	loc.Add("proxy_set_header", "Connection", "$connection_upgrade")
	loc.Add("proxy_set_header", "Early-Data", "$ssl_early_data")

	if strings.HasPrefix(p.Pass, "https") {
		loc.Add("proxy_ssl_protocols", "TLSv1.2", "TLSv1.3")
		loc.Add("proxy_ssl_session_reuse", "off")
		loc.Add("proxy_ssl_server_name", "on")
		sni := p.SNI
		if sni == "" {
			sni = "$proxy_host"
		}
		loc.Add("proxy_ssl_name", sni)
		if p.SSLBackend != nil && p.SSLBackend.Verify {
			loc.Add("proxy_ssl_verify", "on")
			if p.SSLBackend.VerifyDepth > 0 {
				loc.Add("proxy_ssl_verify_depth", strconv.Itoa(p.SSLBackend.VerifyDepth))
			}
			if p.SSLBackend.TrustedCertificate != "" {
				loc.Add("proxy_ssl_trusted_certificate", p.SSLBackend.TrustedCertificate)
			}
		}
	}

	if p.Timeout != nil {
		addDuration(loc, "proxy_connect_timeout", p.Timeout.Connect)
		addDuration(loc, "proxy_read_timeout", p.Timeout.Read)
		addDuration(loc, "proxy_send_timeout", p.Timeout.Send)
	}
	if p.Retry != nil {
		if len(p.Retry.Conditions) > 0 {
			loc.Add("proxy_next_upstream", p.Retry.Conditions...)
		}
		if p.Retry.Tries > 0 {
			loc.Add("proxy_next_upstream_tries", strconv.Itoa(p.Retry.Tries))
		}
		addDuration(loc, "proxy_next_upstream_timeout", p.Retry.Timeout)
	}

	buffering := "off"
	if p.Buffering {
		buffering = "on"
	}
	loc.Add("proxy_buffering", buffering)

	// 不写 off 会继承全局的 proxy_cache
	if p.Cache != nil {
		addProxyCache(loc, p.Cache)
	} else {
		loc.Add("proxy_cache", "off")
	}
	for _, name := range slices.Sorted(maps.Keys(p.Headers)) {
		loc.Add("proxy_set_header", name, p.Headers[name])
	}
	if len(p.Replaces) > 0 {
		loc.Add("proxy_set_header", "Accept-Encoding", "")
		loc.Add("sub_filter_once", "off")
		for _, from := range slices.Sorted(maps.Keys(p.Replaces)) {
			loc.Add("sub_filter", from, p.Replaces[from])
		}
	}
	if p.ResponseHeaders != nil {
		for _, header := range p.ResponseHeaders.Hide {
			loc.Add("proxy_hide_header", header)
		}
		for _, name := range slices.Sorted(maps.Keys(p.ResponseHeaders.Add)) {
			loc.Add("add_header", name, p.ResponseHeaders.Add[name], "always")
		}
	}
	loc.AppendCustom(custom.Nodes)
	return loc, nil
}

func addProxyCache(loc *conf.Directive, cache *types.CacheConfig) {
	loc.Add("proxy_cache", "cache_one")
	if len(cache.Valid) == 0 {
		loc.Add("proxy_cache_valid", "200", "302", "10m")
		loc.Add("proxy_cache_valid", "404", "10s")
	}
	for _, codes := range slices.Sorted(maps.Keys(cache.Valid)) {
		if codes == "any" {
			loc.Add("proxy_cache_valid", cache.Valid[codes])
		} else {
			loc.Add("proxy_cache_valid", append(strings.Fields(codes), cache.Valid[codes])...)
		}
	}
	if len(cache.NoCacheConditions) > 0 {
		loc.Add("proxy_cache_bypass", cache.NoCacheConditions...)
		loc.Add("proxy_no_cache", cache.NoCacheConditions...)
	}
	if len(cache.UseStale) > 0 {
		loc.Add("proxy_cache_use_stale", cache.UseStale...)
	}
	if cache.BackgroundUpdate {
		loc.Add("proxy_cache_background_update", "on")
	}
	if cache.Lock {
		loc.Add("proxy_cache_lock", "on")
	}
	if cache.MinUses > 0 {
		loc.Add("proxy_cache_min_uses", strconv.Itoa(cache.MinUses))
	}
	if len(cache.Methods) > 0 {
		loc.Add("proxy_cache_methods", cache.Methods...)
	}
	if cache.Key != "" {
		loc.Add("proxy_cache_key", cache.Key)
	}
}

func addDuration(loc *conf.Directive, name string, d time.Duration) {
	if d > 0 {
		loc.Add(name, formatDuration(d))
	}
}

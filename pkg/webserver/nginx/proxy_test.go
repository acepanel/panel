package nginx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 三个子测试按顺序切换同一个 vhost，不能并行
func TestProxyCacheToggle(t *testing.T) {
	v, dir := setupProxyVhost(t)
	for _, tt := range []struct {
		name  string
		cache *types.CacheConfig
		want  string
	}{
		{"首次禁用", nil, "off"},
		{"开启缓存", &types.CacheConfig{Valid: map[string]string{"200": "10m"}}, "cache_one"},
		{"再次禁用", nil, "off"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			must.NoError(t, v.SetProxies([]types.Proxy{{Location: "/", Pass: "http://127.0.0.1:8080", Cache: tt.cache}}))
			must.NoError(t, v.Save())
			var err error
			v, err = newProxyVhost(dir)
			must.NoError(t, err)
			cfg, err := ParseFile(filepath.Join(dir, "site", "200-proxy.conf"))
			must.NoError(t, err)
			loc := cfg.GetBlock("location")
			check.Equal(t, loc.Value("proxy_cache"), tt.want)
			check.Equal(t, len(loc.GetAll("proxy_cache")), 1)
			if tt.cache == nil {
				check.Nil(t, v.Proxies()[0].Cache)
			} else {
				must.NotNil(t, v.Proxies()[0].Cache)
				check.DeepEqual(t, v.Proxies()[0].Cache.Valid, tt.cache.Valid)
			}
		})
	}
}

func TestProxyCustom(t *testing.T) {
	v, dir := setupProxyVhost(t)
	custom := "proxy_cache_convert_head off;\n# 注释保留\nproxy_set_header X-Custom value;"
	must.NoError(t, v.SetProxies([]types.Proxy{{Location: "/", Pass: "http://127.0.0.1:8080", Custom: custom}}))
	cfg, err := ParseFile(filepath.Join(dir, "site", "200-proxy.conf"))
	must.NoError(t, err)
	// 要落在 location 里才只对该规则生效
	check.Equal(t, cfg.GetBlock("location").Value("proxy_cache_convert_head"), "off")

	got := v.Proxies()
	must.Len(t, got, 1)
	check.Equal(t, got[0].Custom, custom)
	// 自定义指令里的请求头不回读成面板字段
	check.Empty(t, got[0].Headers)

	// 语法错误时报错，已有文件保持原样
	check.Error(t, v.SetProxies([]types.Proxy{{Location: "/", Pass: "http://127.0.0.1:8080", Custom: "location /x {"}}))
	check.Equal(t, v.Proxies()[0].Custom, custom)
}

func TestProxyResponseHeadersInherit(t *testing.T) {
	v, dir := setupProxyVhost(t)
	for _, tt := range []struct {
		name    string
		headers *types.ResponseHeaderConfig
		want    string
	}{
		{"添加响应头时合并继承", &types.ResponseHeaderConfig{Hide: []string{"X-Powered-By"}, Add: map[string]string{"X-Proxy": "ace"}}, "merge"},
		{"仅隐藏响应头时不写", &types.ResponseHeaderConfig{Hide: []string{"X-Powered-By"}, Add: map[string]string{}}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			must.NoError(t, v.SetProxies([]types.Proxy{{Location: "/", Pass: "http://127.0.0.1:8080", ResponseHeaders: tt.headers}}))
			cfg, err := ParseFile(filepath.Join(dir, "site", "200-proxy.conf"))
			must.NoError(t, err)
			check.Equal(t, cfg.GetBlock("location").Value("add_header_inherit"), tt.want)

			got := v.Proxies()
			must.Len(t, got, 1)
			check.DeepEqual(t, got[0].ResponseHeaders, tt.headers)
			check.Empty(t, got[0].Custom)
		})
	}
}

// 与内置头同名的自定义头顶替内置值而不是多写一条；值与内置相同的不算自定义头，保存一次即收敛
func TestProxyBuiltinHeaders(t *testing.T) {
	v, dir := setupProxyVhost(t)
	path := filepath.Join(dir, "site", "200-proxy.conf")
	must.NoError(t, v.SetProxies([]types.Proxy{{
		Location: "/", Pass: "http://127.0.0.1:8080", Replaces: map[string]string{"old": "new"},
		Headers: map[string]string{"X-Forwarded-Proto": "https", "X-Real-IP": "$remote_addr", "X-Custom": "1", "Accept-Encoding": "gzip"},
	}}))
	cfg, err := ParseFile(path)
	must.NoError(t, err)
	sent := map[string][]string{}
	for _, d := range cfg.GetBlock("location").GetAll("proxy_set_header") {
		sent[d.Arg(0)] = append(sent[d.Arg(0)], d.Arg(1))
	}
	check.DeepEqual(t, sent["X-Forwarded-Proto"], []string{"https"})
	check.DeepEqual(t, sent["X-Real-IP"], []string{"$remote_addr"})
	// 内容替换要清掉 Accept-Encoding，用户给的不能再写一条
	check.DeepEqual(t, sent["Accept-Encoding"], []string{""})

	got := v.Proxies()
	must.Len(t, got, 1)
	check.DeepEqual(t, got[0].Headers, map[string]string{"X-Forwarded-Proto": "https", "X-Custom": "1"})

	first, err := os.ReadFile(path)
	must.NoError(t, err)
	must.NoError(t, v.SetProxies(got))
	second, err := os.ReadFile(path)
	must.NoError(t, err)
	check.Equal(t, string(second), string(first))
}

// 允许列表非空即白名单；nginx 取首条命中的规则，拒绝名单写在前面才能从允许的网段里排除个别地址
func TestProxyAccessControl(t *testing.T) {
	for _, tt := range []struct {
		name string
		ac   types.AccessControlConfig
		conf string
		want *types.AccessControlConfig
	}{
		{"只填允许列表", types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}}, "    allow 10.0.0.0/8;\n    deny all;\n    proxy_pass", &types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}}},
		{
			"允许并排除", types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
			"    deny 10.0.0.99;\n    allow 10.0.0.0/8;\n    deny all;\n    proxy_pass",
			&types.AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}},
		},
		{"只填拒绝列表", types.AccessControlConfig{Deny: []string{"10.0.0.99"}}, "{\n    deny 10.0.0.99;\n    proxy_pass", &types.AccessControlConfig{Deny: []string{"10.0.0.99"}}},
		{"全部拒绝", types.AccessControlConfig{Deny: []string{"all"}}, "{\n    deny all;\n    proxy_pass", &types.AccessControlConfig{Deny: []string{"all"}}},
		{"全部允许", types.AccessControlConfig{Allow: []string{"all"}}, "{\n    proxy_pass", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v, dir := setupProxyVhost(t)
			must.NoError(t, v.SetProxies([]types.Proxy{{Location: "/", Pass: "http://127.0.0.1:8080", AccessControl: &tt.ac}}))
			content, err := os.ReadFile(filepath.Join(dir, "site", "200-proxy.conf"))
			must.NoError(t, err)
			check.Contains(t, string(content), tt.conf)
			got := v.Proxies()
			must.Len(t, got, 1)
			check.DeepEqual(t, got[0].AccessControl, tt.want)
		})
	}
}

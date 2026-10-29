package nginx

import (
	"path/filepath"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 三个子测试按顺序切换同一个 vhost，不能并行
func TestProxyCacheToggle(t *testing.T) {
	v, dir := newProxyVhost(t)
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
			v, err = NewProxyVhost(dir)
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
	v, dir := newProxyVhost(t)
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

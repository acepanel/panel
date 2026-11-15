package webserver

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

func TestWithIPv6Listens(t *testing.T) {
	address := func(listen types.Listen, _ int) string { return listen.Address }

	got := WithIPv6Listens([]types.Listen{
		{Address: "80", Args: []string{"default_server"}},
		{Address: "0.0.0.0:8080"},
		{Address: "127.0.0.1:9000"},
		{Address: "[::1]:9001"},
		{Address: "[::]:80"},
	})
	check.DeepEqual(t, lo.Map(got, address), []string{"80", "0.0.0.0:8080", "127.0.0.1:9000", "[::1]:9001", "[::]:80", "[::]:8080"})

	// 补出来的监听沿用原监听的参数，已经写了的 [::] 不被覆盖
	added := WithIPv6Listens([]types.Listen{{Address: "443", Args: []string{"ssl"}}})
	check.DeepEqual(t, added[1], types.Listen{Address: "[::]:443", Args: []string{"ssl"}})
}

func TestMergeHTTPSListensDefault(t *testing.T) {
	d, err := Get(TypeNginx)
	must.NoError(t, err)
	for _, tt := range []struct {
		name      string
		isDefault bool
		ipv6      bool
		existing  bool
	}{
		{name: "普通站点"},
		{name: "IPv4 默认站点", isDefault: true},
		{name: "IPv6 默认站点", isDefault: true, ipv6: true},
		{name: "已有 HTTPS 监听的默认站点", isDefault: true, ipv6: true, existing: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			listens := []types.Listen{{Address: "80", Args: []string{"backlog=511"}}}
			if tt.isDefault {
				listens[0].Args = append(listens[0].Args, "default_server")
			}
			if tt.existing {
				listens = append(listens, types.Listen{Address: "443", Args: []string{"ssl", "quic"}})
			}
			original := lo.Map(listens, func(l types.Listen, _ int) types.Listen {
				l.Args = slices.Clone(l.Args)
				return l
			})
			merged := d.MergeHTTPSListens(listens, tt.ipv6)
			check.DeepEqual(t, listens, original)

			addresses := []string{"443"}
			if tt.ipv6 {
				addresses = append(addresses, "[::]:443")
			}
			must.Len(t, merged, len(addresses)+1)
			for _, address := range addresses {
				i := slices.IndexFunc(merged, func(l types.Listen) bool { return l.Address == address })
				must.NotEqual(t, i, -1, must.Msgf("缺少监听 %s", address))
				args := merged[i].Args
				check.Equal(t, slices.Contains(args, "default_server"), tt.isDefault)
				check.SliceContains(t, args, "ssl")
				check.SliceContains(t, args, "quic")
				check.SliceNotContains(t, args, "backlog=511")
			}
		})
	}
}

func TestMergeHTTPSListensDefaultLifecycle(t *testing.T) {
	d, err := Get(TypeNginx)
	must.NoError(t, err)
	configDir := t.TempDir()
	for _, name := range []string{"site", "shared"} {
		must.NoError(t, os.Mkdir(filepath.Join(configDir, name), 0755))
	}
	vhost, err := d.NewStaticVhost(configDir)
	must.NoError(t, err)
	must.NoError(t, vhost.SetListen([]types.Listen{{Address: "80"}}))
	must.NoError(t, vhost.SetDefault(true))

	for _, phase := range []string{"首次开启", "关闭后重开"} {
		if phase == "关闭后重开" {
			listens := slices.DeleteFunc(vhost.Listen(), func(l types.Listen) bool {
				return slices.Contains(l.Args, "ssl") || slices.Contains(l.Args, "quic")
			})
			must.NoError(t, vhost.SetListen(listens))
		}
		must.NoError(t, vhost.SetListen(d.MergeHTTPSListens(vhost.Listen(), true)))
		must.NoError(t, vhost.Save())
		vhost, err = d.NewStaticVhost(configDir)
		must.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(configDir, "nginx.conf"))
		must.NoError(t, err)
		for _, listen := range []string{"80", "443 ssl", "443 quic", "[::]:443 ssl", "[::]:443 quic"} {
			check.Contains(t, string(data), "listen "+listen+" default_server;", check.Msgf("%s后的配置：\n%s", phase, data))
		}
	}
}

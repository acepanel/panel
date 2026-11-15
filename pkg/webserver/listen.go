package webserver

import (
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

func WithIPv6Listens(listens []types.Listen) []types.Listen {
	ipv6 := lo.FilterMap(listens, func(listen types.Listen, _ int) (types.Listen, bool) {
		port := listen.Address
		if strings.Contains(listen.Address, ":") {
			host, parsedPort, err := net.SplitHostPort(listen.Address)
			// 写了具体地址的是有意只听那个地址，补 [::] 会把它放开到全部 IPv6 地址
			if err != nil || (host != "0.0.0.0" && host != "*") {
				return types.Listen{}, false
			}
			port = parsedPort
		}
		value, err := strconv.ParseUint(port, 10, 16)
		if err != nil || value == 0 {
			return types.Listen{}, false
		}
		return types.Listen{Address: "[::]:" + port, Args: slices.Clone(listen.Args)}, true
	})

	return lo.UniqBy(slices.Concat(listens, ipv6), func(listen types.Listen) string {
		return listen.Address
	})
}

// MergeHTTPSListens 补充 443 监听，已有的 443 地址补齐 SSL 参数
func (d Dialect) MergeHTTPSListens(listens []types.Listen, listenIPv6 bool) []types.Listen {
	args := d.HTTPSListenArgs()
	// 否则 HTTPS 的默认站点仍是内置站点
	if slices.ContainsFunc(listens, func(listen types.Listen) bool { return slices.Contains(listen.Args, "default_server") }) {
		args = append(args, "default_server")
	}
	addresses := []string{"443"}
	if d.Features().IPv6Listen && listenIPv6 {
		addresses = append(addresses, "[::]:443")
	}
	https := lo.Map(addresses, func(address string, _ int) types.Listen {
		return types.Listen{Address: address, Args: slices.Clone(args)}
	})

	return lo.UniqBy(lo.Map(slices.Concat(listens, https), func(listen types.Listen, _ int) types.Listen {
		if slices.Contains(addresses, listen.Address) {
			listen.Args = lo.Uniq(slices.Concat(listen.Args, args))
		}
		return listen
	}), func(listen types.Listen) string {
		return listen.Address
	})
}

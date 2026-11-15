// Package clientip 提取请求的客户端 IP 并做 IP/CIDR 名单匹配
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// FromRequest 取客户端 IP：header 非空且请求带有效地址时取其中第一个，否则取连接对端地址
func FromRequest(r *http.Request, header string) string {
	if header != "" {
		first, _, _ := strings.Cut(r.Header.Get(header), ",")
		if ip, ok := parse(first); ok {
			return ip
		}
	}

	ip, _ := parse(r.RemoteAddr)
	return ip
}

// Match 判断 ip 是否命中 list 中的任一 IP 或 CIDR
func Match(ip string, list []string) bool {
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}

	return slices.ContainsFunc(list, func(item string) bool {
		if _, ipNet, err := net.ParseCIDR(item); err == nil {
			return ipNet.Contains(addr)
		}
		return addr.Equal(net.ParseIP(item))
	})
}

// parse 解析可能带端口的地址，代理头里的地址通常不带端口而 RemoteAddr 带
func parse(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}

	addr, err := netip.ParseAddr(s)
	if err != nil {
		return s, false
	}
	return addr.String(), true
}

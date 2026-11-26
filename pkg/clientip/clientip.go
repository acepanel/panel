// Package clientip 提取请求的客户端 IP 并做 IP/CIDR 名单匹配
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// FromRequest 取客户端 IP：只采信受信代理转发的 header，沿转发链从右向左跳过受信代理，取第一个非受信地址
func FromRequest(r *http.Request, header string, trusted []string) string {
	ip, _ := parse(r.RemoteAddr)
	if header == "" {
		return ip
	}

	chain := strings.Split(strings.Join(r.Header.Values(header), ","), ",")
	for i := len(chain) - 1; i >= 0 && isTrusted(ip, trusted); i-- {
		hop, ok := parse(chain[i])
		if !ok {
			break
		}
		ip = hop
	}

	return ip
}

// Bucket 返回按来源计数用的键：IPv6 按 /64 归并，同一来源可以随意更换 /64 内的地址
func Bucket(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.Unmap().Is4() {
		return ip
	}

	prefix, _ := addr.Prefix(64)
	return prefix.String()
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

// isTrusted 判断 ip 是否为受信代理，回环与内网地址始终受信
func isTrusted(ip string, trusted []string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate() || Match(ip, trusted)
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

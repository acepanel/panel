package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/libtnb/assert/check"
)

func TestFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		header     string
		value      string
		trusted    []string
		want       string
	}{
		{name: "不带头取对端地址", remoteAddr: "192.0.2.1:54321", want: "192.0.2.1"},
		{name: "未配置头时忽略请求头", remoteAddr: "192.0.2.1:54321", value: "203.0.113.7", want: "192.0.2.1"},
		{name: "头里的地址不带端口", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", value: "203.0.113.7", want: "203.0.113.7"},
		{name: "头里的地址带端口", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", value: "203.0.113.7:443", want: "203.0.113.7"},
		{name: "配置了头但请求未带", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", want: "127.0.0.1"},
		{name: "头里不是地址时回落对端地址", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", value: "unknown", want: "127.0.0.1"},
		{name: "IPv6 对端地址", remoteAddr: "[2001:db8::1]:54321", want: "2001:db8::1"},
		{name: "头里的 IPv6 不带端口", remoteAddr: "[::1]:54321", header: "X-Real-IP", value: "2001:DB8::1", want: "2001:db8::1"},
		{name: "头里的 IPv6 带端口", remoteAddr: "[::1]:54321", header: "X-Real-IP", value: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "对端不受信时忽略请求头", remoteAddr: "198.51.100.9:54321", header: "X-Forwarded-For", value: "10.0.0.1", want: "198.51.100.9"},
		{name: "代理追加真实地址时忽略伪造的前缀", remoteAddr: "127.0.0.1:54321", header: "X-Forwarded-For", value: "10.0.0.1, 198.51.100.9", want: "198.51.100.9"},
		{name: "跳过内网代理", remoteAddr: "127.0.0.1:54321", header: "X-Forwarded-For", value: " 203.0.113.7 , 10.0.0.1", want: "203.0.113.7"},
		{name: "转发链全部受信时取最左", remoteAddr: "127.0.0.1:54321", header: "X-Forwarded-For", value: "192.168.1.5, 10.0.0.1", want: "192.168.1.5"},
		{name: "未列为受信的 CDN 节点即客户端", remoteAddr: "127.0.0.1:54321", header: "X-Forwarded-For", value: "203.0.113.7, 198.51.100.9", want: "198.51.100.9"},
		{name: "跳过受信的 CDN 节点", remoteAddr: "127.0.0.1:54321", header: "X-Forwarded-For", value: "203.0.113.7, 198.51.100.9", trusted: []string{"198.51.100.0/24"}, want: "203.0.113.7"},
		{name: "对端是受信的公网代理", remoteAddr: "198.51.100.9:54321", header: "X-Real-IP", value: "203.0.113.7", trusted: []string{"198.51.100.9"}, want: "203.0.113.7"},
		{name: "链中的非法项截断转发链", remoteAddr: "127.0.0.1:54321", header: "X-Forwarded-For", value: "203.0.113.7, unknown, 10.0.0.1", want: "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.value != "" {
				r.Header.Set("X-Real-IP", tt.value)
				r.Header.Set("X-Forwarded-For", tt.value)
			}
			check.Equal(t, FromRequest(r, tt.header, tt.trusted), tt.want)
		})
	}
}

func TestFromRequestMultipleHeaderLines(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Add("X-Forwarded-For", "10.0.0.1")
	r.Header.Add("X-Forwarded-For", "198.51.100.9")
	check.Equal(t, FromRequest(r, "X-Forwarded-For", nil), "198.51.100.9")
}

func TestBucket(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{name: "IPv4 原样返回", ip: "203.0.113.7", want: "203.0.113.7"},
		{name: "IPv4 映射地址原样返回", ip: "::ffff:203.0.113.7", want: "::ffff:203.0.113.7"},
		{name: "IPv6 按 /64 归并", ip: "2001:db8:1:2:3:4:5:6", want: "2001:db8:1:2::/64"},
		{name: "非法地址原样返回", ip: "unknown", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.Equal(t, Bucket(tt.ip), tt.want)
		})
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		list []string
		want bool
	}{
		{name: "IP 命中", ip: "203.0.113.7", list: []string{"198.51.100.1", "203.0.113.7"}, want: true},
		{name: "IP 未命中", ip: "203.0.113.8", list: []string{"203.0.113.7"}},
		{name: "CIDR 命中", ip: "203.0.113.7", list: []string{"10.0.0.0/8", "203.0.113.0/24"}, want: true},
		{name: "CIDR 未对齐网络号也命中", ip: "203.0.113.7", list: []string{"203.0.113.200/24"}, want: true},
		{name: "CIDR 未命中", ip: "203.0.114.7", list: []string{"203.0.113.0/24"}},
		{name: "IPv6 命中", ip: "2001:db8::1", list: []string{"2001:DB8:0:0::1"}, want: true},
		{name: "IPv6 CIDR 命中", ip: "2001:db8::1", list: []string{"2001:db8::/32"}, want: true},
		{name: "IPv6 CIDR 未命中", ip: "2001:db9::1", list: []string{"2001:db8::/32"}},
		{name: "IPv4 映射地址按 IPv4 匹配", ip: "::ffff:203.0.113.7", list: []string{"203.0.113.0/24"}, want: true},
		{name: "地址族不同不命中", ip: "203.0.113.7", list: []string{"::/0"}},
		{name: "带端口的地址不命中", ip: "203.0.113.7:443", list: []string{"203.0.113.7"}},
		{name: "名单里的非法条目不命中", ip: "203.0.113.7", list: []string{"", "localhost", "203.0.113.7/33"}},
		{name: "空名单不命中", ip: "203.0.113.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.Equal(t, Match(tt.ip, tt.list), tt.want)
		})
	}
}

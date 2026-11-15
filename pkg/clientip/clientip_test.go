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
		want       string
	}{
		{name: "不带头取对端地址", remoteAddr: "192.0.2.1:54321", want: "192.0.2.1"},
		{name: "未配置头时忽略请求头", remoteAddr: "192.0.2.1:54321", value: "203.0.113.7", want: "192.0.2.1"},
		{name: "头里的地址不带端口", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", value: "203.0.113.7", want: "203.0.113.7"},
		{name: "头里的地址带端口", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", value: "203.0.113.7:443", want: "203.0.113.7"},
		{name: "多级代理取第一个", remoteAddr: "127.0.0.1:54321", header: "X-Forwarded-For", value: " 203.0.113.7 , 10.0.0.1", want: "203.0.113.7"},
		{name: "配置了头但请求未带", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", want: "127.0.0.1"},
		{name: "头里不是地址时回落对端地址", remoteAddr: "127.0.0.1:54321", header: "X-Real-IP", value: "unknown", want: "127.0.0.1"},
		{name: "IPv6 对端地址", remoteAddr: "[2001:db8::1]:54321", want: "2001:db8::1"},
		{name: "头里的 IPv6 不带端口", remoteAddr: "[::1]:54321", header: "X-Real-IP", value: "2001:DB8::1", want: "2001:db8::1"},
		{name: "头里的 IPv6 带端口", remoteAddr: "[::1]:54321", header: "X-Real-IP", value: "[2001:db8::1]:443", want: "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.value != "" {
				r.Header.Set("X-Real-IP", tt.value)
				r.Header.Set("X-Forwarded-For", tt.value)
			}
			check.Equal(t, FromRequest(r, tt.header), tt.want)
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

package types

import (
	"regexp"
	"testing"

	"github.com/libtnb/assert/check"
)

func TestDefaultProxyHost(t *testing.T) {
	upstreams := []Upstream{{Name: "example.com_upstream_1"}}
	for _, tt := range []struct {
		pass string
		want string
	}{
		{"http://127.0.0.1:8080", "$host"},
		{"https://[::1]:8443", "$host"},
		{"http://localhost:3000", "$host"},
		{"http://unix:/run/app.sock", "$host"},
		{"http://backend", "$host"},
		// 站点名带点时上游名也带点，只能对照上游列表
		{"http://example.com_upstream_1", "$host"},
		{"http://Example.com_Upstream_1/api", "$host"},
		{"https://api.example.com", "$proxy_host"},
		{"http://backend.internal:8080/path", "$proxy_host"},
		{"://bad", "$host"},
	} {
		check.Equal(t, DefaultProxyHost(tt.pass, upstreams), tt.want, tt.pass)
	}
}

func TestAccessControlLists(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ac    AccessControlConfig
		allow []string
		deny  []string
	}{
		{"白名单里的 deny all 已隐含", AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"all"}}, []string{"10.0.0.0/8"}, nil},
		{"没有白名单的 deny all 保留", AccessControlConfig{Deny: []string{"1.2.3.4", "ALL"}}, nil, []string{"1.2.3.4", "all"}},
		{"allow all 等于不限制", AccessControlConfig{Allow: []string{"All"}}, nil, nil},
		{"allow all 不算白名单", AccessControlConfig{Allow: []string{"all"}, Deny: []string{"all"}}, nil, []string{"all"}},
		{"普通名单原样", AccessControlConfig{Allow: []string{"10.0.0.0/8"}, Deny: []string{"10.0.0.99"}}, []string{"10.0.0.0/8"}, []string{"10.0.0.99"}},
	} {
		allow, deny := tt.ac.Lists()
		check.DeepEqual(t, allow, tt.allow)
		check.DeepEqual(t, deny, tt.deny)
	}
}

func TestSafeName(t *testing.T) {
	check.Equal(t, SafeName("a-b.c_d9"), "a_2db_2ec__d9")
	check.NotEqual(t, SafeName("a-b"), SafeName("a_b"))
}

func TestPHPVersionFromSocket(t *testing.T) {
	check.Equal(t, PHPVersionFromSocket("fastcgi_pass unix:/tmp/php-cgi-84.sock;"), uint(84))
	check.Equal(t, PHPVersionFromSocket("unix//tmp/php-cgi-83.sock"), uint(83))
	check.Equal(t, PHPVersionFromSocket("proxy:unix:/tmp/php-cgi-74.sock|fcgi://localhost/"), uint(74))
	check.Equal(t, PHPVersionFromSocket("unix:/run/app.sock"), uint(0))
}

func TestProxyOrder(t *testing.T) {
	proxies := []Proxy{
		{Location: "/"}, {Location: "/api"}, {Location: "~* \\.(jpg|png)$"}, {Location: "^~ /static/"},
		{Location: "= /health"}, {Location: "/api/v2"}, {Location: "~ ^/v[0-9]+/"}, {Location: "^~ /s"},
	}
	// 精确、^~ 前缀（长者优先）、正则（书写顺序）、普通前缀（长者优先）
	check.DeepEqual(t, ProxyOrder(proxies), []int{4, 3, 7, 2, 6, 5, 1, 0})
}

func TestUnquoteMeta(t *testing.T) {
	for _, s := range []string{"/old", "/a.b/c+d", "old.example.com", `/x\y$`} {
		check.Equal(t, UnquoteMeta(regexp.QuoteMeta(s)), s)
	}
}

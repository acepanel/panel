package types

import (
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

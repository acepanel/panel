package tools

import (
	"net/netip"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func checkIPv4(t *testing.T, ip string) {
	t.Helper()
	addr, err := netip.ParseAddr(ip)
	must.NoError(t, err, must.Msgf("非法地址 %q", ip))
	check.True(t, addr.Is4(), check.Msgf("应为 IPv4，实际 %q", ip))
}

func TestGetMonitoringInfo(t *testing.T) {
	info := CurrentInfo(nil, nil)
	must.NotNil(t, info.Host)
	check.NotZero(t, info.Host.Hostname)
	check.NotNil(t, info.Mem)
}

func TestGetPublicIPv4(t *testing.T) {
	ip, err := GetPublicIPv4()
	must.NoError(t, err)
	checkIPv4(t, ip)
}

func TestGetPublicIPv6(t *testing.T) {
	ip, err := GetPublicIPv6()
	check.Error(t, err)
	check.Empty(t, ip)
}

func TestTraceIP(t *testing.T) {
	ip, err := traceIP("fl=1\nh=example.com\nip=2001:db8::1\nts=1\n")
	must.NoError(t, err)
	check.Equal(t, ip, "2001:db8::1")

	for _, body := range []string{"", "fl=1\nh=example.com", "ip=unknown"} {
		_, err = traceIP(body)
		check.Error(t, err)
	}
}

func TestGetLocalIPv4(t *testing.T) {
	ip, err := GetLocalIPv4()
	must.NoError(t, err)
	checkIPv4(t, ip)
}

func TestGetLocalIPv6(t *testing.T) {
	ip, err := GetLocalIPv6()
	check.Error(t, err)
	check.Empty(t, ip)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		size float64
		want string
	}{
		{name: "B", size: 1, want: "1.00 B"},
		{name: "KB", size: 1 << 10, want: "1.00 KB"},
		{name: "MB", size: 1 << 20, want: "1.00 MB"},
		{name: "GB", size: 1 << 30, want: "1.00 GB"},
		{name: "TB", size: 1 << 40, want: "1.00 TB"},
		{name: "PB", size: 1 << 50, want: "1.00 PB"},
		{name: "EB", size: 1 << 60, want: "1.00 EB"},
		{name: "ZB", size: 1 << 70, want: "1.00 ZB"},
		{name: "YB", size: 1 << 80, want: "1.00 YB"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			check.Equal(t, FormatBytes(test.size), test.want)
		})
	}
}

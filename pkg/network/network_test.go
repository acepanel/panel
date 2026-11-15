package network

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestValidate(t *testing.T) {
	valid := Config{
		Name: "eth0",
		MTU:  1500,
		IPv4: FamilyConfig{Mode: modeManual, Addresses: []string{"192.0.2.10/24"}, Gateway: "198.51.100.1", DNS: []string{"1.1.1.1"}},
		IPv6: FamilyConfig{Mode: modeDisabled},
	}
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{name: "invalid interface", change: func(config *Config) { config.Name = "../eth0" }},
		{name: "invalid CIDR", change: func(config *Config) { config.IPv4.Addresses = []string{"192.0.2.10"} }},
		{name: "wrong family", change: func(config *Config) { config.IPv4.Gateway = "2001:db8::1" }},
		{name: "duplicate DNS", change: func(config *Config) { config.IPv4.DNS = []string{"1.1.1.1", "1.1.1.1"} }},
		{name: "manual without address", change: func(config *Config) { config.IPv4.Addresses = nil }},
		{name: "both disabled", change: func(config *Config) { config.IPv4 = FamilyConfig{Mode: modeDisabled} }},
		{name: "invalid MTU", change: func(config *Config) { config.MTU = 67 }},
		{name: "IPv4 network address", change: func(config *Config) { config.IPv4.Addresses = []string{"192.0.2.0/24"} }},
		{name: "IPv6 subnet-router anycast", change: func(config *Config) {
			config.IPv6 = FamilyConfig{Mode: modeManual, Addresses: []string{"2001:db8:0:b4::/64"}}
		}},
	}
	must.NoError(t, validate(valid))
	pointToPoint := valid
	pointToPoint.IPv4.Addresses = []string{"192.0.2.0/31", "198.51.100.7/32"}
	pointToPoint.IPv6 = FamilyConfig{Mode: modeManual, Addresses: []string{"2001:db8::/127", "2001:db8:1::/128"}}
	must.NoError(t, validate(pointToPoint))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			test.change(&config)
			check.ErrorIs(t, validate(config), ErrValidation)
		})
	}
}

func TestCheckApplied(t *testing.T) {
	config := Config{
		Name: "eth0",
		IPv4: FamilyConfig{Mode: modeManual, Addresses: []string{"192.0.2.10/24"}},
		IPv6: FamilyConfig{Mode: modeManual, Addresses: []string{"2001:DB8:0:00b4::10/64"}},
	}
	link := func(flags, ipv6 string) string {
		return `[{"ifname":"eth0","flags":[` + flags + `],"addr_info":[` +
			`{"family":"inet","local":"192.0.2.10","prefixlen":24},` +
			`{"family":"inet6","local":"2001:db8:0:b4::10","prefixlen":64` + ipv6 + `}]}]`
	}

	must.NoError(t, checkApplied(config, link(`"BROADCAST","UP","LOWER_UP"`, "")))
	check.ErrorContains(t, checkApplied(config, link(`"BROADCAST"`, "")), "is down")
	check.ErrorContains(t, checkApplied(config, link(`"UP"`, `,"tentative":true`)), "did not pass duplicate address detection")
	check.ErrorIs(t, checkApplied(config, link(`"UP"`, `,"tentative":true,"dadfailed":true`)), ErrValidation)

	missing := config
	missing.IPv6.Addresses = []string{"2001:db8:0:b4::20/64"}
	check.ErrorContains(t, checkApplied(missing, link(`"UP"`, "")), "was not applied")
	check.Error(t, checkApplied(config, `Device "eth0" does not exist.`))
}

func TestIfupdownParse(t *testing.T) {
	backend := &ifupdownBackend{}
	file := backend.parse("/etc/network/interfaces", `auto eth0
iface eth0 inet dhcp
    dns-nameservers 1.1.1.1
    mtu 1500

iface eth0 inet6 static
    address 2001:db8::10/64
`)
	must.Len(t, file.stanzas, 2)
	check.DeepEqual(t, backend.family(file.stanzas, "inet"), FamilyConfig{
		Mode:      modeAuto,
		Addresses: []string{},
		AutoDNS:   true,
		DNS:       []string{"1.1.1.1"},
	})
	check.DeepEqual(t, backend.family(file.stanzas, "inet6"), FamilyConfig{
		Mode:      modeManual,
		Addresses: []string{"2001:db8::10/64"},
		AutoDNS:   true,
		DNS:       []string{},
	})
}

func TestNetworkManagerParse(t *testing.T) {
	// 输出取自 NetworkManager 1.42（Debian 12）与 1.54（AlmaLinux 9），两个版本逐字节一致
	tests := []struct {
		name   string
		output string
		ipv4   FamilyConfig
		ipv6   FamilyConfig
		mtu    int
	}{
		{
			name: "单地址手动",
			output: `ipv4.method:manual
ipv4.addresses:192.0.2.10/24
ipv4.gateway:192.0.2.1
ipv4.dns:1.1.1.1
ipv4.ignore-auto-dns:no
ipv6.method:disabled
ipv6.addresses:
ipv6.gateway:
ipv6.dns:
ipv6.ignore-auto-dns:no
802-3-ethernet.mtu:auto`,
			ipv4: FamilyConfig{Mode: modeManual, Addresses: []string{"192.0.2.10/24"}, Gateway: "192.0.2.1", AutoDNS: true, DNS: []string{"1.1.1.1"}},
			ipv6: emptyFamily(),
		},
		{
			name: "双栈多地址多 DNS",
			output: `ipv4.method:manual
ipv4.addresses:192.0.2.20/24, 192.0.2.21/24, 198.51.100.5/25
ipv4.gateway:192.0.2.1
ipv4.dns:1.1.1.1,8.8.8.8
ipv4.ignore-auto-dns:yes
ipv6.method:manual
ipv6.addresses:2001:db8:0:b4::10/64, 2001:db8:0:b4::11/64
ipv6.gateway:2001:db8:0:b4::1
ipv6.dns:2606:4700:4700::1111,2001:4860:4860::8888
ipv6.ignore-auto-dns:yes
802-3-ethernet.mtu:1400`,
			ipv4: FamilyConfig{
				Mode:      modeManual,
				Addresses: []string{"192.0.2.20/24", "192.0.2.21/24", "198.51.100.5/25"},
				Gateway:   "192.0.2.1",
				DNS:       []string{"1.1.1.1", "8.8.8.8"},
			},
			ipv6: FamilyConfig{
				Mode:      modeManual,
				Addresses: []string{"2001:db8:0:b4::10/64", "2001:db8:0:b4::11/64"},
				Gateway:   "2001:db8:0:b4::1",
				DNS:       []string{"2001:4860:4860::8888", "2606:4700:4700::1111"},
			},
			mtu: 1400,
		},
		{
			name: "全自动",
			output: `ipv4.method:auto
ipv4.addresses:
ipv4.gateway:
ipv4.dns:
ipv4.ignore-auto-dns:no
ipv6.method:auto
ipv6.addresses:
ipv6.gateway:
ipv6.dns:
ipv6.ignore-auto-dns:no
802-3-ethernet.mtu:auto`,
			ipv4: FamilyConfig{Mode: modeAuto, Addresses: []string{}, AutoDNS: true, DNS: []string{}},
			ipv6: FamilyConfig{Mode: modeAuto, Addresses: []string{}, AutoDNS: true, DNS: []string{}},
		},
		{
			name: "手动无网关加自动带静态 DNS",
			output: `ipv4.method:manual
ipv4.addresses:192.0.2.50/24
ipv4.gateway:
ipv4.dns:
ipv4.ignore-auto-dns:no
ipv6.method:auto
ipv6.addresses:
ipv6.gateway:
ipv6.dns:2606:4700:4700::1111
ipv6.ignore-auto-dns:yes
802-3-ethernet.mtu:auto`,
			ipv4: FamilyConfig{Mode: modeManual, Addresses: []string{"192.0.2.50/24"}, AutoDNS: true, DNS: []string{}},
			ipv6: FamilyConfig{Mode: modeAuto, Addresses: []string{}, DNS: []string{"2606:4700:4700::1111"}},
		},
		{
			name: "dhcp 归为自动",
			output: `ipv4.method:manual
ipv4.addresses:198.51.100.7/32
ipv4.gateway:
ipv4.dns:
ipv4.ignore-auto-dns:no
ipv6.method:dhcp
ipv6.addresses:
ipv6.gateway:
ipv6.dns:
ipv6.ignore-auto-dns:no
802-3-ethernet.mtu:auto`,
			ipv4: FamilyConfig{Mode: modeManual, Addresses: []string{"198.51.100.7/32"}, AutoDNS: true, DNS: []string{}},
			ipv6: FamilyConfig{Mode: modeAuto, Addresses: []string{}, AutoDNS: true, DNS: []string{}},
		},
		{
			name: "不认识的方式视为禁用",
			output: `ipv4.method:link-local
ipv4.addresses:
ipv4.gateway:
ipv4.dns:
ipv4.ignore-auto-dns:no
ipv6.method:ignore
ipv6.addresses:
ipv6.gateway:
ipv6.dns:
ipv6.ignore-auto-dns:no
802-3-ethernet.mtu:auto`,
			ipv4: emptyFamily(),
			ipv6: emptyFamily(),
		},
		{
			name: "没有以太网设置段时缺 MTU 行",
			output: `ipv4.method:manual
ipv4.addresses:127.0.0.1/8
ipv4.gateway:
ipv4.dns:
ipv4.ignore-auto-dns:no
ipv6.method:manual
ipv6.addresses:::1/128
ipv6.gateway:
ipv6.dns:
ipv6.ignore-auto-dns:no`,
			ipv4: FamilyConfig{Mode: modeManual, Addresses: []string{"127.0.0.1/8"}, AutoDNS: true, DNS: []string{}},
			ipv6: FamilyConfig{Mode: modeManual, Addresses: []string{"::1/128"}, AutoDNS: true, DNS: []string{}},
		},
	}
	backend := &networkManagerBackend{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ipv4, ipv6, mtu := backend.parse(test.output, "ethernet")
			check.DeepEqual(t, ipv4, test.ipv4)
			check.DeepEqual(t, ipv6, test.ipv6)
			check.Equal(t, mtu, test.mtu)
		})
	}
}

package os

import (
	"net"
	"net/netip"
	"runtime"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestPortSetOverlaps(t *testing.T) {
	ports := PortSet{22, 80, 443, 8080}

	check.True(t, ports.Overlaps(80, 80))
	check.True(t, ports.Overlaps(1, 22))
	check.True(t, ports.Overlaps(100, 500))
	check.True(t, ports.Overlaps(8080, 65535))
	check.False(t, ports.Overlaps(23, 79))
	check.False(t, ports.Overlaps(8081, 65535))
	check.False(t, ports.Overlaps(1, 21))
	check.False(t, PortSet(nil).Overlaps(1, 65535))
}

func TestListeningPorts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("depends on /proc/net")
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	must.NoError(t, err)
	defer func() { _ = listener.Close() }()
	packet, err := lc.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	must.NoError(t, err)
	defer func() { _ = packet.Close() }()
	// 已建立连接的本地端口不是监听端口
	var dialer net.Dialer
	conn, err := dialer.DialContext(t.Context(), "tcp", listener.Addr().String())
	must.NoError(t, err)
	defer func() { _ = conn.Close() }()

	port := func(addr net.Addr) uint {
		return uint(netip.MustParseAddrPort(addr.String()).Port())
	}

	tcp, udp, err := ListeningPorts(t.Context())
	must.NoError(t, err)
	check.True(t, tcp.Overlaps(port(listener.Addr()), port(listener.Addr())))
	check.False(t, tcp.Overlaps(port(conn.LocalAddr()), port(conn.LocalAddr())))
	check.True(t, udp.Overlaps(port(packet.LocalAddr()), port(packet.LocalAddr())))
}

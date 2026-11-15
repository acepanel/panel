package frp

import "github.com/acepanel/panel/v3/internal/apps/confval"

// serverFields 可视化参数与 TOML 键的对应关系，表单一律用字符串收值，Kind 决定写回时的字面量类型
func (s *App) serverFields(t *ServerTune) []confval.Field {
	return []confval.Field{
		{Key: "bindAddr", Value: &t.BindAddr},
		{Key: "bindPort", Value: &t.BindPort, Kind: confval.Int},
		{Key: "kcpBindPort", Value: &t.KCPBindPort, Kind: confval.Int},
		{Key: "quicBindPort", Value: &t.QUICBindPort, Kind: confval.Int},
		{Key: "proxyBindAddr", Value: &t.ProxyBindAddr},
		{Key: "subDomainHost", Value: &t.SubDomainHost},
		{Key: "maxPortsPerClient", Value: &t.MaxPortsPerClient, Kind: confval.Int},
		{Key: "udpPacketSize", Value: &t.UDPPacketSize, Kind: confval.Int},
		{Key: "detailedErrorsToClient", Value: &t.DetailedErrorsToClient, Kind: confval.Bool},

		{Key: "auth.method", Value: &t.AuthMethod},
		{Key: "auth.token", Value: &t.AuthToken},

		{Key: "vhostHTTPPort", Value: &t.VhostHTTPPort, Kind: confval.Int},
		{Key: "vhostHTTPSPort", Value: &t.VhostHTTPSPort, Kind: confval.Int},
		{Key: "vhostHTTPTimeout", Value: &t.VhostHTTPTimeout, Kind: confval.Int},
		{Key: "tcpmuxHTTPConnectPort", Value: &t.TCPMuxHTTPConnectPort, Kind: confval.Int},
		{Key: "custom404Page", Value: &t.Custom404Page},

		{Key: "webServer.addr", Value: &t.WebServerAddr},
		{Key: "webServer.port", Value: &t.WebServerPort, Kind: confval.Int},
		{Key: "webServer.user", Value: &t.WebServerUser},
		{Key: "webServer.password", Value: &t.WebServerPassword},
		{Key: "webServer.pprofEnable", Value: &t.WebServerPprofEnable, Kind: confval.Bool},
		{Key: "enablePrometheus", Value: &t.EnablePrometheus, Kind: confval.Bool},

		{Key: "transport.maxPoolCount", Value: &t.TransportMaxPoolCount, Kind: confval.Int},
		{Key: "transport.tcpMux", Value: &t.TransportTCPMux, Kind: confval.Bool},
		{Key: "transport.tcpMuxKeepaliveInterval", Value: &t.TransportTCPMuxKeepaliveInterval, Kind: confval.Int},
		{Key: "transport.tcpKeepalive", Value: &t.TransportTCPKeepalive, Kind: confval.Int},
		{Key: "transport.heartbeatTimeout", Value: &t.TransportHeartbeatTimeout, Kind: confval.Int},
		{Key: "transport.tls.force", Value: &t.TransportTLSForce, Kind: confval.Bool},
		{Key: "transport.tls.certFile", Value: &t.TransportTLSCertFile},
		{Key: "transport.tls.keyFile", Value: &t.TransportTLSKeyFile},
		{Key: "transport.tls.trustedCaFile", Value: &t.TransportTLSTrustedCaFile},

		{Key: "log.to", Value: &t.LogTo},
		{Key: "log.level", Value: &t.LogLevel},
		{Key: "log.maxDays", Value: &t.LogMaxDays, Kind: confval.Int},
		{Key: "log.disablePrintColor", Value: &t.LogDisablePrintColor, Kind: confval.Bool},
	}
}

func (s *App) clientFields(t *ClientTune) []confval.Field {
	return []confval.Field{
		{Key: "user", Value: &t.User},
		{Key: "serverAddr", Value: &t.ServerAddr},
		{Key: "serverPort", Value: &t.ServerPort, Kind: confval.Int},
		{Key: "loginFailExit", Value: &t.LoginFailExit, Kind: confval.Bool},
		{Key: "natHoleStunServer", Value: &t.NatHoleStunServer},
		{Key: "dnsServer", Value: &t.DNSServer},
		{Key: "udpPacketSize", Value: &t.UDPPacketSize, Kind: confval.Int},

		{Key: "auth.method", Value: &t.AuthMethod},
		{Key: "auth.token", Value: &t.AuthToken},

		{Key: "transport.protocol", Value: &t.TransportProtocol},
		{Key: "transport.poolCount", Value: &t.TransportPoolCount, Kind: confval.Int},
		{Key: "transport.tcpMux", Value: &t.TransportTCPMux, Kind: confval.Bool},
		{Key: "transport.tcpMuxKeepaliveInterval", Value: &t.TransportTCPMuxKeepaliveInterval, Kind: confval.Int},
		{Key: "transport.dialServerTimeout", Value: &t.TransportDialServerTimeout, Kind: confval.Int},
		{Key: "transport.dialServerKeepalive", Value: &t.TransportDialServerKeepalive, Kind: confval.Int},
		{Key: "transport.heartbeatInterval", Value: &t.TransportHeartbeatInterval, Kind: confval.Int},
		{Key: "transport.heartbeatTimeout", Value: &t.TransportHeartbeatTimeout, Kind: confval.Int},
		{Key: "transport.connectServerLocalIP", Value: &t.TransportConnectServerLocalIP},
		{Key: "transport.proxyURL", Value: &t.TransportProxyURL},
		{Key: "transport.tls.enable", Value: &t.TransportTLSEnable, Kind: confval.Bool},
		{Key: "transport.tls.certFile", Value: &t.TransportTLSCertFile},
		{Key: "transport.tls.keyFile", Value: &t.TransportTLSKeyFile},
		{Key: "transport.tls.trustedCaFile", Value: &t.TransportTLSTrustedCaFile},
		{Key: "transport.tls.serverName", Value: &t.TransportTLSServerName},

		{Key: "webServer.addr", Value: &t.WebServerAddr},
		{Key: "webServer.port", Value: &t.WebServerPort, Kind: confval.Int},
		{Key: "webServer.user", Value: &t.WebServerUser},
		{Key: "webServer.password", Value: &t.WebServerPassword},
		{Key: "webServer.pprofEnable", Value: &t.WebServerPprofEnable, Kind: confval.Bool},

		{Key: "log.to", Value: &t.LogTo},
		{Key: "log.level", Value: &t.LogLevel},
		{Key: "log.maxDays", Value: &t.LogMaxDays, Kind: confval.Int},
		{Key: "log.disablePrintColor", Value: &t.LogDisablePrintColor, Kind: confval.Bool},
	}
}

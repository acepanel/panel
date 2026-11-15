<script setup lang="ts">
defineOptions({
  name: 'frps-config-tune',
})

import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'
import type { TuneGroup } from '@/utils/tune'

import { authMethodOptions, logLevelOptions, useBoolOptions } from './options'

const { $gettext } = useGettext()
const boolOptions = useBoolOptions()

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: $gettext('Leave a field empty to comment out the option and use the frps default.'),
    fields: [
      {
        key: 'bind_addr',
        label: $gettext('Bind Address (bindAddr)'),
        type: 'text',
        placeholder: '0.0.0.0',
      },
      {
        key: 'bind_port',
        label: $gettext('Bind Port (bindPort)'),
        type: 'text',
        placeholder: '7000',
      },
      {
        key: 'kcp_bind_port',
        label: $gettext('KCP Bind Port (kcpBindPort)'),
        type: 'text',
        placeholder: $gettext('Leave empty to disable KCP'),
      },
      {
        key: 'quic_bind_port',
        label: $gettext('QUIC Bind Port (quicBindPort)'),
        type: 'text',
        placeholder: $gettext('Leave empty to disable QUIC'),
      },
      {
        key: 'proxy_bind_addr',
        label: $gettext('Proxy Bind Address (proxyBindAddr)'),
        type: 'text',
        placeholder: $gettext('Defaults to bindAddr'),
      },
      {
        key: 'sub_domain_host',
        label: $gettext('Subdomain Host (subDomainHost)'),
        type: 'text',
        placeholder: 'frps.com',
      },
      {
        key: 'max_ports_per_client',
        label: $gettext('Max Ports Per Client (maxPortsPerClient)'),
        type: 'text',
        placeholder: $gettext('0 means no limit'),
      },
      {
        key: 'udp_packet_size',
        label: $gettext('UDP Packet Size (udpPacketSize)'),
        type: 'text',
        placeholder: '1500',
      },
      {
        key: 'detailed_errors_to_client',
        label: $gettext('Detailed Errors To Client (detailedErrorsToClient)'),
        type: 'select',
        options: boolOptions,
      },
    ],
  },
  {
    name: 'auth',
    tab: $gettext('Authentication'),
    tip: $gettext('The token must be identical on frps and frpc, otherwise login will fail.'),
    tipType: 'warning',
    fields: [
      {
        key: 'auth_method',
        label: $gettext('Method (auth.method)'),
        type: 'select',
        options: authMethodOptions,
      },
      {
        key: 'auth_token',
        label: $gettext('Token (auth.token)'),
        type: 'text',
        placeholder: $gettext('Leave empty for no authentication'),
        password: true,
      },
    ],
  },
  {
    name: 'vhost',
    tab: $gettext('HTTP(S)'),
    tip: $gettext('Required for http and https type proxies.'),
    fields: [
      {
        key: 'vhost_http_port',
        label: $gettext('HTTP Port (vhostHTTPPort)'),
        type: 'text',
        placeholder: '80',
      },
      {
        key: 'vhost_https_port',
        label: $gettext('HTTPS Port (vhostHTTPSPort)'),
        type: 'text',
        placeholder: '443',
      },
      {
        key: 'vhost_http_timeout',
        label: $gettext('HTTP Response Timeout (vhostHTTPTimeout)'),
        type: 'text',
        placeholder: '60',
      },
      {
        key: 'tcpmux_http_connect_port',
        label: $gettext('TCPMux HTTP Connect Port (tcpmuxHTTPConnectPort)'),
        type: 'text',
        placeholder: $gettext('Required for tcpmux type proxies'),
      },
      {
        key: 'custom_404_page',
        label: $gettext('Custom 404 Page (custom404Page)'),
        type: 'text',
        placeholder: '/path/to/404.html',
      },
    ],
  },
  {
    name: 'dashboard',
    tab: $gettext('Dashboard'),
    tip: $gettext('The frps dashboard is available only when the port is set.'),
    fields: [
      {
        key: 'web_server_addr',
        label: $gettext('Listen Address (webServer.addr)'),
        type: 'text',
        placeholder: '127.0.0.1',
      },
      {
        key: 'web_server_port',
        label: $gettext('Listen Port (webServer.port)'),
        type: 'text',
        placeholder: '7500',
      },
      {
        key: 'web_server_user',
        label: $gettext('Username (webServer.user)'),
        type: 'text',
        placeholder: 'admin',
      },
      {
        key: 'web_server_password',
        label: $gettext('Password (webServer.password)'),
        type: 'text',
        password: true,
      },
      {
        key: 'web_server_pprof_enable',
        label: $gettext('Enable pprof (webServer.pprofEnable)'),
        type: 'select',
        options: boolOptions,
      },
      {
        key: 'enable_prometheus',
        label: $gettext('Enable Prometheus (enablePrometheus)'),
        type: 'select',
        options: boolOptions,
      },
    ],
  },
  {
    name: 'transport',
    tab: $gettext('Transport'),
    tip: $gettext('Do not modify these options unless you know what they do.'),
    tipType: 'warning',
    fields: [
      {
        key: 'transport_max_pool_count',
        label: $gettext('Max Pool Count (transport.maxPoolCount)'),
        type: 'text',
        placeholder: '5',
      },
      {
        key: 'transport_tcp_mux',
        label: $gettext('TCP Mux (transport.tcpMux)'),
        type: 'select',
        options: boolOptions,
      },
      {
        key: 'transport_tcp_mux_keepalive_interval',
        label: $gettext('TCP Mux Keepalive Interval (transport.tcpMuxKeepaliveInterval)'),
        type: 'text',
        placeholder: '30',
      },
      {
        key: 'transport_tcp_keepalive',
        label: $gettext('TCP Keepalive (transport.tcpKeepalive)'),
        type: 'text',
        placeholder: '7200',
      },
      {
        key: 'transport_heartbeat_timeout',
        label: $gettext('Heartbeat Timeout (transport.heartbeatTimeout)'),
        type: 'text',
        placeholder: '90',
      },
      {
        key: 'transport_tls_force',
        label: $gettext('Force TLS (transport.tls.force)'),
        type: 'select',
        options: boolOptions,
      },
      {
        key: 'transport_tls_cert_file',
        label: $gettext('TLS Certificate (transport.tls.certFile)'),
        type: 'text',
        placeholder: 'server.crt',
      },
      {
        key: 'transport_tls_key_file',
        label: $gettext('TLS Private Key (transport.tls.keyFile)'),
        type: 'text',
        placeholder: 'server.key',
      },
      {
        key: 'transport_tls_trusted_ca_file',
        label: $gettext('TLS CA (transport.tls.trustedCaFile)'),
        type: 'text',
        placeholder: 'ca.crt',
      },
    ],
  },
  {
    name: 'log',
    tab: $gettext('Logs'),
    fields: [
      {
        key: 'log_to',
        label: $gettext('Output (log.to)'),
        type: 'text',
        placeholder: './frps.log',
      },
      {
        key: 'log_level',
        label: $gettext('Level (log.level)'),
        type: 'select',
        options: logLevelOptions,
      },
      {
        key: 'log_max_days',
        label: $gettext('Max Days (log.maxDays)'),
        type: 'text',
        placeholder: '3',
      },
      {
        key: 'log_disable_print_color',
        label: $gettext('Disable Colors (log.disablePrintColor)'),
        type: 'select',
        options: boolOptions,
      },
    ],
  },
]
</script>

<template>
  <config-tune-form :fetch="frp.server" :save="frp.saveServer" :groups="groups" />
</template>

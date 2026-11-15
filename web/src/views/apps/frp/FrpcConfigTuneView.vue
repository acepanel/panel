<script setup lang="ts">
defineOptions({
  name: 'frpc-config-tune',
})

import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'
import { toOptions, type TuneGroup } from '@/utils/tune'

import { authMethodOptions, logLevelOptions, useBoolOptions } from './options'

const { $gettext } = useGettext()
const boolOptions = useBoolOptions()

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: $gettext('Leave a field empty to comment out the option and use the frpc default.'),
    fields: [
      {
        key: 'server_addr',
        label: $gettext('Server Address (serverAddr)'),
        type: 'text',
        placeholder: '0.0.0.0',
      },
      {
        key: 'server_port',
        label: $gettext('Server Port (serverPort)'),
        type: 'text',
        placeholder: '7000',
      },
      {
        key: 'user',
        label: $gettext('User (user)'),
        type: 'text',
        placeholder: $gettext('Proxy names will be prefixed with this user'),
      },
      {
        key: 'login_fail_exit',
        label: $gettext('Exit On Login Failure (loginFailExit)'),
        type: 'select',
        options: boolOptions,
      },
      {
        key: 'nat_hole_stun_server',
        label: $gettext('STUN Server (natHoleStunServer)'),
        type: 'text',
        placeholder: 'stun.easyvoip.com:3478',
      },
      {
        key: 'dns_server',
        label: $gettext('DNS Server (dnsServer)'),
        type: 'text',
        placeholder: '8.8.8.8',
      },
      {
        key: 'udp_packet_size',
        label: $gettext('UDP Packet Size (udpPacketSize)'),
        type: 'text',
        placeholder: '1500',
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
    name: 'transport',
    tab: $gettext('Transport'),
    tip: $gettext('Do not modify these options unless you know what they do.'),
    tipType: 'warning',
    fields: [
      {
        key: 'transport_protocol',
        label: $gettext('Protocol (transport.protocol)'),
        type: 'select',
        options: toOptions(['tcp', 'kcp', 'quic', 'websocket', 'wss']),
      },
      {
        key: 'transport_pool_count',
        label: $gettext('Pool Count (transport.poolCount)'),
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
        key: 'transport_dial_server_timeout',
        label: $gettext('Dial Timeout (transport.dialServerTimeout)'),
        type: 'text',
        placeholder: '10',
      },
      {
        key: 'transport_dial_server_keepalive',
        label: $gettext('Dial Keepalive (transport.dialServerKeepalive)'),
        type: 'text',
        placeholder: '7200',
      },
      {
        key: 'transport_heartbeat_interval',
        label: $gettext('Heartbeat Interval (transport.heartbeatInterval)'),
        type: 'text',
        placeholder: '30',
      },
      {
        key: 'transport_heartbeat_timeout',
        label: $gettext('Heartbeat Timeout (transport.heartbeatTimeout)'),
        type: 'text',
        placeholder: '90',
      },
      {
        key: 'transport_connect_server_local_ip',
        label: $gettext('Local IP (transport.connectServerLocalIP)'),
        type: 'text',
        placeholder: '0.0.0.0',
      },
      {
        key: 'transport_proxy_url',
        label: $gettext('Proxy URL (transport.proxyURL)'),
        type: 'text',
        placeholder: 'socks5://user:passwd@192.168.1.128:1080',
      },
      {
        key: 'transport_tls_enable',
        label: $gettext('Enable TLS (transport.tls.enable)'),
        type: 'select',
        options: boolOptions,
      },
      {
        key: 'transport_tls_cert_file',
        label: $gettext('TLS Certificate (transport.tls.certFile)'),
        type: 'text',
        placeholder: 'client.crt',
      },
      {
        key: 'transport_tls_key_file',
        label: $gettext('TLS Private Key (transport.tls.keyFile)'),
        type: 'text',
        placeholder: 'client.key',
      },
      {
        key: 'transport_tls_trusted_ca_file',
        label: $gettext('TLS CA (transport.tls.trustedCaFile)'),
        type: 'text',
        placeholder: 'ca.crt',
      },
      {
        key: 'transport_tls_server_name',
        label: $gettext('TLS Server Name (transport.tls.serverName)'),
        type: 'text',
        placeholder: 'example.com',
      },
    ],
  },
  {
    name: 'admin',
    tab: $gettext('Admin API'),
    tip: $gettext('The frpc admin API is available only when the port is set.'),
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
        placeholder: '7400',
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
        placeholder: './frpc.log',
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
  <config-tune-form :fetch="frp.client" :save="frp.saveClient" :groups="groups" />
</template>

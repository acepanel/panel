import { http } from '@/utils'

export default {
  // DNS
  dns: (): any => http.Get('/toolbox_system/dns'),
  // 设置 DNS
  updateDns: (dns1: string, dns2: string): any => http.Post('/toolbox_system/dns', { dns1, dns2 }),
  // SWAP
  swap: (): any => http.Get('/toolbox_system/swap'),
  // 设置 SWAP
  updateSwap: (size: number): any => http.Post('/toolbox_system/swap', { size }),
  // 时区
  timezone: (): any => http.Get('/toolbox_system/timezone'),
  // 设置时区
  updateTimezone: (timezone: string): any => http.Post('/toolbox_system/timezone', { timezone }),
  // 设置时间
  updateTime: (time: string): any => http.Post('/toolbox_system/time', { time }),
  // 同步时间（可选指定 NTP 服务器）
  syncTime: (server?: string): any => http.Post('/toolbox_system/sync_time', { server }),
  // 获取 NTP 服务器配置
  ntpServers: (): any => http.Get('/toolbox_system/ntp_servers'),
  // 设置 NTP 服务器配置
  updateNtpServers: (servers: string[]): any =>
    http.Post('/toolbox_system/ntp_servers', { servers }),
  // 主机名
  hostname: (): any => http.Get('/toolbox_system/hostname'),
  // Hosts
  hosts: (): any => http.Get('/toolbox_system/hosts'),
  // 设置主机名
  updateHostname: (hostname: string): any => http.Post('/toolbox_system/hostname', { hostname }),
  // 设置 Hosts
  updateHosts: (hosts: string): any => http.Post('/toolbox_system/hosts', { hosts }),
  // 设置 Root 密码
  updateRootPassword: (password: string): any =>
    http.Post('/toolbox_system/root_password', { password })
}

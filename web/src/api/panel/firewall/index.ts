import { http } from '@/utils'

export default {
  // 获取防火墙状态
  status: (): any => http.Get('/firewall/status'),
  // 设置防火墙状态
  updateStatus: (status: boolean): any => http.Post('/firewall/status', { status }),
  // 获取防火墙规则
  rules: (page: number, limit: number): any =>
    http.Get('/firewall/rule', { params: { page, limit } }),
  // 创建防火墙规则
  createRule: (rule: any): any => http.Post('/firewall/rule', rule),
  // 删除防火墙规则
  deleteRule: (rule: any): any => http.Delete('/firewall/rule', rule),
  // 获取防火墙IP规则
  ipRules: (page: number, limit: number): any =>
    http.Get('/firewall/ip_rule', { params: { page, limit } }),
  // 创建防火墙IP规则
  createIpRule: (rule: any): any => http.Post('/firewall/ip_rule', rule),
  // 删除防火墙IP规则
  deleteIpRule: (rule: any): any => http.Delete('/firewall/ip_rule', rule),
  // 获取防火墙转发规则
  forwards: (page: number, limit: number): any =>
    http.Get('/firewall/forward', { params: { page, limit } }),
  // 创建防火墙转发规则
  createForward: (rule: any): any => http.Post('/firewall/forward', rule),
  // 删除防火墙转发规则
  deleteForward: (rule: any): any => http.Delete('/firewall/forward', rule),
  // 获取端口占用进程信息
  portUsage: (port: number, protocol: string): any =>
    http.Get('/firewall/rule/port_usage', { params: { port, protocol } }),
  // 扫描感知 - 获取设置
  scanSetting: (): any => http.Get('/firewall/scan/setting'),
  // 扫描感知 - 更新设置
  updateScanSetting: (setting: any): any => http.Post('/firewall/scan/setting', setting),
  // 扫描感知 - 获取可用网卡
  scanInterfaces: (): any => http.Get('/firewall/scan/interfaces'),
  // 扫描感知 - 获取汇总
  scanSummary: (start: string, end: string): any =>
    http.Get('/firewall/scan/summary', { params: { start, end } }),
  // 扫描感知 - 获取趋势
  scanTrend: (start: string, end: string): any =>
    http.Get('/firewall/scan/trend', { params: { start, end } }),
  // 扫描感知 - 获取 Top 源 IP
  scanTopIPs: (start: string, end: string, limit: number): any =>
    http.Get('/firewall/scan/top_ips', { params: { start, end, limit } }),
  // 扫描感知 - 获取 Top 端口
  scanTopPorts: (start: string, end: string, limit: number): any =>
    http.Get('/firewall/scan/top_ports', { params: { start, end, limit } }),
  // 扫描感知 - 获取事件列表
  scanEvents: (
    start: string,
    end: string,
    page: number,
    limit: number,
    sourceIP?: string,
    port?: number,
    location?: string,
  ): any =>
    http.Get('/firewall/scan/events', {
      params: {
        start,
        end,
        page,
        limit,
        source_ip: sourceIP,
        port: port || undefined,
        location: location || undefined,
      },
    }),
  // 扫描感知 - 清空数据
  scanClear: (): any => http.Post('/firewall/scan/clear'),
}

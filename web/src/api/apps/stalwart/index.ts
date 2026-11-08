import { http } from '@/utils'

export default {
  // 状态概览
  status: (): any => http.Get('/apps/stalwart/status'),
  // 初始化
  init: (hostname: string, domain: string): any =>
    http.Post('/apps/stalwart/init', { hostname, domain }),
  // 自检
  check: (): any => http.Get('/apps/stalwart/check'),
  // 管理员凭据
  admin: (): any => http.Get('/apps/stalwart/admin'),
  // 一键创建或同步反代站点
  proxy: (cert_id: number): any => http.Post('/apps/stalwart/proxy', { cert_id }),

  // 域名列表
  domains: (page: number, limit: number): any =>
    http.Get('/apps/stalwart/domains', { params: { page, limit } }),
  // 添加域名
  domainCreate: (name: string): any => http.Post('/apps/stalwart/domains', { name }),
  // 删除域名
  domainDelete: (id: string): any => http.Delete(`/apps/stalwart/domains/${id}`),
  // 设置兜底地址
  domainCatchAll: (id: string, address: string): any =>
    http.Post(`/apps/stalwart/domains/${id}/catch_all`, { address }),
  // DNS 记录与状态
  domainDNS: (id: string): any => http.Get(`/apps/stalwart/domains/${id}/dns`),
  // 一键添加解析
  domainDNSPublish: (id: string, dns_id: number, records: string[]): any =>
    http.Post(`/apps/stalwart/domains/${id}/dns/publish`, { dns_id, records }),

  // 账户列表
  accounts: (page: number, limit: number, domain_id: string, keyword: string): any =>
    http.Get('/apps/stalwart/accounts', { params: { page, limit, domain_id, keyword } }),
  // 添加账户
  accountCreate: (data: any): any => http.Post('/apps/stalwart/accounts', data),
  // 修改账户
  accountUpdate: (id: string, data: any): any => http.Put(`/apps/stalwart/accounts/${id}`, data),
  // 修改密码
  accountPassword: (id: string, password: string): any =>
    http.Post(`/apps/stalwart/accounts/${id}/password`, { password }),
  // 删除账户
  accountDelete: (id: string): any => http.Delete(`/apps/stalwart/accounts/${id}`),

  // 队列列表
  queue: (page: number, limit: number, keyword: string): any =>
    http.Get('/apps/stalwart/queue', { params: { page, limit, keyword } }),
  // 立即重试
  queueRetry: (id: string): any => http.Post(`/apps/stalwart/queue/${id}/retry`, {}),
  // 删除队列邮件
  queueDelete: (id: string): any => http.Delete(`/apps/stalwart/queue/${id}`),

  // 设置
  settings: (): any => http.Get('/apps/stalwart/settings'),
  // 保存主机名
  saveHostname: (hostname: string): any =>
    http.Post('/apps/stalwart/settings/hostname', { hostname }),
  // 绑定证书
  saveCert: (cert_id: number): any => http.Post('/apps/stalwart/settings/cert', { cert_id }),
  // 保存监听端口
  saveListeners: (data: any): any => http.Post('/apps/stalwart/settings/listeners', data),
  // 保存出站中继
  saveRelay: (data: any): any => http.Post('/apps/stalwart/settings/relay', data),
}

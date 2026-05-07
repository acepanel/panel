import { http } from '@/utils'

export default {
  // 保护列表
  jails: (page: number, limit: number): any =>
    http.Get('/apps/fail2ban/jails', { params: { page, limit } }),
  // 添加保护
  add: (data: any): any => http.Post('/apps/fail2ban/jails', data),
  // 删除保护
  delete: (name: string): any => http.Delete('/apps/fail2ban/jails', { name }),
  // 封禁列表
  jail: (name: string): any => http.Get('/apps/fail2ban/jails/' + name),
  // 解封 IP
  unban: (name: string, ip: string): any => http.Post('/apps/fail2ban/unban', { name, ip }),
  // 获取白名单
  whitelist: (): any => http.Get('/apps/fail2ban/white_list'),
  // 设置白名单
  setWhitelist: (ip: string): any => http.Post('/apps/fail2ban/white_list', { ip }),
}

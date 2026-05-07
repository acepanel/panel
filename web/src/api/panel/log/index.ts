import { http } from '@/utils'

export default {
  // 获取日志列表
  list: (type: 'app' | 'db' | 'http', limit: number = 100, date: string = ''): any =>
    http.Get('/log/list', { params: { type, limit, date } }),
  // 获取日志日期列表
  dates: (type: 'app' | 'db' | 'http'): any => http.Get('/log/dates', { params: { type } }),
  // 获取 SSH 登录日志
  ssh: (limit: number = 100): any => http.Get('/log/ssh', { params: { limit } }),
}

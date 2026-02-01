import { http } from '@/utils'

export default {
  // 获取日志列表
  list: (type: 'app' | 'db' | 'http', limit: number = 100, date: string = ''): any =>
    http.Get('/log/list', { params: { type, limit, date } }),

  // 获取日志日期列表
  dates: (type: 'app' | 'db' | 'http'): any => http.Get('/log/dates', { params: { type } })
}

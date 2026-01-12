import { http } from '@/utils'

export default {
  // 获取日志列表
  list: (type: 'app' | 'db' | 'http', limit: number = 100): any =>
    http.Get('/log/list', { params: { type, limit } })
}

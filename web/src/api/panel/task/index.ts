import { http } from '@/utils'

export default {
  // 获取状态
  status: (): any => http.Get('/task/status', { meta: { noAlert: true } }),
  // 获取任务列表
  list: (page: number, limit: number): any => http.Get('/task', { params: { page, limit } }),
  // 获取任务
  get: (id: number): any => http.Get(`/task/${id}`),
  // 删除任务
  delete: (id: number): any => http.Delete(`/task/${id}`),
}

import { http } from '@/utils'

export default {
  // 获取状态
  status: (): any => http.Get('/task/status', { meta: { noAlert: true } }),
  // 获取任务列表
  list: (page: number, limit: number): any => http.Get('/task', { params: { page, limit } }),
  // 批量获取任务，已删除的不在结果里
  query: (ids: number[]): any => http.Post('/task/query', { ids }, { meta: { noAlert: true } }),
  // 删除任务
  delete: (id: number): any => http.Delete(`/task/${id}`),
  // 取消任务
  cancel: (id: number): any => http.Post(`/task/${id}/cancel`),
}

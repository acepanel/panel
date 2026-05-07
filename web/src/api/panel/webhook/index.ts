import { http } from '@/utils'

export default {
  // 获取 WebHook 列表
  list: (page: number, limit: number): any => http.Get('/webhook', { params: { page, limit } }),
  // 获取 WebHook 信息
  get: (id: number): any => http.Get(`/webhook/${id}`),
  // 创建 WebHook
  create: (req: any): any => http.Post('/webhook', req),
  // 修改 WebHook
  update: (id: number, req: any): any => http.Put(`/webhook/${id}`, req),
  // 删除 WebHook
  delete: (id: number): any => http.Delete(`/webhook/${id}`),
}

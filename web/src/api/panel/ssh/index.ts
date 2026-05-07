import { http } from '@/utils'

export default {
  // 获取主机列表
  list: (page: number, limit: number): any => http.Get('/ssh', { params: { page, limit } }),
  // 获取主机信息
  get: (id: number): any => http.Get(`/ssh/${id}`),
  // 创建主机
  create: (req: any): any => http.Post('/ssh', req),
  // 修改主机
  update: (id: number, req: any): any => http.Put(`/ssh/${id}`, req),
  // 删除主机
  delete: (id: number): any => http.Delete(`/ssh/${id}`),
}

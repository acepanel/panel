import { http } from '@/utils'

export default {
  // 获取备份账号列表
  list: (page: number, limit: number): any =>
    http.Get('/backup_storage', { params: { page, limit } }),
  // 获取备份账号
  get: (id: number): any => http.Get(`/backup_storage/${id}`),
  // 创建备份账号
  create: (data: any): any => http.Post('/backup_storage', data),
  // 更新备份账号
  update: (id: number, data: any): any => http.Put(`/backup_storage/${id}`, data),
  // 删除备份账号
  delete: (id: number): any => http.Delete(`/backup_storage/${id}`),
}

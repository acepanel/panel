import { http } from '@/utils'

export default {
  // 获取备份列表
  list: (type: string, page: number, limit: number): any =>
    http.Get(`/backup/${type}`, { params: { page, limit } }),
  // 创建备份
  create: (type: string, target: string, storage: number): any =>
    http.Post(`/backup/${type}`, { target, storage }),
  // 上传备份
  upload: (type: string, formData: FormData): any => http.Post(`/backup/${type}/upload`, formData),
  // 删除备份
  delete: (type: string, file: string): any => http.Delete(`/backup/${type}/delete`, { file }),
  // 恢复备份
  restore: (type: string, file: string, target: string): any =>
    http.Post(`/backup/${type}/restore`, { file, target }),
}

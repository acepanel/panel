import { http } from '@/utils'

export default {
  // 列表
  mounts: (page: number, limit: number): any =>
    http.Get('/apps/s3fs/mounts', { params: { page, limit } }),
  // 添加
  add: (data: any): any => http.Post('/apps/s3fs/mounts', data),
  // 删除
  delete: (id: number): any => http.Delete('/apps/s3fs/mounts', { id }),
}

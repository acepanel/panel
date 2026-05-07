import { http } from '@/utils'

export default {
  // 获取项目列表
  list: (type: string, page: number, limit: number): any =>
    http.Get('/project', { params: { type, page, limit } }),
  // 获取项目详情
  get: (id: number): any => http.Get(`/project/${id}`),
  // 创建项目
  create: (data: any): any => http.Post('/project', data),
  // 更新项目
  update: (id: number, data: any): any => http.Put(`/project/${id}`, data),
  // 删除项目
  delete: (id: number): any => http.Delete(`/project/${id}`),
}

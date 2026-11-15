import { http } from '@/utils'

export default {
  // 获取模版列表
  list: (page: number, pageSize: number, category?: string, query?: string): any =>
    http.Get(`/template`, { params: { page, limit: pageSize, category, query } }),
  // 使用模版创建编排
  create: (data: {
    slug: string
    name: string
    compose: string
    envs: { key: string; value: string }[]
  }): any => http.Post('/template', data),
}

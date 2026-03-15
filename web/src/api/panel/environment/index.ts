import { http } from '@/utils'

export default {
  // 获取环境类型列表
  types: (): any => http.Get('/environment/types'),
  // 获取环境列表
  list: (page: number, limit: number, type?: string, query?: string): any =>
    http.Get('/environment/list', { params: { page, limit, type, query } }),
  // 安装环境
  install: (type: string, slug: string): any => http.Post('/environment/install', { type, slug }),
  // 卸载环境
  uninstall: (type: string, slug: string): any =>
    http.Post('/environment/uninstall', { type, slug }),
  // 更新环境
  update: (type: string, slug: string): any => http.Post('/environment/update', { type, slug })
}

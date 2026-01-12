import { http } from '@/utils'

export default {
  // 获取分类列表
  categories: (): any => http.Get('/app/categories'),
  // 获取应用列表
  list: (page: number, limit: number, category?: string): any =>
    http.Get('/app/list', { params: { page, limit, category } }),
  // 安装应用
  install: (slug: string, channel: string | null): any =>
    http.Post('/app/install', { slug, channel }),
  // 卸载应用
  uninstall: (slug: string): any => http.Post('/app/uninstall', { slug }),
  // 更新应用
  update: (slug: string): any => http.Post('/app/update', { slug }),
  // 设置首页显示
  updateShow: (slug: string, show: boolean): any => http.Post('/app/update_show', { slug, show }),
  // 更新首页显示排序
  updateOrder: (slugs: string[]): any => http.Post('/app/update_order', { slugs }),
  // 应用是否已安装
  isInstalled: (slugs: string): any => http.Get('/app/is_installed', { params: { slugs } }),
  // 更新缓存
  updateCache: (): any => http.Get('/app/update_cache')
}

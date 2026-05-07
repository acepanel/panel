import { http } from '@/utils'

export default {
  // 设为 CLI 版本
  setCli: (slug: string): any => http.Post(`/environment/go/${slug}/set_cli`),
  // 获取代理
  getProxy: (slug: string): any => http.Get(`/environment/go/${slug}/proxy`),
  // 设置代理
  setProxy: (slug: string, proxy: string): any =>
    http.Post(`/environment/go/${slug}/proxy`, { proxy }),
}

import { http } from '@/utils'

export default {
  // 设为 CLI 版本
  setCli: (slug: string): any => http.Post(`/environment/nodejs/${slug}/set_cli`),
  // 获取镜像
  getRegistry: (slug: string): any => http.Get(`/environment/nodejs/${slug}/registry`),
  // 设置镜像
  setRegistry: (slug: string, registry: string): any =>
    http.Post(`/environment/nodejs/${slug}/registry`, { registry }),
}

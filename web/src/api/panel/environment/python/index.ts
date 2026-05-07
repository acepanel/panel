import { http } from '@/utils'

export default {
  // 设为 CLI 版本
  setCli: (slug: string): any => http.Post(`/environment/python/${slug}/set_cli`),
  // 获取镜像
  getMirror: (slug: string): any => http.Get(`/environment/python/${slug}/mirror`),
  // 设置镜像
  setMirror: (slug: string, mirror: string): any =>
    http.Post(`/environment/python/${slug}/mirror`, { mirror }),
}

import { http } from '@/utils'

export default {
  // 获取配置
  config: (name: string): any => http.Get('/apps/frp/config', { params: { name } }),
  // 保存配置
  saveConfig: (name: string, config: string): any => http.Post('/apps/frp/config', { name, config }),
  // 获取运行用户
  user: (name: string): any => http.Get('/apps/frp/user', { params: { name } }),
  // 设置运行用户
  saveUser: (name: string, user: string, group: string): any =>
    http.Post('/apps/frp/user', { name, user, group })
}

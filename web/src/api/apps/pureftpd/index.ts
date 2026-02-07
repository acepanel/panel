import { http } from '@/utils'

export default {
  // 列表
  list: (page: number, limit: number): any =>
    http.Get('/apps/pureftpd/users', { params: { page, limit } }),
  // 添加
  add: (username: string, password: string, path: string): any =>
    http.Post('/apps/pureftpd/users', { username, password, path }),
  // 删除
  delete: (username: string): any => http.Delete(`/apps/pureftpd/users/${username}`),
  // 修改密码
  changePassword: (username: string, password: string): any =>
    http.Post(`/apps/pureftpd/users/${username}/password`, { password }),
  // 获取端口
  port: (): any => http.Get('/apps/pureftpd/port'),
  // 修改端口
  updatePort: (port: number): any => http.Post('/apps/pureftpd/port', { port }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/pureftpd/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/pureftpd/config_tune', data)
}

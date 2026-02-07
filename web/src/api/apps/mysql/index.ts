import { http } from '@/utils'

export default {
  // 负载状态
  load: (): any => http.Get('/apps/mysql/load'),
  // 获取配置
  config: (): any => http.Get('/apps/mysql/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/mysql/config', { config }),
  // 清空日志
  clearLog: (): any => http.Post('/apps/mysql/clear_log'),
  // 获取慢查询日志
  slowLog: (): any => http.Get('/apps/mysql/slow_log'),
  // 清空慢查询日志
  clearSlowLog: (): any => http.Post('/apps/mysql/clear_slow_log'),
  // 获取 root 密码
  rootPassword: (): any => http.Get('/apps/mysql/root_password'),
  // 修改 root 密码
  setRootPassword: (password: string): any => http.Post('/apps/mysql/root_password', { password }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/mysql/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/mysql/config_tune', data)
}

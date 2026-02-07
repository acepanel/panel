import { http } from '@/utils'

export default {
  // 负载状态
  load: (): any => http.Get('/apps/postgresql/load'),
  // 获取配置
  config: (): any => http.Get('/apps/postgresql/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/postgresql/config', { config }),
  // 获取用户配置
  userConfig: (): any => http.Get('/apps/postgresql/user_config'),
  // 保存配置
  saveUserConfig: (config: string): any => http.Post('/apps/postgresql/user_config', { config }),
  // 获取日志
  log: (): any => http.Get('/apps/postgresql/log'),
  // 清空错误日志
  clearLog: (): any => http.Post('/apps/postgresql/clear_log'),
  // 获取 postgres 密码
  postgresPassword: (): any => http.Get('/apps/postgresql/postgres_password'),
  // 设置 postgres 密码
  setPostgresPassword: (password: string): any =>
    http.Post('/apps/postgresql/postgres_password', { password }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/postgresql/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/postgresql/config_tune', data)
}

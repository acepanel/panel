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
  // 获取 postgres 密码
  postgresPassword: (): any => http.Get('/apps/postgresql/postgres_password'),
  // 设置 postgres 密码
  setPostgresPassword: (password: string): any =>
    http.Post('/apps/postgresql/postgres_password', { password }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/postgresql/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/postgresql/config_tune', data),
  // 扩展管理
  extensions: (): any => http.Get('/apps/postgresql/extensions'),
  installExtension: (slug: string): any => http.Post('/apps/postgresql/extensions', { slug }),
  uninstallExtension: (slug: string): any => http.Delete('/apps/postgresql/extensions', { slug }),
  enableExtension: (slug: string, database: string): any =>
    http.Post('/apps/postgresql/extensions/enable', { slug, database }),
  // 性能
  sessions: (): any => http.Get('/apps/postgresql/sessions'),
  terminateSession: (pid: number): any => http.Post(`/apps/postgresql/sessions/${pid}/terminate`),
  topSQL: (params: any): any => http.Get('/apps/postgresql/top_sql', { params }),
  enableTopSQL: (): any => http.Post('/apps/postgresql/top_sql/enable'),
  resetTopSQL: (): any => http.Post('/apps/postgresql/top_sql/reset'),
  // 维护
  databases: (): any => http.Get('/apps/postgresql/databases'),
  bloat: (params: any): any => http.Get('/apps/postgresql/bloat', { params }),
  runMaintenance: (data: any): any => http.Post('/apps/postgresql/maintenance', data),
  wal: (): any => http.Get('/apps/postgresql/wal'),
  dropReplicationSlot: (slot: string): any =>
    http.Delete(`/apps/postgresql/replication_slots/${slot}`),
}

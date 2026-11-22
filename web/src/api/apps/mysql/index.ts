import { http } from '@/utils'

export const createMysqlApi = (slug: string) => ({
  // 负载状态
  load: (): any => http.Get(`/apps/${slug}/load`),
  // 获取配置
  config: (): any => http.Get(`/apps/${slug}/config`),
  // 保存配置
  saveConfig: (config: string): any => http.Post(`/apps/${slug}/config`, { config }),
  // 获取慢查询日志
  slowLog: (): any => http.Get(`/apps/${slug}/slow_log`),
  // 获取 root 密码
  rootPassword: (): any => http.Get(`/apps/${slug}/root_password`),
  // 修改 root 密码
  setRootPassword: (password: string): any =>
    http.Post(`/apps/${slug}/root_password`, { password }),
  // 获取配置调整参数
  configTune: (): any => http.Get(`/apps/${slug}/config_tune`),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post(`/apps/${slug}/config_tune`, data),
  // 性能
  processes: (): any => http.Get(`/apps/${slug}/processes`),
  killProcess: (id: number): any => http.Post(`/apps/${slug}/processes/${id}/kill`),
  transactions: (): any => http.Get(`/apps/${slug}/transactions`),
  topSQL: (params: any): any => http.Get(`/apps/${slug}/top_sql`, { params }),
  enableTopSQL: (): any => http.Post(`/apps/${slug}/top_sql/enable`),
  resetTopSQL: (): any => http.Post(`/apps/${slug}/top_sql/reset`),
  // 维护
  databases: (): any => http.Get(`/apps/${slug}/databases`),
  tables: (params: any): any => http.Get(`/apps/${slug}/tables`, { params }),
  runMaintenance: (data: any): any => http.Post(`/apps/${slug}/maintenance`, data),
  binlogs: (): any => http.Get(`/apps/${slug}/binlogs`),
  purgeBinlog: (file: string): any => http.Post(`/apps/${slug}/binlogs/purge`, { file }),
  replication: (): any => http.Get(`/apps/${slug}/replication`),
})

export default createMysqlApi('mysql')

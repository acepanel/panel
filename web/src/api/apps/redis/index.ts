import { http } from '@/utils'

export const createRedisApi = (slug: string) => ({
  // 负载状态
  load: (): any => http.Get(`/apps/${slug}/load`),
  // 获取配置
  config: (): any => http.Get(`/apps/${slug}/config`),
  // 保存配置
  saveConfig: (config: string): any => http.Post(`/apps/${slug}/config`, { config }),
  // 获取配置调整参数
  configTune: (): any => http.Get(`/apps/${slug}/config_tune`),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post(`/apps/${slug}/config_tune`, data),
  // 慢日志
  slowLog: (): any => http.Get(`/apps/${slug}/slow_log`),
  resetSlowLog: (): any => http.Post(`/apps/${slug}/slow_log/reset`),
  // 客户端连接
  clients: (): any => http.Get(`/apps/${slug}/clients`),
  killClient: (id: number): any => http.Post(`/apps/${slug}/clients/kill`, { id }),
  // 内存诊断
  memory: (): any => http.Get(`/apps/${slug}/memory`),
  // 扫描大 Key
  scanBigKeys: (): any => http.Post(`/apps/${slug}/bigkeys`),
})

export default createRedisApi('redis')

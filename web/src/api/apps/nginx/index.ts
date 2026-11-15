import { http } from '@/utils'

export const createNginxApi = (slug: string) => ({
  // 负载状态
  load: (): any => http.Get(`/apps/${slug}/load`),
  // 获取配置
  config: (): any => http.Get(`/apps/${slug}/config`),
  // 保存配置
  saveConfig: (config: string): any => http.Post(`/apps/${slug}/config`, { config }),
  // 获取错误日志
  errorLog: (): any => http.Get(`/apps/${slug}/error_log`),
  // 清空错误日志
  clearErrorLog: (): any => http.Post(`/apps/${slug}/clear_error_log`),
  // 获取配置调整参数
  configTune: (): any => http.Get(`/apps/${slug}/config_tune`),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post(`/apps/${slug}/config_tune`, data),

  // Stream Server 接口
  stream: {
    listServers: (): any => http.Get(`/apps/${slug}/stream/servers`),
    createServer: (data: any): any => http.Post(`/apps/${slug}/stream/servers`, data),
    updateServer: (name: string, data: any): any =>
      http.Put(`/apps/${slug}/stream/servers/${name}`, data),
    deleteServer: (name: string): any => http.Delete(`/apps/${slug}/stream/servers/${name}`),
    listUpstreams: (): any => http.Get(`/apps/${slug}/stream/upstreams`),
    createUpstream: (data: any): any => http.Post(`/apps/${slug}/stream/upstreams`, data),
    updateUpstream: (name: string, data: any): any =>
      http.Put(`/apps/${slug}/stream/upstreams/${name}`, data),
    deleteUpstream: (name: string): any => http.Delete(`/apps/${slug}/stream/upstreams/${name}`),
  },
})

export default createNginxApi('nginx')

import { http } from '@/utils'

export default {
  // 负载状态
  load: (): any => http.Get('/apps/openresty/load'),
  // 获取配置
  config: (): any => http.Get('/apps/openresty/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/openresty/config', { config }),
  // 获取错误日志
  errorLog: (): any => http.Get('/apps/openresty/error_log'),
  // 清空错误日志
  clearErrorLog: (): any => http.Post('/apps/openresty/clear_error_log'),

  // Stream Server 接口
  stream: {
    // Server 相关
    listServers: (): any => http.Get('/apps/openresty/stream/servers'),
    createServer: (data: any): any => http.Post('/apps/openresty/stream/servers', data),
    getServer: (name: string): any => http.Get(`/apps/openresty/stream/servers/${name}`),
    updateServer: (name: string, data: any): any => http.Put(`/apps/openresty/stream/servers/${name}`, data),
    deleteServer: (name: string): any => http.Delete(`/apps/openresty/stream/servers/${name}`),

    // Upstream 相关
    listUpstreams: (): any => http.Get('/apps/openresty/stream/upstreams'),
    createUpstream: (data: any): any => http.Post('/apps/openresty/stream/upstreams', data),
    getUpstream: (name: string): any => http.Get(`/apps/openresty/stream/upstreams/${name}`),
    updateUpstream: (name: string, data: any): any => http.Put(`/apps/openresty/stream/upstreams/${name}`, data),
    deleteUpstream: (name: string): any => http.Delete(`/apps/openresty/stream/upstreams/${name}`)
  }
}

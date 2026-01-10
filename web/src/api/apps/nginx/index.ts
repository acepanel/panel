import { http } from '@/utils'

export default {
  // 负载状态
  load: (): any => http.Get('/apps/nginx/load'),
  // 获取配置
  config: (): any => http.Get('/apps/nginx/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/nginx/config', { config }),
  // 获取错误日志
  errorLog: (): any => http.Get('/apps/nginx/error_log'),
  // 清空错误日志
  clearErrorLog: (): any => http.Post('/apps/nginx/clear_error_log'),

  // Stream Server 接口
  stream: {
    listServers: (): any => http.Get('/apps/nginx/stream/servers'),
    createServer: (data: any): any => http.Post('/apps/nginx/stream/servers', data),
    updateServer: (name: string, data: any): any =>
      http.Put(`/apps/nginx/stream/servers/${name}`, data),
    deleteServer: (name: string): any => http.Delete(`/apps/nginx/stream/servers/${name}`),
    listUpstreams: (): any => http.Get('/apps/nginx/stream/upstreams'),
    createUpstream: (data: any): any => http.Post('/apps/nginx/stream/upstreams', data),
    updateUpstream: (name: string, data: any): any =>
      http.Put(`/apps/nginx/stream/upstreams/${name}`, data),
    deleteUpstream: (name: string): any => http.Delete(`/apps/nginx/stream/upstreams/${name}`)
  }
}

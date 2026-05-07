import { http } from '@/utils'

export default {
  // 负载状态
  load: (): any => http.Get('/apps/grafana/load'),
  // 获取配置
  config: (): any => http.Get('/apps/grafana/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/grafana/config', { config }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/grafana/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/grafana/config_tune', data),
  // 数据源管理
  datasources: (): any => http.Get('/apps/grafana/datasources'),
  createDatasource: (data: any): any => http.Post('/apps/grafana/datasources', data),
  updateDatasource: (name: string, data: any): any =>
    http.Post(`/apps/grafana/datasources/${encodeURIComponent(name)}`, data),
  deleteDatasource: (name: string): any =>
    http.Delete(`/apps/grafana/datasources/${encodeURIComponent(name)}`),
}

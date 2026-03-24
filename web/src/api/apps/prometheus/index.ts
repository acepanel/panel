import { http } from '@/utils'

export default {
  // 负载状态
  load: (): any => http.Get('/apps/prometheus/load'),
  // 获取配置
  config: (): any => http.Get('/apps/prometheus/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/prometheus/config', { config }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/prometheus/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/prometheus/config_tune', data),
  // Exporters 管理
  exporters: (): any => http.Get('/apps/prometheus/exporters'),
  installExporter: (slug: string): any => http.Post('/apps/prometheus/exporters', { slug }),
  uninstallExporter: (slug: string): any => http.Delete('/apps/prometheus/exporters', { slug }),
  startExporter: (slug: string): any => http.Post(`/apps/prometheus/exporters/${slug}/start`),
  stopExporter: (slug: string): any => http.Post(`/apps/prometheus/exporters/${slug}/stop`),
  restartExporter: (slug: string): any => http.Post(`/apps/prometheus/exporters/${slug}/restart`),
  exporterConfig: (slug: string): any => http.Get(`/apps/prometheus/exporters/${slug}/config`),
  saveExporterConfig: (slug: string, config: string): any =>
    http.Post(`/apps/prometheus/exporters/${slug}/config`, { config })
}

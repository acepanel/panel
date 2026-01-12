import { http } from '@/utils'

export default {
  // 设为 CLI 版本
  setCli: (slug: number): any => http.Post(`/environment/php/${slug}/set_cli`),
  // 获取 phpinfo
  phpinfo: (slug: number): any => http.Get(`/environment/php/${slug}/phpinfo`),
  // 获取配置
  config: (slug: number): any => http.Get(`/environment/php/${slug}/config`),
  // 保存配置
  saveConfig: (slug: number, config: string): any =>
    http.Post(`/environment/php/${slug}/config`, { config }),
  // 获取FPM配置
  fpmConfig: (slug: number): any => http.Get(`/environment/php/${slug}/fpm_config`),
  // 保存FPM配置
  saveFPMConfig: (slug: number, config: string): any =>
    http.Post(`/environment/php/${slug}/fpm_config`, { config }),
  // 负载状态
  load: (slug: number): any => http.Get(`/environment/php/${slug}/load`),
  // 获取日志
  log: (slug: number): any => http.Get(`/environment/php/${slug}/log`),
  // 清空日志
  clearLog: (slug: number): any => http.Post(`/environment/php/${slug}/clear_log`),
  // 获取慢日志
  slowLog: (slug: number): any => http.Get(`/environment/php/${slug}/slow_log`),
  // 清空慢日志
  clearSlowLog: (slug: number): any => http.Post(`/environment/php/${slug}/clear_slow_log`),
  // 拓展列表
  modules: (slug: number): any => http.Get(`/environment/php/${slug}/modules`),
  // 安装拓展
  installModule: (slug: number, module: string): any =>
    http.Post(`/environment/php/${slug}/modules`, { slug: module }),
  // 卸载拓展
  uninstallModule: (slug: number, module: string): any =>
    http.Delete(`/environment/php/${slug}/modules`, { slug: module })
}

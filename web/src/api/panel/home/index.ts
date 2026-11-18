import { http } from '@/utils'

export default {
  // 面板信息
  panel: (): any => http.Get('/home/panel'),
  // 首页应用
  apps: (): any => http.Get('/home/apps'),
  // 实时信息
  current: (nets: string[], disks: string[]): any =>
    http.Post('/home/current', { nets, disks }, { meta: { noAlert: true } }),
  // 系统信息
  systemInfo: (): any => http.Get('/home/system_info'),
  // 统计信息
  countInfo: (): any => http.Get('/home/count_info'),
  // 已安装的环境
  installedEnvironment: (): any => http.Get('/home/installed_environment'),
  // 更新信息：是否有新版本及要展示的更新日志
  updateInfo: (): any => http.Get('/home/update_info', { meta: { noAlert: true } }),
  // 重启面板
  restart: (): any => http.Post('/home/restart'),
  // 重启服务器
  restartServer: (): any => http.Post('/home/restart_server'),
  // Top 进程
  topProcesses: (type: string): any => http.Get('/home/top_processes', { params: { type } }),
  // 运行时信息
  runtimeInfo: (): any => http.Get('/home/runtime_info'),
  // Goroutine 列表
  goroutines: (): any => http.Get('/home/goroutines'),
  // 面板健康问题列表（全局横幅使用）
  health: (): any => http.Get('/home/health', { meta: { noAlert: true } }),
}

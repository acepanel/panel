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
  // 检查更新
  checkUpdate: (): any => http.Get('/home/check_update'),
  // 更新日志
  updateInfo: (): any => http.Get('/home/update_info'),
  // 更新面板
  update: (): any => http.Post('/home/update'),
  // 重启面板
  restart: (): any => http.Post('/home/restart'),
  // 重启服务器
  restartServer: (): any => http.Post('/home/restart_server')
}

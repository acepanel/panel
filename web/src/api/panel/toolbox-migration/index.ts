import { http } from '@/utils'

export default {
  // 获取迁移状态
  status: (): any => http.Get('/toolbox_migration/status'),
  // 预检查远程环境
  precheck: (data: any): any => http.Post('/toolbox_migration/precheck', data),
  // 获取可迁移项
  items: (): any => http.Get('/toolbox_migration/items'),
  // 开始迁移
  start: (data: any): any => http.Post('/toolbox_migration/start', data),
  // 重置迁移
  reset: (): any => http.Post('/toolbox_migration/reset'),
  // 获取迁移结果
  results: (): any => http.Get('/toolbox_migration/results'),
  // 下载迁移日志
  logUrl: '/api/toolbox_migration/log'
}

import { http } from '@/utils'

export default {
  // 开关
  setting: (): any => http.Get('/monitor/setting'),
  // 保存设置
  updateSetting: (enabled: boolean, days: number, interval: number): any =>
    http.Post('/monitor/setting', { enabled, days, interval }),
  // 清空监控记录
  clear: (): any => http.Post('/monitor/clear'),
  // 监控记录
  list: (start: number, end: number): any => http.Get('/monitor/list', { params: { start, end } }),
}

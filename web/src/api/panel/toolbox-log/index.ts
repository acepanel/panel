import { http } from '@/utils'

export default {
  // 扫描日志
  scan: (type: string): any => http.Get('/toolbox_log/scan', { type }),
  // 清理日志
  clean: (type: string): any => http.Post('/toolbox_log/clean', { type })
}

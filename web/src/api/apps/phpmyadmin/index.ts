import { http } from '@/utils'

export default {
  // 获取信息
  info: (): any => http.Get('/apps/phpmyadmin/info'),
  // 设置端口
  port: (port: number): any => http.Post('/apps/phpmyadmin/port', { port }),
  // 获取配置
  config: (): any => http.Get('/apps/phpmyadmin/config'),
  // 保存配置
  updateConfig: (config: string): any => http.Post('/apps/phpmyadmin/config', { config }),
}

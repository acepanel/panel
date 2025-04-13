import { http } from '@/utils'

export default {
  // 获取配置
  config: (): any => http.Get('/apps/codeserver/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/codeserver/config', { config })
}

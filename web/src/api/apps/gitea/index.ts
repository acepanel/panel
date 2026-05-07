import { http } from '@/utils'

export default {
  // 获取配置
  config: (): any => http.Get('/apps/gitea/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/gitea/config', { config }),
}

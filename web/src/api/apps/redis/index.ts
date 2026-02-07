import { http } from '@/utils'

export default {
  // 负载状态
  load: (): any => http.Get('/apps/redis/load'),
  // 获取配置
  config: (): any => http.Get('/apps/redis/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/redis/config', { config }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/redis/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/redis/config_tune', data)
}

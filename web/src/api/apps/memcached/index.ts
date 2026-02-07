import { http } from '@/utils'

export default {
  load: (): any => http.Get('/apps/memcached/load'),
  config: (): any => http.Get('/apps/memcached/config'),
  updateConfig: (config: string): any => http.Post('/apps/memcached/config', { config }),
  // 获取配置调整参数
  configTune: (): any => http.Get('/apps/memcached/config_tune'),
  // 保存配置调整参数
  saveConfigTune: (data: any): any => http.Post('/apps/memcached/config_tune', data)
}

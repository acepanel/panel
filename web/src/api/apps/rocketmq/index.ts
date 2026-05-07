import { http } from '@/utils'

export default {
  load: (): any => http.Get('/apps/rocketmq/load'),
  config: (): any => http.Get('/apps/rocketmq/config'),
  saveConfig: (config: string): any => http.Post('/apps/rocketmq/config', { config }),
  configTune: (): any => http.Get('/apps/rocketmq/config_tune'),
  saveConfigTune: (data: any): any => http.Post('/apps/rocketmq/config_tune', data),
}

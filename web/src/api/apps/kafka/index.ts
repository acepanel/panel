import { http } from '@/utils'

export default {
  load: (): any => http.Get('/apps/kafka/load'),
  config: (): any => http.Get('/apps/kafka/config'),
  saveConfig: (config: string): any => http.Post('/apps/kafka/config', { config }),
  configTune: (): any => http.Get('/apps/kafka/config_tune'),
  saveConfigTune: (data: any): any => http.Post('/apps/kafka/config_tune', data),
}

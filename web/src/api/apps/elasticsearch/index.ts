import { http } from '@/utils'

export default {
  load: (): any => http.Get('/apps/elasticsearch/load'),
  config: (): any => http.Get('/apps/elasticsearch/config'),
  saveConfig: (config: string): any => http.Post('/apps/elasticsearch/config', { config }),
  configTune: (): any => http.Get('/apps/elasticsearch/config_tune'),
  saveConfigTune: (data: any): any => http.Post('/apps/elasticsearch/config_tune', data),
}

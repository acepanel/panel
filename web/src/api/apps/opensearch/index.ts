import { http } from '@/utils'

export default {
  load: (): any => http.Get('/apps/opensearch/load'),
  config: (): any => http.Get('/apps/opensearch/config'),
  saveConfig: (config: string): any => http.Post('/apps/opensearch/config', { config }),
  configTune: (): any => http.Get('/apps/opensearch/config_tune'),
  saveConfigTune: (data: any): any => http.Post('/apps/opensearch/config_tune', data),
}

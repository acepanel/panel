import { http } from '@/utils'

export default {
  load: (): any => http.Get('/apps/valkey/load'),
  config: (): any => http.Get('/apps/valkey/config'),
  saveConfig: (config: string): any => http.Post('/apps/valkey/config', { config }),
  configTune: (): any => http.Get('/apps/valkey/config_tune'),
  saveConfigTune: (data: any): any => http.Post('/apps/valkey/config_tune', data),
}

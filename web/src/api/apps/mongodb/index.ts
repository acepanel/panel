import { http } from '@/utils/http'

export default {
  load: (): any => http.Get('/apps/mongodb/load'),
  config: (): any => http.Get('/apps/mongodb/config'),
  saveConfig: (config: string): any => http.Post('/apps/mongodb/config', { config }),
  configTune: (): any => http.Get('/apps/mongodb/config_tune'),
  saveConfigTune: (data: any): any => http.Post('/apps/mongodb/config_tune', data),
  adminPassword: (): any => http.Get('/apps/mongodb/admin_password'),
  setAdminPassword: (password: string): any =>
    http.Post('/apps/mongodb/admin_password', { password }),
}

import { http } from '@/utils'

export default {
  config: (): any => http.Get('/apps/docker/config'),
  updateConfig: (config: string): any => http.Post('/apps/docker/config', { config }),
  settings: (): any => http.Get('/apps/docker/settings'),
  updateSettings: (settings: any): any => http.Post('/apps/docker/settings', { settings }),
}

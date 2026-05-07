import { http } from '@/utils/http'

export default {
  load: (): any => http.Get('/apps/clickhouse/load'),
  config: (): any => http.Get('/apps/clickhouse/config'),
  saveConfig: (config: string): any => http.Post('/apps/clickhouse/config', { config }),
  configTune: (): any => http.Get('/apps/clickhouse/config_tune'),
  saveConfigTune: (data: any): any => http.Post('/apps/clickhouse/config_tune', data),
  defaultPassword: (): any => http.Get('/apps/clickhouse/default_password'),
  setDefaultPassword: (password: string): any =>
    http.Post('/apps/clickhouse/default_password', { password }),
}

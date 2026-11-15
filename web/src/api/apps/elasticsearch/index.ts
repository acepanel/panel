import { http } from '@/utils'

export const createElasticsearchApi = (slug: string) => ({
  load: (): any => http.Get(`/apps/${slug}/load`),
  config: (): any => http.Get(`/apps/${slug}/config`),
  saveConfig: (config: string): any => http.Post(`/apps/${slug}/config`, { config }),
  configTune: (): any => http.Get(`/apps/${slug}/config_tune`),
  saveConfigTune: (data: any): any => http.Post(`/apps/${slug}/config_tune`, data),
})

export default createElasticsearchApi('elasticsearch')

import { http } from '@/utils'

export default {
  // 获取配置
  config: (): any => http.Get('/apps/rsync/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/rsync/config', { config }),
  // 模块列表
  modules: (page: number, limit: number): any =>
    http.Get('/apps/rsync/modules', { params: { page, limit } }),
  // 添加模块
  addModule: (module: any): any => http.Post('/apps/rsync/modules', module),
  // 删除模块
  deleteModule: (name: string): any => http.Delete(`/apps/rsync/modules/${name}`),
  // 更新模块
  updateModule: (name: string, module: any): any =>
    http.Post(`/apps/rsync/modules/${name}`, module),
}

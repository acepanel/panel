import { http } from '@/utils'

export default {
  // 服务名称
  service: (): any => http.Get('/apps/supervisor/service'),
  // 获取错误日志
  log: (): any => http.Get('/apps/supervisor/log'),
  // 获取配置
  config: (): any => http.Get('/apps/supervisor/config'),
  // 保存配置
  saveConfig: (config: string): any => http.Post('/apps/supervisor/config', { config }),
  // 进程列表
  processes: (page: number, limit: number): any =>
    http.Get('/apps/supervisor/processes', { params: { page, limit } }),
  // 进程启动
  startProcess: (process: string): any => http.Post(`/apps/supervisor/processes/${process}/start`),
  // 进程停止
  stopProcess: (process: string): any => http.Post(`/apps/supervisor/processes/${process}/stop`),
  // 进程重启
  restartProcess: (process: string): any =>
    http.Post(`/apps/supervisor/processes/${process}/restart`),
  // 进程日志
  processLog: (process: string): any => http.Get(`/apps/supervisor/processes/${process}/log`),
  // 进程配置
  processConfig: (process: string): any => http.Get(`/apps/supervisor/processes/${process}`),
  // 保存进程配置
  saveProcessConfig: (process: string, config: string): any =>
    http.Post(`/apps/supervisor/processes/${process}`, { config }),
  // 创建进程
  createProcess: (process: any): any => http.Post('/apps/supervisor/processes', process),
  // 删除进程
  deleteProcess: (process: string): any => http.Delete(`/apps/supervisor/processes/${process}`),
}

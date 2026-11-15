import { http } from '@/utils'

export default {
  // 状态与环境检测
  status: (): any => http.Get('/tamper/status'),
  // 保存全局设置(含开关)
  saveSetting: (data: any): any => http.Post('/tamper/setting', data),
  // 激活 eBPF(改 grub 并重启系统)
  activateEBPF: (): any => http.Post('/tamper/activate_ebpf'),
  // 批量查询路径保护状态
  checkPaths: (paths: string[]): any => http.Post('/tamper/check_paths', { paths }),
  // 添加/移除路径保护
  protect: (path: string, protect: boolean): any => http.Post('/tamper/protect', { path, protect }),
  // 规则列表
  rules: (page: number, limit: number): any =>
    http.Get('/tamper/rule', { params: { page, limit } }),
  // 新增规则
  createRule: (data: any): any => http.Post('/tamper/rule', data),
  // 更新规则
  updateRule: (id: number, data: any): any => http.Put(`/tamper/rule/${id}`, data),
  // 删除规则
  deleteRule: (id: number): any => http.Delete(`/tamper/rule/${id}`),
  // 拦截日志
  logs: (page: number, limit: number): any => http.Get('/tamper/log', { params: { page, limit } }),
  // 清空日志
  clearLogs: (): any => http.Delete('/tamper/log'),
}

import { http } from '@/utils'

export interface ProcessListParams {
  page: number
  limit: number
  sort?: string // pid, name, cpu, rss, start_time, ppid, num_threads
  order?: string // asc, desc
  status?: string // R, S, T, I, Z, W, L
  keyword?: string
}

export default {
  // 获取进程列表
  list: (params: ProcessListParams) => http.Get(`/process`, { params }),
  // 获取进程详情
  detail: (pid: number) => http.Get(`/process/detail`, { params: { pid } }),
  // 杀死进程 (SIGKILL)
  kill: (pid: number) => http.Post(`/process/kill`, { pid }),
  // 向进程发送信号
  signal: (pid: number, signal: number) => http.Post(`/process/signal`, { pid, signal })
}

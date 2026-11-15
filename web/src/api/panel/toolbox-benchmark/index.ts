import { http } from '@/utils'

// 被测机器的概况，cores 为物理核心数，取不到时为 0；memory 为字节
export interface BenchmarkMachine {
  model: string
  cores: number
  threads: number
  memory: number
}

// 一项 CPU 测试的单线程与多线程得分
export interface BenchmarkCpu {
  single: number
  multi: number
  threads: number
}

// bandwidth 为字节/秒，latency 为纳秒
export interface BenchmarkMemory {
  score: number
  bandwidth: number
  latency: number
}

// 顺序读写为字节/秒，随机读写为 IOPS
export interface BenchmarkDisk {
  score: number
  seq_read: number
  seq_write: number
  rand_read: number
  rand_write: number
}

export default {
  // 被测机器的概况
  machine: (): any => http.Get('/toolbox_benchmark/machine'),
  // 运行一项测试，测不了的原因由页面显示在对应的卡片里
  test: (name: string): any =>
    http.Post('/toolbox_benchmark/test', { name }, { meta: { noAlert: true } }),
}

// 配置调优预设生成
export interface TuneProfile {
  memoryMB: number
  cpuCores: number
  disk: 'hdd' | 'ssd' | 'nvme'
  scenario: string
  connections: number
}

export interface SizePair {
  num: number
  unit: string
}

// MB 数值转 num+unit
export const mbToPair = (mb: number, units: [string, string] = ['MB', 'GB']): SizePair => {
  if (mb >= 1024 && mb % 1024 === 0) {
    return { num: mb / 1024, unit: units[1] }
  }
  return { num: mb, unit: units[0] }
}

// kB 数值转 num+unit
export const kbToPair = (kb: number, units: [string, string] = ['kB', 'MB']): SizePair => {
  if (kb >= 1024 && kb % 1024 === 0) {
    return { num: kb / 1024, unit: units[1] }
  }
  return { num: kb, unit: units[0] }
}

// PostgreSQL，算法对齐 PGTune（PostgreSQL 18、数据量为内存的 1 到 3 倍），null 表示沿用默认值
export const postgresqlPreset = (p: TuneProfile) => {
  const dw = p.scenario === 'dw'
  const sharedBuffersMB = Math.floor(p.memoryMB / 4)
  const maintenanceWorkMemMB = Math.min(Math.floor(p.memoryMB / (dw ? 8 : 16)), 8192)
  const walSizes: Record<string, [number, number]> = {
    web: [1, 4],
    oltp: [2, 8],
    dw: [4, 16],
    mixed: [1, 4],
  }
  const [minWal, maxWal] = walSizes[p.scenario] ?? [1, 4]
  // 少于 4 核时不调并行参数，此时 max_parallel_workers 是默认的 8
  const parallel = p.cpuCores >= 4
  const parallelWorkers = parallel ? Math.min(p.cpuCores, 1024) : 8
  const halfWorkers = Math.ceil(parallelWorkers / 2)
  // 数仓放宽单个查询的 worker 数，但要留够 4 个查询同时并行
  const workersPerGather = dw
    ? Math.min(halfWorkers, Math.max(4, Math.floor(parallelWorkers / 4)))
    : Math.min(halfWorkers, 4)
  // shared_buffers 以外的内存由连接和并行 worker 分摊，每个按同时 3 次排序或哈希估算
  const workMemBudgetMB = (p.memoryMB - sharedBuffersMB) / ((p.connections + parallelWorkers) * 3)
  const workMemMB = Math.floor(dw || p.scenario === 'mixed' ? workMemBudgetMB / 2 : workMemBudgetMB)
  const ioConcurrency = { hdd: 2, ssd: 200, nvme: 1000 }[p.disk]
  const autovacuumMaxWorkers = p.cpuCores >= 32 ? 5 : p.cpuCores >= 16 ? 4 : null

  return {
    maxConnections: p.connections,
    sharedBuffers: mbToPair(sharedBuffersMB),
    effectiveCacheSize: mbToPair(Math.floor((p.memoryMB * 3) / 4)),
    maintenanceWorkMem: mbToPair(maintenanceWorkMemMB),
    workMem: mbToPair(Math.max(workMemMB, 4)),
    // shared_buffers 的 1/32，与 wal_buffers = -1 时自动取的值相同
    walBuffersKB: Math.min(Math.max(sharedBuffersMB * 32, 64), 16384),
    walCompression: 'lz4',
    minWalSizeGB: minWal,
    maxWalSizeGB: maxWal,
    checkpointCompletionTarget: '0.9',
    defaultStatisticsTarget: dw ? 500 : 100,
    // 数仓放不进内存时，过低的随机读代价会让大范围读取误选索引扫描
    randomPageCost: p.disk === 'hdd' || dw ? '4.0' : '1.1',
    effectiveIoConcurrency: ioConcurrency,
    maintenanceIoConcurrency: ioConcurrency,
    maxWorkerProcesses: parallel ? Math.max(parallelWorkers, 8) : null,
    maxParallelWorkers: parallel ? parallelWorkers : null,
    maxParallelWorkersPerGather: parallel ? workersPerGather : null,
    maxParallelMaintenanceWorkers: parallel ? Math.min(halfWorkers, 4) : null,
    autovacuumMaxWorkers,
    // maintenance_work_mem 调大后，每个自动清理 worker 都按它申请内存会有 OOM 风险
    autovacuumWorkMem: maintenanceWorkMemMB > 1024 ? mbToPair(1024) : null,
    // 代价上限由所有 worker 平分，随 worker 数放大才能保持单个 worker 的速度
    autovacuumVacuumCostLimit: autovacuumMaxWorkers && Math.round((200 * autovacuumMaxWorkers) / 3),
    hugePages: p.memoryMB >= 32768 ? 'try' : 'off',
  }
}

// MySQL
export const mysqlPreset = (p: TuneProfile) => {
  const mem = p.memoryMB
  const mg: [string, string] = ['M', 'G']
  const bufferPoolMB = Math.max(Math.floor((mem * 0.55) / 128) * 128, 128)
  const connections = Math.min(Math.max(Math.ceil(mem / 12 / 50) * 50, 100), 1000)
  // 小 / 中 / 大三档：<2G / <8G / 其余
  const pick = (small: number, medium: number, large: number) =>
    mem < 2048 ? small : mem < 8192 ? medium : large

  return {
    maxConnections: connections,
    tableOpenCache: pick(1024, 4000, 8000),
    threadCacheSize: Math.min(Math.max(Math.floor(connections / 8), 8), 100),
    keyBufferSize: mbToPair(pick(8, 16, 32), mg),
    sortBufferSizeK: pick(512, 1024, 2048),
    readBufferSizeK: pick(256, 512, 1024),
    readRndBufferSizeK: pick(512, 512, 1024),
    joinBufferSizeK: pick(512, 512, 2048),
    threadStackK: 512,
    myisamSortBufferSize: mbToPair(pick(8, 16, 32), mg),
    tmpTableSize: mbToPair(pick(32, 64, 256), mg),
    maxHeapTableSize: mbToPair(pick(32, 64, 256), mg),
    innodbBufferPoolSize: mbToPair(bufferPoolMB, mg),
    innodbLogBufferSize: mbToPair(pick(16, 32, 64), mg),
    innodbReadIoThreads: Math.max(Math.floor(p.cpuCores / 2), 1),
    innodbWriteIoThreads: Math.max(Math.floor(p.cpuCores / 2), 1),
  }
}

// Redis
export const redisPreset = (p: TuneProfile) => {
  const policies: Record<string, string> = {
    cache: 'allkeys-lru',
    mixed: 'volatile-lru',
    persistent: 'noeviction',
  }

  return {
    maxmemoryMB: Math.floor(p.memoryMB * 0.7),
    maxmemoryPolicy: policies[p.scenario] ?? 'noeviction',
    appendonly: p.scenario === 'cache' ? 'no' : 'yes',
    appendfsync: 'everysec',
  }
}

// Nginx
export const nginxPreset = (p: TuneProfile) => {
  return {
    workerProcesses: 'auto',
    workerConnections: p.memoryMB < 2048 ? 10240 : 65535,
    keepaliveTimeout: 60,
    gzip: 'on',
    gzipCompLevel: 6,
    brotli: 'on',
    brotliCompLevel: 6,
    brotliStatic: 'on',
    zstd: 'on',
    zstdCompLevel: 10,
    zstdStatic: 'on',
  }
}

// PHP-FPM / OPcache
export const phpPreset = (p: TuneProfile) => {
  const perProcessMB = Number(p.scenario) || 50
  const maxChildren = Math.max(Math.floor(p.memoryMB / perProcessMB), 4)
  // OPcache 按项目体量给，但不超过分配内存的 1/4
  const opcacheMB = Math.min(
    perProcessMB >= 128 ? 512 : perProcessMB >= 80 ? 256 : 128,
    Math.max(Math.floor(p.memoryMB / 4), 64),
  )

  return {
    pm: 'dynamic',
    pmMaxChildren: maxChildren,
    pmStartServers: Math.max(Math.floor(maxChildren / 4), 2),
    pmMinSpareServers: Math.max(Math.floor(maxChildren / 8), 2),
    pmMaxSpareServers: Math.max(Math.floor(maxChildren / 2), 4),
    memoryLimitM: perProcessMB >= 80 ? 512 : 256,
    opcacheMemoryConsumption: opcacheMB,
    opcacheInternedStringsBuffer: Math.min(Math.floor(opcacheMB / 4), 64),
    opcacheMaxAcceleratedFiles: 100000,
    opcacheRevalidateFreq: 3,
    opcacheJit: 'tracing',
    opcacheJitBufferSizeM: Math.floor(opcacheMB / 2),
  }
}

// Apache
export const apachePreset = (p: TuneProfile) => {
  const threadsPerChild = 25
  const workers = Math.max(
    Math.floor(p.memoryMB / 8 / threadsPerChild) * threadsPerChild,
    threadsPerChild * 2,
  )

  return {
    startServers: Math.min(Math.max(Math.floor(p.cpuCores / 2), 2), 4),
    threadsPerChild,
    maxRequestWorkers: Math.min(workers, 1600),
    minSpareThreads: threadsPerChild,
    maxSpareThreads: threadsPerChild * 3,
    maxConnectionsPerChild: 0,
    timeout: 60,
    keepAlive: 'On',
    maxKeepAliveRequests: 1000,
    keepAliveTimeout: 5,
  }
}

<script setup lang="ts">
defineOptions({
  name: 'postgresql-config-tune',
})

import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'
import {
  composeSize,
  toOptions,
  type TuneField,
  type TuneGroup,
  type TunePreset,
} from '@/utils/tune'
import { kbToPair, postgresqlPreset, type SizePair } from '@/utils/tunepreset'

const { $gettext } = useGettext()

const size = {
  type: 'size',
  min: 0,
  units: ['kB', 'MB', 'GB'],
  defaultUnit: 'MB',
} satisfies Partial<TuneField>

const duration = {
  type: 'size',
  units: ['s', 'min', 'h'],
  defaultUnit: 'min',
} satisfies Partial<TuneField>

const groups: TuneGroup[] = [
  {
    name: 'connection',
    tab: $gettext('Connection'),
    tip: $gettext('PostgreSQL connection and authentication settings.'),
    fields: [
      {
        key: 'listen_addresses',
        label: $gettext('Listen Addresses (listen_addresses)'),
        type: 'text',
        placeholder: $gettext('e.g. localhost or *'),
      },
      {
        key: 'port',
        label: $gettext('Port (port)'),
        type: 'number',
        placeholder: $gettext('e.g. 5432'),
        min: 1,
        max: 65535,
      },
      {
        key: 'max_connections',
        label: $gettext('Max Connections (max_connections)'),
        type: 'number',
        placeholder: $gettext('e.g. 200'),
        min: 1,
      },
      {
        key: 'superuser_reserved_connections',
        label: $gettext('Superuser Reserved Connections (superuser_reserved_connections)'),
        type: 'number',
        placeholder: $gettext('e.g. 3'),
        min: 0,
      },
      {
        key: 'idle_in_transaction_session_timeout',
        label: $gettext(
          'Idle In Transaction Session Timeout (idle_in_transaction_session_timeout)',
        ),
        placeholder: $gettext('e.g. 10, 0 disables the timeout'),
        min: 0,
        ...duration,
      },
    ],
  },
  {
    name: 'memory',
    tab: $gettext('Memory'),
    tip: $gettext('PostgreSQL memory allocation settings.'),
    fields: [
      {
        key: 'shared_buffers',
        label: $gettext('Shared Buffers (shared_buffers)'),
        placeholder: $gettext('e.g. 256'),
        ...size,
      },
      {
        key: 'work_mem',
        label: $gettext('Work Mem (work_mem)'),
        placeholder: $gettext('e.g. 1260'),
        ...size,
      },
      {
        key: 'hash_mem_multiplier',
        label: $gettext('Hash Mem Multiplier (hash_mem_multiplier)'),
        type: 'text',
        placeholder: $gettext('e.g. 2.0'),
      },
      {
        key: 'maintenance_work_mem',
        label: $gettext('Maintenance Work Mem (maintenance_work_mem)'),
        placeholder: $gettext('e.g. 64'),
        ...size,
      },
      {
        key: 'temp_buffers',
        label: $gettext('Temp Buffers (temp_buffers)'),
        placeholder: $gettext('e.g. 8'),
        ...size,
      },
      {
        key: 'effective_cache_size',
        label: $gettext('Effective Cache Size (effective_cache_size)'),
        placeholder: $gettext('e.g. 768'),
        ...size,
      },
      {
        key: 'huge_pages',
        label: $gettext('Huge Pages (huge_pages)'),
        type: 'select',
        options: toOptions(['off', 'on', 'try']),
      },
    ],
  },
  {
    name: 'wal',
    tab: 'WAL',
    tip: $gettext('Write-Ahead Logging (WAL) settings.'),
    fields: [
      {
        key: 'wal_level',
        label: $gettext('WAL Level (wal_level)'),
        type: 'select',
        options: toOptions(['minimal', 'replica', 'logical']),
      },
      {
        key: 'max_wal_senders',
        label: $gettext('Max WAL Senders (max_wal_senders)'),
        type: 'number',
        placeholder: $gettext('e.g. 10, must be 0 when wal_level is minimal'),
        min: 0,
      },
      {
        key: 'wal_buffers',
        label: $gettext('WAL Buffers (wal_buffers)'),
        placeholder: $gettext('e.g. 7864'),
        ...size,
      },
      {
        key: 'wal_compression',
        label: $gettext('WAL Compression (wal_compression)'),
        type: 'select',
        options: toOptions(['off', 'pglz', 'lz4', 'zstd']),
      },
      {
        key: 'synchronous_commit',
        label: $gettext('Synchronous Commit (synchronous_commit)'),
        type: 'select',
        options: toOptions(['on', 'off', 'local', 'remote_write', 'remote_apply']),
      },
      {
        key: 'max_wal_size',
        label: $gettext('Max WAL Size (max_wal_size)'),
        placeholder: $gettext('e.g. 4'),
        ...size,
      },
      {
        key: 'min_wal_size',
        label: $gettext('Min WAL Size (min_wal_size)'),
        placeholder: $gettext('e.g. 1'),
        ...size,
      },
      {
        key: 'checkpoint_timeout',
        label: $gettext('Checkpoint Timeout (checkpoint_timeout)'),
        placeholder: $gettext('e.g. 15'),
        min: 1,
        ...duration,
      },
      {
        key: 'checkpoint_completion_target',
        label: $gettext('Checkpoint Completion Target (checkpoint_completion_target)'),
        type: 'text',
        placeholder: $gettext('e.g. 0.9'),
      },
    ],
  },
  {
    name: 'query',
    tab: $gettext('Query Optimization'),
    tip: $gettext('Query planner and optimization settings.'),
    fields: [
      {
        key: 'default_statistics_target',
        label: $gettext('Default Statistics Target (default_statistics_target)'),
        type: 'number',
        placeholder: $gettext('e.g. 100'),
        min: 1,
      },
      {
        key: 'random_page_cost',
        label: $gettext('Random Page Cost (random_page_cost)'),
        type: 'text',
        placeholder: $gettext('e.g. 1.1'),
      },
      {
        key: 'effective_io_concurrency',
        label: $gettext('Effective IO Concurrency (effective_io_concurrency)'),
        type: 'number',
        placeholder: $gettext('e.g. 200'),
        min: 0,
      },
      {
        key: 'maintenance_io_concurrency',
        label: $gettext('Maintenance IO Concurrency (maintenance_io_concurrency)'),
        type: 'number',
        placeholder: $gettext('e.g. 200'),
        min: 0,
      },
    ],
  },
  {
    name: 'parallel',
    tab: $gettext('Parallel'),
    tip: $gettext(
      'Parallel query and background worker settings. max_worker_processes requires PostgreSQL restart to take effect.',
    ),
    fields: [
      {
        key: 'max_worker_processes',
        label: $gettext('Max Worker Processes (max_worker_processes)'),
        type: 'number',
        placeholder: $gettext('e.g. 8'),
        min: 0,
      },
      {
        key: 'max_parallel_workers',
        label: $gettext('Max Parallel Workers (max_parallel_workers)'),
        type: 'number',
        placeholder: $gettext('e.g. 8'),
        min: 0,
      },
      {
        key: 'max_parallel_workers_per_gather',
        label: $gettext('Max Parallel Workers Per Gather (max_parallel_workers_per_gather)'),
        type: 'number',
        placeholder: $gettext('e.g. 2'),
        min: 0,
      },
      {
        key: 'max_parallel_maintenance_workers',
        label: $gettext('Max Parallel Maintenance Workers (max_parallel_maintenance_workers)'),
        type: 'number',
        placeholder: $gettext('e.g. 2'),
        min: 0,
      },
    ],
  },
  {
    name: 'autovacuum',
    tab: $gettext('Autovacuum'),
    tip: $gettext('Automatic vacuum and analyze settings.'),
    fields: [
      {
        key: 'autovacuum_max_workers',
        label: $gettext('Autovacuum Max Workers (autovacuum_max_workers)'),
        type: 'number',
        placeholder: $gettext('e.g. 3'),
        min: 1,
      },
      {
        key: 'autovacuum_work_mem',
        label: $gettext('Autovacuum Work Mem (autovacuum_work_mem)'),
        placeholder: $gettext('e.g. 256'),
        ...size,
      },
      {
        key: 'autovacuum_vacuum_cost_limit',
        label: $gettext('Autovacuum Vacuum Cost Limit (autovacuum_vacuum_cost_limit)'),
        type: 'number',
        placeholder: $gettext('e.g. 200'),
        min: -1,
        max: 10000,
      },
      {
        key: 'autovacuum_vacuum_scale_factor',
        label: $gettext('Autovacuum Vacuum Scale Factor (autovacuum_vacuum_scale_factor)'),
        type: 'text',
        placeholder: $gettext('e.g. 0.2'),
      },
      {
        key: 'autovacuum_analyze_scale_factor',
        label: $gettext('Autovacuum Analyze Scale Factor (autovacuum_analyze_scale_factor)'),
        type: 'text',
        placeholder: $gettext('e.g. 0.1'),
      },
    ],
  },
  {
    name: 'logging',
    tab: $gettext('Logging'),
    tip: $gettext('PostgreSQL logging settings.'),
    fields: [
      {
        key: 'log_destination',
        label: $gettext('Log Destination (log_destination)'),
        type: 'text',
        placeholder: $gettext('e.g. stderr'),
      },
      {
        key: 'log_statement',
        label: $gettext('Log Statement (log_statement)'),
        type: 'select',
        options: toOptions(['none', 'ddl', 'mod', 'all']),
      },
      {
        key: 'log_min_duration_statement',
        label: $gettext('Log Min Duration Statement (log_min_duration_statement)'),
        type: 'number',
        placeholder: $gettext('e.g. -1 (disabled) or milliseconds'),
        min: -1,
      },
      {
        key: 'log_lock_waits',
        label: $gettext('Log Lock Waits (log_lock_waits)'),
        type: 'select',
        options: toOptions(['on', 'off']),
      },
      {
        key: 'log_temp_files',
        label: $gettext('Log Temp Files (log_temp_files)'),
        type: 'number',
        placeholder: $gettext('e.g. -1 (disabled) or kilobytes'),
        min: -1,
      },
      {
        key: 'log_timezone',
        label: $gettext('Log Timezone (log_timezone)'),
        type: 'text',
        placeholder: $gettext('e.g. Asia/Shanghai'),
      },
    ],
  },
  {
    name: 'io',
    tab: 'IO',
    tip: $gettext('IO and storage settings. io_method requires PostgreSQL restart to take effect.'),
    fields: [
      {
        key: 'io_method',
        label: $gettext('IO Method (io_method)'),
        type: 'select',
        options: toOptions(['sync', 'worker', 'io_uring']),
      },
      {
        key: 'default_toast_compression',
        label: $gettext('Default TOAST Compression (default_toast_compression)'),
        type: 'select',
        options: toOptions(['pglz', 'lz4']),
      },
    ],
  },
]

const compose = ({ num, unit }: SizePair) => composeSize(num, unit)

const preset: TunePreset = {
  fields: ['memory', 'cpu', 'disk', 'connections'],
  scenarioOptions: [
    { label: $gettext('Web Application'), value: 'web' },
    { label: 'OLTP', value: 'oltp' },
    { label: $gettext('Data Warehouse'), value: 'dw' },
    { label: $gettext('Mixed'), value: 'mixed' },
  ],
  memoryRatio: 0.75,
  apply: (profile) => {
    const r = postgresqlPreset(profile)
    return {
      max_connections: r.maxConnections,
      shared_buffers: compose(r.sharedBuffers),
      effective_cache_size: compose(r.effectiveCacheSize),
      maintenance_work_mem: compose(r.maintenanceWorkMem),
      work_mem: compose(r.workMem),
      wal_buffers: compose(kbToPair(r.walBuffersKB)),
      wal_compression: r.walCompression,
      min_wal_size: `${r.minWalSizeGB}GB`,
      max_wal_size: `${r.maxWalSizeGB}GB`,
      checkpoint_completion_target: r.checkpointCompletionTarget,
      default_statistics_target: r.defaultStatisticsTarget,
      random_page_cost: r.randomPageCost,
      effective_io_concurrency: r.effectiveIoConcurrency,
      maintenance_io_concurrency: r.maintenanceIoConcurrency,
      max_worker_processes: r.maxWorkerProcesses,
      max_parallel_workers: r.maxParallelWorkers,
      max_parallel_workers_per_gather: r.maxParallelWorkersPerGather,
      max_parallel_maintenance_workers: r.maxParallelMaintenanceWorkers,
      autovacuum_max_workers: r.autovacuumMaxWorkers,
      autovacuum_work_mem: r.autovacuumWorkMem && compose(r.autovacuumWorkMem),
      autovacuum_vacuum_cost_limit: r.autovacuumVacuumCostLimit,
      huge_pages: r.hugePages,
    }
  },
}
</script>

<template>
  <config-tune-form
    :fetch="postgresql.configTune"
    :save="postgresql.saveConfigTune"
    :groups="groups"
    :preset="preset"
  />
</template>

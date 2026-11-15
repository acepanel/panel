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
        key: 'maintenance_work_mem',
        label: $gettext('Maintenance Work Mem (maintenance_work_mem)'),
        placeholder: $gettext('e.g. 64'),
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
        key: 'wal_buffers',
        label: $gettext('WAL Buffers (wal_buffers)'),
        placeholder: $gettext('e.g. 7864'),
        ...size,
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
        key: 'log_min_duration_statement',
        label: $gettext('Log Min Duration Statement (log_min_duration_statement)'),
        type: 'number',
        placeholder: $gettext('e.g. -1 (disabled) or milliseconds'),
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
    tip: $gettext('IO method settings. Requires PostgreSQL restart to take effect.'),
    fields: [
      {
        key: 'io_method',
        label: $gettext('IO Method (io_method)'),
        type: 'select',
        options: toOptions(['sync', 'worker', 'io_uring']),
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
      work_mem: compose(kbToPair(r.workMemKB)),
      wal_buffers: compose(kbToPair(r.walBuffersKB)),
      min_wal_size: `${r.minWalSizeGB}GB`,
      max_wal_size: `${r.maxWalSizeGB}GB`,
      checkpoint_completion_target: r.checkpointCompletionTarget,
      default_statistics_target: r.defaultStatisticsTarget,
      random_page_cost: r.randomPageCost,
      effective_io_concurrency: r.effectiveIoConcurrency,
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

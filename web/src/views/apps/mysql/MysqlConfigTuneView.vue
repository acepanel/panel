<script setup lang="ts">
defineOptions({
  name: 'mysql-config-tune',
})

import { useGettext } from 'vue3-gettext'

import {
  composeSize,
  toOptions,
  type TuneField,
  type TuneGroup,
  type TunePreset,
} from '@/utils/tune'
import { kbToPair, mysqlPreset, type SizePair } from '@/utils/tunepreset'

defineProps<{
  api: any
}>()

const { $gettext } = useGettext()

const size = {
  type: 'size',
  min: 0,
  units: ['K', 'M', 'G'],
  defaultUnit: 'M',
} satisfies Partial<TuneField>

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: $gettext('Common MySQL general settings.'),
    fields: [
      {
        key: 'port',
        label: $gettext('Port (port)'),
        type: 'number',
        placeholder: $gettext('e.g. 3306'),
        min: 1,
        max: 65535,
      },
      {
        key: 'max_connections',
        label: $gettext('Max Connections (max_connections)'),
        type: 'number',
        placeholder: $gettext('e.g. 50'),
        min: 1,
      },
      {
        key: 'max_connect_errors',
        label: $gettext('Max Connect Errors (max_connect_errors)'),
        type: 'number',
        placeholder: $gettext('e.g. 100'),
        min: 1,
      },
      {
        key: 'default_storage_engine',
        label: $gettext('Default Storage Engine (default_storage_engine)'),
        type: 'select',
        options: [
          ...toOptions(['InnoDB', 'MyISAM', 'MEMORY', 'CSV']),
          { label: 'RocksDB', value: 'ROCKSDB' },
        ],
      },
      {
        key: 'table_open_cache',
        label: $gettext('Table Open Cache (table_open_cache)'),
        type: 'number',
        placeholder: $gettext('e.g. 64'),
        min: 1,
      },
      {
        key: 'max_allowed_packet',
        label: $gettext('Max Allowed Packet (max_allowed_packet)'),
        placeholder: $gettext('e.g. 1'),
        ...size,
      },
      {
        key: 'open_files_limit',
        label: $gettext('Open Files Limit (open_files_limit)'),
        type: 'number',
        placeholder: $gettext('e.g. 65535'),
        min: 1,
      },
    ],
  },
  {
    name: 'performance',
    tab: $gettext('Performance Tuning'),
    tip: $gettext('MySQL performance buffer and cache settings.'),
    fields: [
      {
        key: 'key_buffer_size',
        label: $gettext('Key Buffer Size (key_buffer_size)'),
        placeholder: $gettext('e.g. 8'),
        ...size,
      },
      {
        key: 'sort_buffer_size',
        label: $gettext('Sort Buffer Size (sort_buffer_size)'),
        placeholder: $gettext('e.g. 256'),
        ...size,
      },
      {
        key: 'read_buffer_size',
        label: $gettext('Read Buffer Size (read_buffer_size)'),
        placeholder: $gettext('e.g. 256'),
        ...size,
      },
      {
        key: 'read_rnd_buffer_size',
        label: $gettext('Read Rnd Buffer Size (read_rnd_buffer_size)'),
        placeholder: $gettext('e.g. 256'),
        ...size,
      },
      {
        key: 'join_buffer_size',
        label: $gettext('Join Buffer Size (join_buffer_size)'),
        placeholder: $gettext('e.g. 128'),
        ...size,
      },
      {
        key: 'thread_cache_size',
        label: $gettext('Thread Cache Size (thread_cache_size)'),
        type: 'number',
        placeholder: $gettext('e.g. 16'),
        min: 0,
      },
      {
        key: 'thread_stack',
        label: $gettext('Thread Stack (thread_stack)'),
        placeholder: $gettext('e.g. 192'),
        ...size,
      },
      {
        key: 'tmp_table_size',
        label: $gettext('Tmp Table Size (tmp_table_size)'),
        placeholder: $gettext('e.g. 16'),
        ...size,
      },
      {
        key: 'max_heap_table_size',
        label: $gettext('Max Heap Table Size (max_heap_table_size)'),
        placeholder: $gettext('e.g. 16'),
        ...size,
      },
      {
        key: 'myisam_sort_buffer_size',
        label: $gettext('MyISAM Sort Buffer Size (myisam_sort_buffer_size)'),
        placeholder: $gettext('e.g. 8'),
        ...size,
      },
    ],
  },
  {
    name: 'innodb',
    tab: 'InnoDB',
    tip: $gettext('InnoDB storage engine settings.'),
    fields: [
      {
        key: 'innodb_buffer_pool_size',
        label: $gettext('Buffer Pool Size (innodb_buffer_pool_size)'),
        placeholder: $gettext('e.g. 64'),
        ...size,
      },
      {
        key: 'innodb_log_buffer_size',
        label: $gettext('Log Buffer Size (innodb_log_buffer_size)'),
        placeholder: $gettext('e.g. 16'),
        ...size,
      },
      {
        key: 'innodb_flush_log_at_trx_commit',
        label: $gettext('Flush Log At Trx Commit (innodb_flush_log_at_trx_commit)'),
        type: 'select',
        options: toOptions(['0', '1', '2']),
      },
      {
        key: 'innodb_lock_wait_timeout',
        label: $gettext('Lock Wait Timeout (innodb_lock_wait_timeout)'),
        type: 'number',
        placeholder: $gettext('e.g. 50'),
        min: 0,
      },
      {
        key: 'innodb_max_dirty_pages_pct',
        label: $gettext('Max Dirty Pages Pct (innodb_max_dirty_pages_pct)'),
        type: 'number',
        placeholder: $gettext('e.g. 90'),
        min: 0,
        max: 100,
      },
      {
        key: 'innodb_read_io_threads',
        label: $gettext('Read IO Threads (innodb_read_io_threads)'),
        type: 'number',
        placeholder: $gettext('e.g. 1'),
        min: 1,
      },
      {
        key: 'innodb_write_io_threads',
        label: $gettext('Write IO Threads (innodb_write_io_threads)'),
        type: 'number',
        placeholder: $gettext('e.g. 1'),
        min: 1,
      },
    ],
  },
  {
    name: 'logging',
    tab: $gettext('Logging'),
    tip: $gettext('MySQL logging settings.'),
    fields: [
      {
        key: 'slow_query_log',
        label: $gettext('Slow Query Log (slow_query_log)'),
        type: 'select',
        options: [
          { label: $gettext('On'), value: '1' },
          { label: $gettext('Off'), value: '0' },
        ],
      },
      {
        key: 'long_query_time',
        label: $gettext('Long Query Time (long_query_time)'),
        type: 'number',
        placeholder: $gettext('e.g. 3 (seconds)'),
        min: 0,
      },
    ],
  },
]

const compose = ({ num, unit }: SizePair) => composeSize(num, unit)

const preset: TunePreset = {
  fields: ['memory', 'cpu'],
  memoryRatio: 0.75,
  apply: (profile) => {
    const r = mysqlPreset(profile)
    return {
      max_connections: r.maxConnections,
      table_open_cache: r.tableOpenCache,
      key_buffer_size: compose(r.keyBufferSize),
      sort_buffer_size: compose(kbToPair(r.sortBufferSizeK, ['K', 'M'])),
      read_buffer_size: compose(kbToPair(r.readBufferSizeK, ['K', 'M'])),
      read_rnd_buffer_size: compose(kbToPair(r.readRndBufferSizeK, ['K', 'M'])),
      join_buffer_size: compose(kbToPair(r.joinBufferSizeK, ['K', 'M'])),
      thread_stack: `${r.threadStackK}K`,
      myisam_sort_buffer_size: compose(r.myisamSortBufferSize),
      thread_cache_size: r.threadCacheSize,
      tmp_table_size: compose(r.tmpTableSize),
      max_heap_table_size: compose(r.maxHeapTableSize),
      innodb_buffer_pool_size: compose(r.innodbBufferPoolSize),
      innodb_log_buffer_size: compose(r.innodbLogBufferSize),
      innodb_read_io_threads: r.innodbReadIoThreads,
      innodb_write_io_threads: r.innodbWriteIoThreads,
    }
  },
}
</script>

<template>
  <config-tune-form
    :fetch="api.configTune"
    :save="api.saveConfigTune"
    :groups="groups"
    :preset="preset"
  />
</template>

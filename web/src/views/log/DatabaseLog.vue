<script setup lang="ts">
defineOptions({
  name: 'database-log',
})

import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import LogTable from './LogTable.vue'

const { $gettext } = useGettext()

interface LogEntry {
  time: string
  level: string
  msg: string
  extra?: Record<string, any>
}

const columns = [
  {
    title: $gettext('Time'),
    key: 'time',
    width: 180,
    render: (row: LogEntry) => {
      const date = new Date(row.time)
      return date.toLocaleString()
    },
  },
  {
    title: $gettext('Level'),
    key: 'level',
    width: 80,
    render: (row: LogEntry) => {
      const typeMap: Record<string, 'success' | 'warning' | 'error' | 'info'> = {
        INFO: 'success',
        WARN: 'warning',
        ERROR: 'error',
        DEBUG: 'info',
      }
      return h(NTag, { type: typeMap[row.level] || 'default', size: 'small' }, () => row.level)
    },
  },
  {
    title: $gettext('Query'),
    key: 'query',
    ellipsis: {
      tooltip: true,
    },
    render: (row: LogEntry) => {
      return row.extra?.query || row.msg || '-'
    },
  },
  {
    title: $gettext('Duration'),
    key: 'duration',
    width: 120,
    render: (row: LogEntry) => {
      if (row.extra?.duration) {
        // 纳秒转毫秒
        const ms = Number(row.extra.duration) / 1000000
        return `${ms.toFixed(2)} ms`
      }
      return '-'
    },
  },
  {
    title: $gettext('Rows'),
    key: 'rows',
    width: 80,
    render: (row: LogEntry) => {
      return row.extra?.rows !== undefined ? row.extra.rows : '-'
    },
  },
]
</script>

<template>
  <log-table type="db" :columns="columns" />
</template>

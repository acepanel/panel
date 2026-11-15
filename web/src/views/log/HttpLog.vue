<script setup lang="ts">
defineOptions({
  name: 'http-log',
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

// 获取状态码颜色
const getStatusType = (code: number): 'success' | 'warning' | 'error' | 'info' => {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 300 && code < 400) return 'info'
  if (code >= 400 && code < 500) return 'warning'
  return 'error'
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
    title: $gettext('Method'),
    key: 'method',
    width: 80,
    render: (row: LogEntry) => {
      const method = row.extra?.['http.request.method'] || '-'
      const colorMap: Record<string, string> = {
        GET: '#52c41a',
        POST: '#1890ff',
        PUT: '#faad14',
        DELETE: '#ff4d4f',
        PATCH: '#722ed1',
      }
      return h(
        'span',
        { style: { color: colorMap[method] || 'inherit', fontWeight: 'bold' } },
        method,
      )
    },
  },
  {
    title: $gettext('Path'),
    key: 'path',
    ellipsis: {
      tooltip: true,
    },
    render: (row: LogEntry) => {
      return row.extra?.['url.path'] || '-'
    },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 80,
    render: (row: LogEntry) => {
      const status = row.extra?.['http.response.status_code']
      if (status) {
        return h(NTag, { type: getStatusType(status), size: 'small' }, () => status)
      }
      return '-'
    },
  },
  {
    title: $gettext('Duration'),
    key: 'duration',
    width: 120,
    render: (row: LogEntry) => {
      const duration = row.extra?.['event.duration']
      if (duration) {
        // 纳秒转毫秒
        const ms = Number(duration) / 1000000
        return `${ms.toFixed(2)} ms`
      }
      return '-'
    },
  },
  {
    title: $gettext('Client IP'),
    key: 'client_ip',
    width: 150,
    render: (row: LogEntry) => {
      const ip = row.extra?.['client.ip'] || '-'
      // 移除端口号
      return typeof ip === 'string' && ip.includes(':') ? ip.split(':')[0] : ip
    },
  },
]
</script>

<template>
  <log-table type="http" :columns="columns" />
</template>

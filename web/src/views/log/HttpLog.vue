<script setup lang="ts">
defineOptions({
  name: 'http-log'
})

import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import log from '@/api/panel/log'

const { $gettext } = useGettext()

// 日志条目类型定义
interface LogEntry {
  time: string
  level: string
  msg: string
  extra?: Record<string, any>
}

// 数据加载
const limit = ref(200)
const { loading, data, send: refresh } = useRequest(
  () => log.list('http', limit.value),
  { initialData: [] }
)

// 获取状态码颜色
const getStatusType = (code: number): 'success' | 'warning' | 'error' | 'info' => {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 300 && code < 400) return 'info'
  if (code >= 400 && code < 500) return 'warning'
  return 'error'
}

// 表格列配置
const columns = [
  {
    title: $gettext('Time'),
    key: 'time',
    width: 180,
    render: (row: LogEntry) => {
      const date = new Date(row.time)
      return date.toLocaleString()
    }
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
        PATCH: '#722ed1'
      }
      return h('span', { style: { color: colorMap[method] || 'inherit', fontWeight: 'bold' } }, method)
    }
  },
  {
    title: $gettext('Path'),
    key: 'path',
    ellipsis: {
      tooltip: true
    },
    render: (row: LogEntry) => {
      return row.extra?.['url.path'] || '-'
    }
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
    }
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
    }
  },
  {
    title: $gettext('Client IP'),
    key: 'client_ip',
    width: 150,
    render: (row: LogEntry) => {
      const ip = row.extra?.['client.ip'] || '-'
      // 移除端口号
      return typeof ip === 'string' && ip.includes(':') ? ip.split(':')[0] : ip
    }
  }
]

// 刷新
const handleRefresh = () => {
  refresh()
}
</script>

<template>
  <div class="flex flex-col h-full">
    <div class="mb-4 flex gap-4 items-center">
      <span>{{ $gettext('Show entries') }}:</span>
      <n-select
        v-model:value="limit"
        :options="[
          { label: '100', value: 100 },
          { label: '200', value: 200 },
          { label: '500', value: 500 },
          { label: '1000', value: 1000 }
        ]"
        class="w-100px"
        @update:value="handleRefresh"
      />
      <n-button type="primary" @click="handleRefresh">
        {{ $gettext('Refresh') }}
      </n-button>
    </div>
    <n-data-table
      :columns="columns"
      :data="data"
      :loading="loading"
      :bordered="false"
      :max-height="600"
      :scroll-x="800"
      virtual-scroll
    />
  </div>
</template>

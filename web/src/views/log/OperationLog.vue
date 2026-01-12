<script setup lang="ts">
defineOptions({
  name: 'operation-log'
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
  type?: string
  operator_id?: number
  operator_name?: string
  extra?: Record<string, any>
}

// 数据加载
const limit = ref(200)
const { loading, data, send: refresh } = useRequest(
  () => log.list('app', limit.value),
  { initialData: [] }
)

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
    title: $gettext('Level'),
    key: 'level',
    width: 80,
    render: (row: LogEntry) => {
      const typeMap: Record<string, 'success' | 'warning' | 'error' | 'info'> = {
        INFO: 'success',
        WARN: 'warning',
        ERROR: 'error',
        DEBUG: 'info'
      }
      return h(NTag, { type: typeMap[row.level] || 'default', size: 'small' }, () => row.level)
    }
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render: (row: LogEntry) => {
      return row.type || '-'
    }
  },
  {
    title: $gettext('Operator'),
    key: 'operator_name',
    width: 120,
    render: (row: LogEntry) => {
      if (row.operator_id === 0 || row.operator_id === undefined) {
        return $gettext('System')
      }
      return row.operator_name || `#${row.operator_id}`
    }
  },
  {
    title: $gettext('Message'),
    key: 'msg',
    ellipsis: {
      tooltip: true
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

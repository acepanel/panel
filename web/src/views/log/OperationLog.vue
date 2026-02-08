<script setup lang="ts">
defineOptions({
  name: 'operation-log'
})

import { NButton, NPopover, NTag } from 'naive-ui'
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
const selectedDate = ref<string | null>(null)

// 获取可用的日志日期列表
const { data: dates } = useRequest(() => log.dates('app'), { initialData: [] })

// 日期选项
const dateOptions = computed(() => {
  const options = [{ label: $gettext('Today'), value: '' }]
  if (dates.value) {
    for (const date of dates.value) {
      options.push({ label: date, value: date })
    }
  }
  return options
})

const { loading, data, send: refresh } = useRequest(
  () => log.list('app', limit.value, selectedDate.value || ''),
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
  },
  {
    title: $gettext('Details'),
    key: 'extra',
    width: 100,
    render: (row: LogEntry) => {
      const extra = row.extra
      if (!extra || Object.keys(extra).length === 0) return '-'

      const formatValue = (value: any): string => {
        if (typeof value === 'object' && value !== null) {
          return JSON.stringify(value, null, 2)
        }
        return String(value)
      }

      return h(
        NPopover,
        { trigger: 'click', placement: 'left', style: { maxWidth: '500px' } },
        {
          trigger: () =>
            h(NButton, { text: true, type: 'primary', size: 'small' }, () =>
              $gettext('%{count} fields', { count: Object.keys(extra).length.toString() })
            ),
          default: () =>
            h(
              'div',
              { style: 'max-height: 400px; overflow-y: auto;' },
              Object.entries(extra).map(([key, value]) =>
                h('div', { style: 'margin-bottom: 8px;' }, [
                  h(
                    'div',
                    {
                      style:
                        'font-weight: bold; color: var(--n-text-color); font-size: 13px; margin-bottom: 2px;'
                    },
                    key
                  ),
                  h(
                    typeof value === 'object' && value !== null ? 'pre' : 'div',
                    {
                      style:
                        typeof value === 'object' && value !== null
                          ? 'background: var(--n-color); padding: 6px 8px; border-radius: 4px; font-size: 12px; white-space: pre-wrap; word-break: break-all; margin: 0;'
                          : 'color: var(--n-text-color); font-size: 13px; word-break: break-all;'
                    },
                    formatValue(value)
                  )
                ])
              )
            )
        }
      )
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
      <span>{{ $gettext('Date') }}:</span>
      <n-select
        v-model:value="selectedDate"
        :options="dateOptions"
        class="w-150px"
        @update:value="handleRefresh"
      />
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
      class="flex-1 min-h-0"
      :columns="columns"
      :data="data"
      :loading="loading"
      :bordered="false"
      flex-height
      :scroll-x="800"
      virtual-scroll
    />
  </div>
</template>

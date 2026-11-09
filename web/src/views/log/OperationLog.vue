<script setup lang="ts">
defineOptions({
  name: 'operation-log',
})

import { NButton, NPopover, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import log from '@/api/panel/log'

import CleanPopover from './CleanPopover.vue'

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
const selectedDate = ref('')
const selectedLevel = ref('')
const keyword = ref('')

// 操作类型显示名，键对应后端 biz.OperationType 常量
const typeLabels: Record<string, string> = {
  panel: $gettext('Panel'),
  website: $gettext('Website'),
  database: $gettext('Database'),
  database_user: $gettext('Database User'),
  database_server: $gettext('Database Server'),
  project: $gettext('Project'),
  cert: $gettext('Certificate'),
  file: $gettext('File'),
  app: $gettext('App'),
  cron: $gettext('Cron'),
  backup: $gettext('Backup'),
  container: $gettext('Container'),
  firewall: $gettext('Firewall'),
  safe: $gettext('Security'),
  ssh: $gettext('SSH'),
  setting: $gettext('Setting'),
  monitor: $gettext('Monitor'),
  webhook: $gettext('Webhook'),
  user: $gettext('User'),
  environment: $gettext('Environment'),
  toolbox: $gettext('Toolbox'),
  system: $gettext('System'),
}

// 日志级别选项
const levelOptions = [
  { label: $gettext('All'), value: '' },
  { label: 'DEBUG', value: 'DEBUG' },
  { label: 'INFO', value: 'INFO' },
  { label: 'WARN', value: 'WARN' },
  { label: 'ERROR', value: 'ERROR' },
]

// 获取可用的日志日期列表
const { data: dates, send: refreshDates } = useRequest(() => log.dates('app'), {
  initialData: [],
})

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

const {
  loading,
  data,
  send: refresh,
} = useRequest(
  () => log.list('app', limit.value, selectedDate.value, selectedLevel.value, keyword.value),
  {
    initialData: [],
  },
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
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render: (row: LogEntry) => {
      return row.type ? typeLabels[row.type] || row.type : '-'
    },
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
    },
  },
  {
    title: $gettext('Message'),
    key: 'msg',
    ellipsis: {
      tooltip: true,
    },
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
        {
          trigger: 'click',
          placement: 'left',
          style: { maxWidth: 'min(500px, calc(100vw - 24px))' },
        },
        {
          trigger: () =>
            h(NButton, { text: true, type: 'primary', size: 'small' }, () =>
              $gettext('%{count} fields', { count: Object.keys(extra).length.toString() }),
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
                        'font-weight: bold; color: var(--n-text-color); font-size: 13px; margin-bottom: 2px;',
                    },
                    key,
                  ),
                  h(
                    typeof value === 'object' && value !== null ? 'pre' : 'div',
                    {
                      style:
                        typeof value === 'object' && value !== null
                          ? 'background: var(--n-color); padding: 6px 8px; border-radius: 4px; font-size: 12px; white-space: pre-wrap; word-break: break-all; margin: 0;'
                          : 'color: var(--n-text-color); font-size: 13px; word-break: break-all;',
                    },
                    formatValue(value),
                  ),
                ]),
              ),
            ),
        },
      )
    },
  },
]

// 刷新
const handleRefresh = () => {
  refresh()
}

// 清理后选中的日期可能已被删掉，回到今天
const handleCleaned = () => {
  selectedDate.value = ''
  refreshDates()
  refresh()
}
</script>

<template>
  <n-flex vertical class="h-full">
    <n-flex align="center">
      <div class="flex gap-3 items-center">
        <span>{{ $gettext('Date') }}:</span>
        <n-select
          v-model:value="selectedDate"
          :options="dateOptions"
          class="w-37"
          @update:value="handleRefresh"
        />
      </div>
      <div class="flex gap-3 items-center">
        <span>{{ $gettext('Level') }}:</span>
        <n-select
          v-model:value="selectedLevel"
          :options="levelOptions"
          class="w-25"
          @update:value="handleRefresh"
        />
      </div>
      <div class="flex gap-3 items-center">
        <span>{{ $gettext('Show entries') }}:</span>
        <n-select
          v-model:value="limit"
          :options="[
            { label: '100', value: 100 },
            { label: '200', value: 200 },
            { label: '500', value: 500 },
            { label: '1000', value: 1000 },
          ]"
          class="w-25"
          @update:value="handleRefresh"
        />
      </div>
      <n-input
        v-model:value="keyword"
        :placeholder="$gettext('Search')"
        clearable
        class="md:!w-60"
        @keyup.enter="handleRefresh"
        @clear="handleRefresh"
      />
      <n-button type="primary" @click="handleRefresh">
        {{ $gettext('Refresh') }}
      </n-button>
      <clean-popover type="app" @cleaned="handleCleaned" />
    </n-flex>
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
  </n-flex>
</template>

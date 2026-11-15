<script setup lang="ts">
defineOptions({
  name: 'operation-log',
})

import { NButton, NPopover, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import LogTable from './LogTable.vue'

const { $gettext } = useGettext()

interface LogEntry {
  time: string
  level: string
  msg: string
  type?: string
  operator_id?: number
  operator_name?: string
  extra?: Record<string, any>
}

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
</script>

<template>
  <log-table type="app" :columns="columns" />
</template>

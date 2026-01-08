<script setup lang="ts">
import { NButton, NDataTable, NPopconfirm, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import process, { type ProcessListParams } from '@/api/panel/process'
import { formatBytes, formatDateTime, formatPercent } from '@/utils'

const { $gettext } = useGettext()

const columns: any = [
  {
    title: 'PID',
    key: 'pid',
    width: 120,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Parent PID'),
    key: 'ppid',
    width: 120,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Threads'),
    key: 'num_threads',
    width: 100,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('User'),
    key: 'username',
    minWidth: 100,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Status'),
    key: 'status',
    minWidth: 150,
    ellipsis: { tooltip: true },
    render(row: any) {
      switch (row.status) {
        case 'R':
          return h(NTag, { type: 'success' }, { default: () => $gettext('Running') })
        case 'S':
          return h(NTag, { type: 'warning' }, { default: () => $gettext('Sleeping') })
        case 'T':
          return h(NTag, { type: 'error' }, { default: () => $gettext('Stopped') })
        case 'I':
          return h(NTag, { type: 'primary' }, { default: () => $gettext('Idle') })
        case 'Z':
          return h(NTag, { type: 'error' }, { default: () => $gettext('Zombie') })
        case 'W':
          return h(NTag, { type: 'warning' }, { default: () => $gettext('Waiting') })
        case 'L':
          return h(NTag, { type: 'info' }, { default: () => $gettext('Locked') })
        default:
          return h(NTag, { type: 'default' }, { default: () => row.status })
      }
    }
  },
  {
    title: 'CPU',
    key: 'cpu',
    minWidth: 100,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatPercent(row.cpu) + '%'
    }
  },
  {
    title: $gettext('Memory'),
    key: 'rss',
    minWidth: 100,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatBytes(row.rss)
    }
  },
  {
    title: $gettext('Start Time'),
    key: 'start_time',
    width: 160,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatDateTime(row.start_time)
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 150,
    hideInExcel: true,
    render(row: any) {
      return h(
        NPopconfirm,
        {
          onPositiveClick: () => {
            useRequest(process.kill(row.pid)).onSuccess(() => {
              refresh()
              window.$message.success(
                $gettext('Process %{ pid } has been terminated', { pid: row.pid })
              )
            })
          }
        },
        {
          default: () => {
            return $gettext('Are you sure you want to terminate process %{ pid }?', {
              pid: row.pid
            })
          },
          trigger: () => {
            return h(
              NButton,
              {
                size: 'small',
                type: 'error'
              },
              {
                default: () => $gettext('Terminate')
              }
            )
          }
        }
      )
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => {
    const params: ProcessListParams = { page, limit: pageSize }
    return process.list(params)
  },
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)
</script>

<template>
  <n-flex vertical>
    <n-data-table
      striped
      remote
      :scroll-x="1400"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.pid"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200]
      }"
    />
  </n-flex>
</template>

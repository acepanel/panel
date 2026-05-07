<script setup lang="ts">
defineOptions({
  name: 'ssh-log',
})

import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import log from '@/api/panel/log'

const { $gettext } = useGettext()

// SSH 日志条目类型
interface SSHLogEntry {
  time: string
  user: string
  ip: string
  port: string
  method: string
  status: string
}

const limit = ref(100)

const { loading, data, send: refresh } = useRequest(() => log.ssh(limit.value), { initialData: [] })

// 状态映射
const getStatusType = (status: string): 'success' | 'warning' | 'error' | 'info' => {
  switch (status) {
    case 'accepted':
      return 'success'
    case 'failed':
      return 'error'
    case 'invalid_user':
      return 'warning'
    default:
      return 'info'
  }
}

const getStatusLabel = (status: string): string => {
  switch (status) {
    case 'accepted':
      return $gettext('Accepted')
    case 'failed':
      return $gettext('Failed')
    case 'invalid_user':
      return $gettext('Invalid User')
    case 'disconnected':
      return $gettext('Disconnected')
    default:
      return status
  }
}

const columns = [
  {
    title: $gettext('Time'),
    key: 'time',
    width: 180,
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 120,
    render: (row: SSHLogEntry) => {
      return h(NTag, { type: getStatusType(row.status), size: 'small' }, () =>
        getStatusLabel(row.status),
      )
    },
  },
  {
    title: $gettext('User'),
    key: 'user',
    width: 120,
  },
  {
    title: $gettext('Source IP'),
    key: 'ip',
    width: 150,
  },
  {
    title: $gettext('Port'),
    key: 'port',
    width: 80,
  },
  {
    title: $gettext('Auth Method'),
    key: 'method',
    width: 120,
  },
]

const handleRefresh = () => {
  refresh()
}
</script>

<template>
  <n-flex vertical class="h-full">
    <n-flex align="center">
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
      <n-button type="primary" @click="handleRefresh">
        {{ $gettext('Refresh') }}
      </n-button>
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

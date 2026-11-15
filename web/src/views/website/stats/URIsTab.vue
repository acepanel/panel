<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import { formatBytes } from '@/utils/file'

import { useStatQuery } from './useStatQuery'

const { $gettext } = useGettext()

const { loading, items, total, pagination } = useStatQuery(({ start, end, sites, page, limit }) =>
  website.statURIs(start, end, sites, page, limit),
)

const columns = computed(() => [
  { title: 'URI', key: 'uri', ellipsis: { tooltip: true } },
  {
    title: $gettext('Requests'),
    key: 'requests',
    sorter: (a: any, b: any) => a.requests - b.requests,
  },
  {
    title: $gettext('Bandwidth'),
    key: 'bandwidth',
    render: (row: any) => formatBytes(row.bandwidth),
    sorter: (a: any, b: any) => a.bandwidth - b.bandwidth,
  },
  { title: $gettext('Errors'), key: 'errors', sorter: (a: any, b: any) => a.errors - b.errors },
])
</script>

<template>
  <n-spin :show="loading">
    <n-data-table :columns="columns" :data="items" :bordered="false" size="small" :scroll-x="600" />
    <n-flex justify="end" class="mt-3" v-if="total > 0">
      <n-pagination v-bind="pagination" />
    </n-flex>
  </n-spin>
</template>

<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'

import { useStatQuery } from './useStatQuery'

const { $gettext } = useGettext()

const statusFilter = ref<number>(0)

const statusOptions = [
  { label: $gettext('All'), value: 0 },
  { label: '400', value: 400 },
  { label: '401', value: 401 },
  { label: '403', value: 403 },
  { label: '404', value: 404 },
  { label: '405', value: 405 },
  { label: '429', value: 429 },
  { label: '500', value: 500 },
  { label: '502', value: 502 },
  { label: '503', value: 503 },
  { label: '504', value: 504 },
]

const { loading, items, total, pagination, reload } = useStatQuery(
  ({ start, end, sites, page, limit }) =>
    website.statErrors(start, end, sites, statusFilter.value || undefined, page, limit),
)

watch(statusFilter, reload)

function formatTime(t: string): string {
  if (!t) return '-'
  const d = new Date(t)
  return d.toLocaleString()
}

const columns = computed<any[]>(() => [
  {
    type: 'expand',
    renderExpand: (row: any) => {
      return h(
        'pre',
        {
          style:
            'margin:0;padding:8px 12px;white-space:pre-wrap;word-break:break-all;font-size:12px;max-height:300px;overflow:auto',
        },
        row.body || $gettext('No request body'),
      )
    },
  },
  {
    title: $gettext('Time'),
    key: 'created_at',
    width: 170,
    render: (row: any) => formatTime(row.created_at),
  },
  { title: $gettext('Site'), key: 'site', width: 140, ellipsis: { tooltip: true } },
  { title: 'URI', key: 'uri', ellipsis: { tooltip: true } },
  { title: $gettext('Method'), key: 'method', width: 80 },
  { title: $gettext('Status'), key: 'status', width: 80 },
  { title: 'IP', key: 'ip', width: 140 },
  { title: 'User-Agent', key: 'ua', ellipsis: { tooltip: true } },
])
</script>

<template>
  <n-flex vertical :size="12">
    <n-flex align="center">
      <span class="text-14px">{{ $gettext('Status Code') }}:</span>
      <n-select v-model:value="statusFilter" :options="statusOptions" class="w-30" size="small" />
    </n-flex>

    <n-spin :show="loading">
      <n-data-table
        :columns="columns"
        :data="items"
        :bordered="false"
        size="small"
        :scroll-x="950"
        :row-key="(row: any) => row.id"
      />
      <n-flex justify="end" class="mt-3" v-if="total > 0">
        <n-pagination v-bind="pagination" />
      </n-flex>
    </n-spin>
  </n-flex>
</template>

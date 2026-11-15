<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import { formatBytes } from '@/utils/file'

import { useStatQuery } from './useStatQuery'

const { $gettext } = useGettext()

const threshold = ref<number | null>(100)

const { loading, items, total, pagination, reload } = useStatQuery(
  ({ start, end, sites, page, limit }) =>
    website.statSlowURIs(
      start,
      end,
      sites,
      threshold.value && threshold.value > 0 ? threshold.value : undefined,
      page,
      limit,
    ),
)

function avgTime(row: any): string {
  if (!row.request_time_count || row.request_time_count === 0) return '-'
  return (row.request_time_sum / row.request_time_count).toFixed(1) + ' ms'
}

const columns = computed(() => [
  { title: 'URI', key: 'uri', ellipsis: { tooltip: true } },
  {
    title: $gettext('Requests'),
    key: 'requests',
    sorter: (a: any, b: any) => a.requests - b.requests,
  },
  {
    title: $gettext('Avg Response Time'),
    key: 'avg_time',
    render: (row: any) => avgTime(row),
    sorter: (a: any, b: any) => {
      const aAvg = a.request_time_count > 0 ? a.request_time_sum / a.request_time_count : 0
      const bAvg = b.request_time_count > 0 ? b.request_time_sum / b.request_time_count : 0
      return aAvg - bAvg
    },
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
  <n-flex vertical :size="16">
    <n-flex align="center" :size="8">
      <span class="text-14px">{{ $gettext('Threshold') }}</span>
      <n-input-number
        v-model:value="threshold"
        :min="0"
        :step="50"
        size="small"
        class="w-35"
        @update:value="reload"
      >
        <template #suffix>ms</template>
      </n-input-number>
    </n-flex>
    <n-spin :show="loading">
      <n-data-table
        :columns="columns"
        :data="items"
        :bordered="false"
        size="small"
        :scroll-x="700"
      />
      <n-flex justify="end" class="mt-3" v-if="total > 0">
        <n-pagination v-bind="pagination" />
      </n-flex>
    </n-spin>
  </n-flex>
</template>

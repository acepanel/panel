<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import { formatBytes } from '@/utils/file'

const { $gettext } = useGettext()

const ctx = inject<any>('statContext')!

const loading = ref(false)
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(50)
const threshold = ref<number | null>(100)

const loadData = () => {
  loading.value = true
  useRequest(
    website.statSlowURIs(
      ctx.dateRange.value.start,
      ctx.dateRange.value.end,
      ctx.sitesParam.value,
      threshold.value && threshold.value > 0 ? threshold.value : undefined,
      page.value,
      limit.value,
    ),
  )
    .onSuccess(({ data }: any) => {
      items.value = data.items || []
      total.value = data.total || 0
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch([() => ctx.dateRange.value, () => ctx.sitesParam.value, () => ctx.refreshKey.value], () => {
  page.value = 1
  loadData()
})

onMounted(() => {
  loadData()
})

const handleThresholdChange = () => {
  page.value = 1
  loadData()
}

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

const handlePageChange = (p: number) => {
  page.value = p
  loadData()
}

const handlePageSizeChange = (s: number) => {
  limit.value = s
  page.value = 1
  loadData()
}
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
        @update:value="handleThresholdChange"
      >
        <template #suffix>ms</template>
      </n-input-number>
    </n-flex>
    <n-spin :show="loading">
      <n-data-table :columns="columns" :data="items" :bordered="false" size="small" />
      <n-flex justify="end" class="mt-3" v-if="total > 0">
        <n-pagination
          v-model:page="page"
          :page-size="limit"
          :item-count="total"
          :page-sizes="[20, 50, 100]"
          show-size-picker
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </n-flex>
    </n-spin>
  </n-flex>
</template>

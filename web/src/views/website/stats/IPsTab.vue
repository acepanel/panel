<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import { formatBytes } from '@/utils/file'

import { codeToName } from './country-name-map'

const { $gettext } = useGettext()

use([CanvasRenderer, BarChart, TooltipComponent, GridComponent])

const ctx = inject<any>('statContext')!

const loading = ref(false)
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(50)

// ISP 分布数据
const ispItems = ref<any[]>([])

const loadData = () => {
  loading.value = true
  useRequest(
    website.statIPs(
      ctx.dateRange.value.start,
      ctx.dateRange.value.end,
      ctx.sitesParam.value,
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

const loadISPs = () => {
  useRequest(
    website.statGeos(
      ctx.dateRange.value.start,
      ctx.dateRange.value.end,
      ctx.sitesParam.value,
      'isp',
      undefined,
      15,
    ),
  ).onSuccess(({ data }: any) => {
    ispItems.value = data.items || []
  })
}

watch([() => ctx.dateRange.value, () => ctx.sitesParam.value, () => ctx.refreshKey.value], () => {
  page.value = 1
  loadData()
  loadISPs()
})

onMounted(() => {
  loadData()
  loadISPs()
})

// ========== ISP 分布柱状图 ==========

const ispBarOption = computed<EChartsOption>(() => {
  const sorted = ispItems.value.filter((i: any) => i.country)
  const reversed = [...sorted].reverse()

  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const idx = sorted.length - 1 - params[0].dataIndex
        const item = sorted[idx]
        if (!item) return ''
        return `${params[0].name}<br/>${$gettext('Requests')}: ${params[0].value.toLocaleString()}<br/>${$gettext('Bandwidth')}: ${formatBytes(item.bandwidth)}`
      },
    },
    grid: { left: 100, right: 40, top: 10, bottom: 30 },
    xAxis: { type: 'value' },
    yAxis: {
      type: 'category',
      data: reversed.map((i: any) => i.country || $gettext('Unknown')),
      axisLabel: { width: 80, overflow: 'truncate' },
    },
    series: [
      {
        type: 'bar',
        data: reversed.map((i: any) => i.requests),
        barMaxWidth: 30,
      },
    ],
  }
})

// ========== 表格 ==========

const formatGeo = (row: any) => {
  const country = codeToName[row.country] || row.country
  const parts = [country, row.region, row.city].filter((s) => s && s !== '')
  return parts.join(' ') || '-'
}

const columns = computed(() => [
  { title: 'IP', key: 'ip', ellipsis: { tooltip: true } },
  {
    title: $gettext('Location'),
    key: 'country',
    render: (row: any) => formatGeo(row),
    ellipsis: { tooltip: true },
  },
  {
    title: 'ISP',
    key: 'isp',
    render: (row: any) => row.isp || '-',
    ellipsis: { tooltip: true },
  },
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
  <n-flex vertical :size="20">
    <n-spin :show="loading">
      <n-card :bordered="false" :title="$gettext('ISP Distribution')" v-if="ispItems.length > 0">
        <v-chart class="h-300px" :option="ispBarOption" autoresize />
      </n-card>
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

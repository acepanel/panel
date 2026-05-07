<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { BarChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'

const { $gettext } = useGettext()

use([CanvasRenderer, BarChart, TooltipComponent, LegendComponent, GridComponent])

const ctx = inject<any>('statContext')!

const loading = ref(false)
const items = ref<any[]>([])
const total = ref(0)

const loadData = () => {
  loading.value = true
  useRequest(
    website.statSpiders(ctx.dateRange.value.start, ctx.dateRange.value.end, ctx.sitesParam.value),
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
  loadData()
})

onMounted(() => {
  loadData()
})

const chartOption = computed<EChartsOption>(() => {
  const data = items.value.slice(0, 20)
  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const p = params[0]
        const item = data[p.dataIndex]
        return `<div style="font-size:12px"><b>${p.name}</b><br/>${$gettext('Requests')}: ${p.value.toLocaleString()}<br/>${$gettext('Percentage')}: ${item?.percent?.toFixed(1) || 0}%</div>`
      },
    },
    grid: { left: 120, right: 40, top: 10, bottom: 30 },
    xAxis: { type: 'value' },
    yAxis: {
      type: 'category',
      data: data.map((i: any) => i.spider).reverse(),
      axisLabel: { width: 100, overflow: 'truncate' },
    },
    series: [
      {
        type: 'bar',
        data: data.map((i: any) => i.requests).reverse(),
        barMaxWidth: 30,
      },
    ],
  }
})

const columns = computed(() => [
  { title: $gettext('Spider'), key: 'spider' },
  {
    title: $gettext('Requests'),
    key: 'requests',
    sorter: (a: any, b: any) => a.requests - b.requests,
  },
  {
    title: $gettext('Percentage'),
    key: 'percent',
    render: (row: any) => `${(row.percent || 0).toFixed(1)}%`,
  },
])
</script>

<template>
  <n-flex vertical :size="20">
    <n-spin :show="loading">
      <n-card :bordered="false" v-if="items.length > 0">
        <v-chart class="h-400px" :option="chartOption" autoresize />
      </n-card>
      <n-data-table :columns="columns" :data="items" :bordered="false" size="small" />
    </n-spin>
  </n-flex>
</template>

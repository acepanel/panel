<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'

const { $gettext } = useGettext()

use([CanvasRenderer, LineChart, TooltipComponent, LegendComponent, GridComponent])

const ctx = inject<any>('statContext')!

const loading = ref(false)

interface StatTotals {
  status_2xx: number
  status_3xx: number
  status_4xx: number
  status_5xx: number
}

interface SeriesPoint {
  key: string
  status_2xx: number
  status_3xx: number
  status_4xx: number
  status_5xx: number
}

const totals = ref<StatTotals>({ status_2xx: 0, status_3xx: 0, status_4xx: 0, status_5xx: 0 })
const series = ref<SeriesPoint[]>([])

const isSingleDay = computed(() => ctx.dateRange.value.start === ctx.dateRange.value.end)

const loadData = () => {
  loading.value = true
  useRequest(
    website.statOverview(ctx.dateRange.value.start, ctx.dateRange.value.end, ctx.sitesParam.value),
  )
    .onSuccess(({ data }: any) => {
      totals.value = data.current
      series.value = data.series || []
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

// 汇总
const totalRequests = computed(() => {
  const t = totals.value
  return (t.status_2xx || 0) + (t.status_3xx || 0) + (t.status_4xx || 0) + (t.status_5xx || 0)
})

function pct(value: number): string {
  if (totalRequests.value === 0) return '0%'
  return ((value / totalRequests.value) * 100).toFixed(1) + '%'
}

const cards = computed(() => [
  { label: '2xx', value: totals.value.status_2xx || 0, color: '#18a058' },
  { label: '3xx', value: totals.value.status_3xx || 0, color: '#2080f0' },
  { label: '4xx', value: totals.value.status_4xx || 0, color: '#f0a020' },
  { label: '5xx', value: totals.value.status_5xx || 0, color: '#d03050' },
])

// 图表
function formatXLabel(key: string): string {
  if (isSingleDay.value) {
    return `${key.padStart(2, '0')}:00`
  }
  return key.slice(5)
}

const chartOption = computed<EChartsOption>(() => {
  const data = series.value
  const xData = data.map((s) => formatXLabel(s.key))

  return {
    tooltip: {
      trigger: 'axis',
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        let total = 0
        for (const p of params) total += p.value || 0
        let html = `<div style="font-size:12px"><div style="margin-bottom:4px;font-weight:bold">${params[0].name}</div>`
        for (const p of params) {
          const val = (p.value || 0).toLocaleString()
          const ratio = total > 0 ? ((p.value / total) * 100).toFixed(1) + '%' : '0%'
          html += `<div>${p.marker} ${p.seriesName}: ${val} (${ratio})</div>`
        }
        html += '</div>'
        return html
      },
    },
    legend: { top: 0, right: 0 },
    grid: { left: 60, right: 20, top: 40, bottom: 30 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: xData,
      axisLabel: {
        interval: isSingleDay.value
          ? (_: number, value: string) => {
              const hour = parseInt(value.split(':')[0] || '0', 10)
              return hour % 3 === 0
            }
          : 'auto',
      },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { type: 'dashed' } },
    },
    series: [
      {
        name: '2xx',
        type: 'line',
        stack: 'status',
        smooth: true,
        symbol: 'none',
        areaStyle: { opacity: 0.6 },
        lineStyle: { width: 1 },
        itemStyle: { color: '#18a058' },
        data: data.map((s) => s.status_2xx || 0),
      },
      {
        name: '3xx',
        type: 'line',
        stack: 'status',
        smooth: true,
        symbol: 'none',
        areaStyle: { opacity: 0.6 },
        lineStyle: { width: 1 },
        itemStyle: { color: '#2080f0' },
        data: data.map((s) => s.status_3xx || 0),
      },
      {
        name: '4xx',
        type: 'line',
        stack: 'status',
        smooth: true,
        symbol: 'none',
        areaStyle: { opacity: 0.6 },
        lineStyle: { width: 1 },
        itemStyle: { color: '#f0a020' },
        data: data.map((s) => s.status_4xx || 0),
      },
      {
        name: '5xx',
        type: 'line',
        stack: 'status',
        smooth: true,
        symbol: 'none',
        areaStyle: { opacity: 0.6 },
        lineStyle: { width: 1 },
        itemStyle: { color: '#d03050' },
        data: data.map((s) => s.status_5xx || 0),
      },
    ],
  }
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-spin :show="loading">
      <div class="gap-3 grid grid-cols-2 lg:grid-cols-4">
        <n-card v-for="c in cards" :key="c.label" :bordered="false" size="small">
          <div class="flex flex-col gap-1">
            <span class="text-12px" :style="{ color: c.color }">{{ c.label }}</span>
            <span class="text-20px font-bold">{{ c.value.toLocaleString() }}</span>
            <span class="text-12px text-[var(--text-color-3)]">{{ pct(c.value) }}</span>
          </div>
        </n-card>
      </div>
    </n-spin>

    <n-card :bordered="false">
      <template #header>{{ $gettext('Status Codes') }}</template>
      <n-spin :show="loading">
        <v-chart class="h-350px" :option="chartOption" autoresize />
      </n-spin>
    </n-card>
  </n-flex>
</template>

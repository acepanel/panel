<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { BarChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'

const { $gettext } = useGettext()

use([CanvasRenderer, BarChart, PieChart, TooltipComponent, LegendComponent, GridComponent])

const ctx = inject<any>('statContext')!

const loading = ref(false)
const browsers = ref<any[]>([])
const oss = ref<any[]>([])
const items = ref<any[]>([])

const loadData = () => {
  loading.value = true
  useRequest(
    website.statClients(ctx.dateRange.value.start, ctx.dateRange.value.end, ctx.sitesParam.value),
  )
    .onSuccess(({ data }: any) => {
      browsers.value = data.browsers || []
      oss.value = data.os || []
      items.value = data.items || []
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

const browserChartOption = computed<EChartsOption>(() => {
  const data = browsers.value.slice(0, 10)
  return {
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)',
    },
    legend: { orient: 'vertical', right: 10, top: 'center' },
    series: [
      {
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['35%', '50%'],
        avoidLabelOverlap: false,
        label: { show: false },
        data: data.map((i: any) => ({ name: i.name, value: i.requests })),
      },
    ],
  }
})

const osChartOption = computed<EChartsOption>(() => {
  const data = oss.value.slice(0, 15)
  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
    },
    grid: { left: 100, right: 40, top: 10, bottom: 30 },
    xAxis: { type: 'value' },
    yAxis: {
      type: 'category',
      data: data.map((i: any) => i.name).reverse(),
      axisLabel: { width: 80, overflow: 'truncate' },
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
  { title: $gettext('Browser'), key: 'browser' },
  { title: $gettext('OS'), key: 'os' },
  {
    title: $gettext('Requests'),
    key: 'requests',
    sorter: (a: any, b: any) => a.requests - b.requests,
  },
])
</script>

<template>
  <n-flex vertical :size="20">
    <n-spin :show="loading">
      <div class="gap-3 grid grid-cols-1 lg:grid-cols-2" v-if="browsers.length > 0">
        <n-card :bordered="false" :title="$gettext('Browsers')">
          <v-chart class="h-300px" :option="browserChartOption" autoresize />
        </n-card>
        <n-card :bordered="false" :title="$gettext('OS')">
          <v-chart class="h-300px" :option="osChartOption" autoresize />
        </n-card>
      </div>
      <n-data-table :columns="columns" :data="items" :bordered="false" size="small" />
    </n-spin>
  </n-flex>
</template>

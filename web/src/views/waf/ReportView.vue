<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { BarChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import waf from '@/api/panel/waf'

const { $gettext } = useGettext()

use([CanvasRenderer, BarChart, PieChart, TooltipComponent, GridComponent, LegendComponent])

const loading = ref(false)
const rangeDays = ref(7)
const stats = ref<any>({ total: 0, by_action: [], by_severity: [], top_ip: [] })

const rangeOptions = computed(() => [
  { label: $gettext('Last 24 hours'), value: 1 },
  { label: $gettext('Last 7 days'), value: 7 },
  { label: $gettext('Last 30 days'), value: 30 },
])

const since = computed(() => Math.floor(Date.now() / 1000) - rangeDays.value * 86400)

const loadData = () => {
  loading.value = true
  useRequest(waf.stats(since.value))
    .onSuccess(({ data }: any) => {
      stats.value = {
        total: data?.total || 0,
        by_action: data?.by_action || [],
        by_severity: data?.by_severity || [],
        top_ip: data?.top_ip || [],
      }
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch(rangeDays, () => loadData())

onMounted(() => loadData())

// 按动作统计（用于统计卡片）
const actionCount = (action: string): number => {
  const item = stats.value.by_action.find((i: any) => i.key === action)
  return item ? item.count : 0
}

// 动作分布饼图
const actionPieOption = computed<EChartsOption>(() => ({
  tooltip: { trigger: 'item' },
  legend: { bottom: 0 },
  series: [
    {
      type: 'pie',
      radius: ['40%', '65%'],
      data: stats.value.by_action.map((i: any) => ({ name: i.key, value: i.count })),
    },
  ],
}))

// 严重度分布柱状图
const severityBarOption = computed<EChartsOption>(() => {
  const data = [...stats.value.by_severity].sort((a: any, b: any) => Number(a.key) - Number(b.key))
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 40, right: 20, top: 20, bottom: 30 },
    xAxis: {
      type: 'category',
      data: data.map((i: any) => `${$gettext('Level')} ${i.key}`),
    },
    yAxis: { type: 'value' },
    series: [{ type: 'bar', data: data.map((i: any) => i.count), barMaxWidth: 40 }],
  }
})

const topIpColumns: any = [
  { title: $gettext('Client IP'), key: 'key', minWidth: 160 },
  {
    title: $gettext('Attacks'),
    key: 'count',
    width: 120,
    sorter: (a: any, b: any) => a.count - b.count,
  },
]
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex align="center">
      <n-select v-model:value="rangeDays" :options="rangeOptions" class="w-40" />
      <n-button @click="loadData">{{ $gettext('Refresh') }}</n-button>
    </n-flex>
    <n-spin :show="loading">
      <!-- 总览卡片 -->
      <n-grid :cols="24" :x-gap="16" :y-gap="16">
        <n-gi :span="6">
          <n-card :bordered="false">
            <n-statistic :label="$gettext('Total Events')" :value="stats.total" />
          </n-card>
        </n-gi>
        <n-gi :span="6">
          <n-card :bordered="false">
            <n-statistic :label="$gettext('Blocked')" :value="actionCount('block')" />
          </n-card>
        </n-gi>
        <n-gi :span="6">
          <n-card :bordered="false">
            <n-statistic :label="$gettext('Challenged')" :value="actionCount('challenge')" />
          </n-card>
        </n-gi>
        <n-gi :span="6">
          <n-card :bordered="false">
            <n-statistic :label="$gettext('Banned')" :value="actionCount('ban')" />
          </n-card>
        </n-gi>
      </n-grid>

      <!-- 图表区 -->
      <n-grid :cols="24" :x-gap="16" :y-gap="16" class="mt-4">
        <n-gi :span="12">
          <n-card :bordered="false" :title="$gettext('Action Distribution')">
            <v-chart
              v-if="stats.by_action.length > 0"
              class="h-300px"
              :option="actionPieOption"
              autoresize
            />
            <n-empty v-else :description="$gettext('No data')" class="my-10" />
          </n-card>
        </n-gi>
        <n-gi :span="12">
          <n-card :bordered="false" :title="$gettext('Severity Distribution')">
            <v-chart
              v-if="stats.by_severity.length > 0"
              class="h-300px"
              :option="severityBarOption"
              autoresize
            />
            <n-empty v-else :description="$gettext('No data')" class="my-10" />
          </n-card>
        </n-gi>
      </n-grid>

      <!-- Top 攻击源 -->
      <n-card :bordered="false" :title="$gettext('Top Attack Source IPs')" class="mt-4">
        <n-data-table
          :columns="topIpColumns"
          :data="stats.top_ip"
          size="small"
          :bordered="false"
        />
      </n-card>
    </n-spin>
  </n-flex>
</template>

<style scoped lang="scss"></style>

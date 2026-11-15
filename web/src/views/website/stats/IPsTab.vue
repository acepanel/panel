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
import { useStatQuery } from './useStatQuery'

const { $gettext } = useGettext()

use([CanvasRenderer, BarChart, TooltipComponent, GridComponent])

const { loading, items, total, pagination } = useStatQuery(({ start, end, sites, page, limit }) =>
  website.statIPs(start, end, sites, page, limit),
)

// ISP 分布数据
const { items: ispItems } = useStatQuery(({ start, end, sites }) =>
  website.statGeos(start, end, sites, 'isp', undefined, 15),
)

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
</script>

<template>
  <n-flex vertical :size="20">
    <n-spin :show="loading">
      <n-card :bordered="false" :title="$gettext('ISP Distribution')" v-if="ispItems.length > 0">
        <v-chart class="h-300px" :option="ispBarOption" autoresize />
      </n-card>
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

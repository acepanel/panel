<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { BarChart, MapChart } from 'echarts/charts'
import {
  GeoComponent,
  GridComponent,
  TooltipComponent,
  VisualMapComponent,
} from 'echarts/components'
import { registerMap, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import { useThemeStore } from '@/stores'
import { formatBytes } from '@/utils/file'

import { codeToGeoName, codeToName } from './country-name-map'

const { $gettext } = useGettext()
const themeStore = useThemeStore()

use([
  CanvasRenderer,
  BarChart,
  MapChart,
  TooltipComponent,
  GridComponent,
  VisualMapComponent,
  GeoComponent,
])

const ctx = inject<any>('statContext')!

const loading = ref(false)
const items = ref<any[]>([])
const groupBy = ref('country')
const mapReady = ref(false)

// 懒加载世界地图 GeoJSON
const loadMap = async () => {
  if (mapReady.value) return
  try {
    const resp = await fetch('/data/world.json')
    const geoJson = await resp.json()
    registerMap('world', geoJson as any)
    mapReady.value = true
  } catch (e) {
    console.warn('Failed to load world map:', e)
  }
}

const loadData = () => {
  loading.value = true
  useRequest(
    website.statGeos(
      ctx.dateRange.value.start,
      ctx.dateRange.value.end,
      ctx.sitesParam.value,
      groupBy.value,
      undefined,
      100,
    ),
  )
    .onSuccess(({ data }: any) => {
      items.value = data.items || []
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch([() => ctx.dateRange.value, () => ctx.sitesParam.value, () => ctx.refreshKey.value], () => {
  loadData()
})

watch(groupBy, () => {
  loadData()
})

onMounted(() => {
  loadMap()
  loadData()
})

// ========== 表格 ==========

const nameColumn = computed(() => {
  return groupBy.value === 'region'
    ? { title: $gettext('Region'), key: 'region' }
    : { title: $gettext('Country'), key: 'country' }
})

const columns = computed(() => [
  {
    title: nameColumn.value.title,
    key: nameColumn.value.key,
    render: (row: any) => {
      const val = row[nameColumn.value.key]
      if (nameColumn.value.key === 'country') {
        return codeToName[val] || val || $gettext('Unknown')
      }
      return val || $gettext('Unknown')
    },
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

// ========== 图表 ==========

const chartTitle = computed(() => {
  return groupBy.value === 'region'
    ? $gettext('Region Distribution')
    : $gettext('Country Distribution')
})

// 将 ISO 国家代码转为 GeoJSON 英文名
const toGeoName = (code: string): string => codeToGeoName[code] || code

// 构建国家请求数映射（用于 tooltip 查带宽）
const countryDataMap = computed(() => {
  const map = new Map<string, any>()
  for (const item of items.value) {
    const name = toGeoName(item.country)
    const existing = map.get(name)
    if (existing) {
      existing.requests += item.requests
      existing.bandwidth += item.bandwidth
    } else {
      map.set(name, { ...item, geoName: name })
    }
  }
  return map
})

// 世界地图配置
const mapChartOption = computed<EChartsOption>(() => {
  const isDark = themeStore.darkMode
  const data = Array.from(countryDataMap.value.values()).map((item) => ({
    name: item.geoName,
    value: item.requests,
    bandwidth: item.bandwidth,
    originalName: codeToName[item.country] || item.country,
  }))

  const maxValue = data.reduce((max, d) => Math.max(max, d.value), 0)

  return {
    tooltip: {
      trigger: 'item',
      formatter: (params: any) => {
        if (!params.data?.value) return `${params.name}: ${$gettext('No data')}`
        const originalName = params.data.originalName || params.name
        return `${originalName}<br/>${$gettext('Requests')}: ${params.data.value.toLocaleString()}<br/>${$gettext('Bandwidth')}: ${formatBytes(params.data.bandwidth)}`
      },
    },
    visualMap: {
      min: 0,
      max: maxValue || 100,
      left: 'left',
      bottom: 20,
      calculable: true,
      inRange: {
        color: isDark
          ? ['#1a3a4a', '#2a6a8a', '#3a9aca', '#5ac0ea', '#8ae0ff']
          : ['#e0f3ff', '#a0d4f0', '#60b0e0', '#2090d0', '#0070c0'],
      },
      textStyle: {
        color: isDark ? '#ccc' : '#333',
      },
    },
    series: [
      {
        type: 'map',
        map: 'world',
        layoutCenter: ['50%', '50%'],
        layoutSize: '180%',
        roam: true,
        emphasis: {
          label: { show: true },
          itemStyle: { areaColor: isDark ? '#4a8aaa' : '#3399cc' },
        },
        itemStyle: {
          areaColor: isDark ? '#2a2a3a' : '#e9ecef',
          borderColor: isDark ? '#444' : '#aaa',
        },
        data,
      },
    ],
  }
})

// 条形图配置（region 模式使用）
const barChartOption = computed<EChartsOption>(() => {
  const data = items.value.slice(0, 15)
  const reversed = [...data].reverse()
  const key = nameColumn.value.key
  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const idx = data.length - 1 - params[0].dataIndex
        const item = data[idx]
        if (!item) return ''
        return `${params[0].name}<br/>${$gettext('Requests')}: ${params[0].value.toLocaleString()}<br/>${$gettext('Bandwidth')}: ${formatBytes(item.bandwidth)}`
      },
    },
    grid: { left: 100, right: 40, top: 10, bottom: 30 },
    xAxis: { type: 'value' },
    yAxis: {
      type: 'category',
      data: reversed.map((i: any) => i[key] || $gettext('Unknown')),
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

const showMap = computed(() => groupBy.value === 'country' && mapReady.value)
</script>

<template>
  <n-flex vertical :size="20">
    <n-spin :show="loading">
      <!-- 维度切换 -->
      <n-flex align="center" class="mb-3">
        <n-radio-group v-model:value="groupBy" size="small">
          <n-radio-button value="country">{{ $gettext('Country') }}</n-radio-button>
          <n-radio-button value="region">{{ $gettext('Region') }}</n-radio-button>
        </n-radio-group>
      </n-flex>

      <!-- 世界地图（country 模式） -->
      <n-card :bordered="false" :title="chartTitle" v-if="showMap && items.length > 0">
        <v-chart class="h-400px" :option="mapChartOption" autoresize />
      </n-card>

      <!-- 条形图（region 模式） -->
      <n-card :bordered="false" :title="chartTitle" v-if="!showMap && items.length > 0">
        <v-chart class="h-300px" :option="barChartOption" autoresize />
      </n-card>

      <!-- 表格 -->
      <n-data-table :columns="columns" :data="items" :bordered="false" size="small" />
    </n-spin>
  </n-flex>
</template>

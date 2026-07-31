<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { MapChart } from 'echarts/charts'
import { GeoComponent, TooltipComponent, VisualMapComponent } from 'echarts/components'
import { registerMap, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import waf from '@/api/panel/waf'
import { useThemeStore } from '@/stores'
import { codeToGeoName, codeToName } from '@/views/website/stats/country-name-map'

const { $gettext } = useGettext()
const themeStore = useThemeStore()

use([CanvasRenderer, MapChart, TooltipComponent, VisualMapComponent, GeoComponent])

const loading = ref(false)
const items = ref<any[]>([])
const mapReady = ref(false)
const rangeDays = ref(7)

const rangeOptions = computed(() => [
  { label: $gettext('Last 24 hours'), value: 1 },
  { label: $gettext('Last 7 days'), value: 7 },
  { label: $gettext('Last 30 days'), value: 30 },
])

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

const since = computed(() => Math.floor(Date.now() / 1000) - rangeDays.value * 86400)

const loadData = () => {
  loading.value = true
  useRequest(waf.attackMap(since.value))
    .onSuccess(({ data }: any) => {
      // agent /api/attack-map 返回 { points, top_country, generated_at }，top_country 为 [{key,count}]
      items.value = data?.top_country || []
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch(rangeDays, () => loadData())

onMounted(() => {
  loadMap()
  loadData()
})

const toGeoName = (code: string): string => codeToGeoName[code] || code

const mapOption = computed<EChartsOption>(() => {
  const isDark = themeStore.darkMode
  const data = items.value
    .filter((i: any) => i.key)
    .map((i: any) => ({
      name: toGeoName(i.key),
      value: i.count,
      originalName: codeToName[i.key] || i.key,
    }))
  const maxValue = data.reduce((max: number, d: any) => Math.max(max, d.value), 0)

  return {
    tooltip: {
      trigger: 'item',
      formatter: (params: any) => {
        if (!params.data?.value) return `${params.name}: ${$gettext('No data')}`
        const name = params.data.originalName || params.name
        return `${name}<br/>${$gettext('Attacks')}: ${params.data.value.toLocaleString()}`
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
          ? ['#3a1a1a', '#6a2a2a', '#ca3a3a', '#ea5a5a', '#ff8a8a']
          : ['#ffe0e0', '#f0a0a0', '#e06060', '#d02020', '#c00000'],
      },
      textStyle: { color: isDark ? '#ccc' : '#333' },
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
          itemStyle: { areaColor: isDark ? '#aa4a4a' : '#cc3333' },
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

const tableColumns: any = [
  {
    title: $gettext('Country'),
    key: 'key',
    render: (row: any) => codeToName[row.key] || row.key || $gettext('Unknown'),
  },
  {
    title: $gettext('Attacks'),
    key: 'count',
    sorter: (a: any, b: any) => a.count - b.count,
  },
]

// 按攻击数倒序展示
const sortedItems = computed(() =>
  [...items.value].sort((a: any, b: any) => b.count - a.count)
)
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex align="center">
      <n-select v-model:value="rangeDays" :options="rangeOptions" class="w-40" />
      <n-button @click="loadData">{{ $gettext('Refresh') }}</n-button>
    </n-flex>
    <n-spin :show="loading">
      <n-card v-if="mapReady && items.length > 0" :bordered="false" :title="$gettext('Attack Map')">
        <v-chart class="h-450px" :option="mapOption" autoresize />
      </n-card>
      <n-empty
        v-else-if="!loading && items.length === 0"
        :description="$gettext('No attack data')"
        class="my-10"
      />
      <n-card :bordered="false" :title="$gettext('Top Attack Sources')" class="mt-4">
        <n-data-table :columns="tableColumns" :data="sortedItems" size="small" :bordered="false" />
      </n-card>
    </n-spin>
  </n-flex>
</template>

<style scoped lang="scss"></style>

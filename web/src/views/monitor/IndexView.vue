<script setup lang="ts">
defineOptions({
  name: 'monitor-index',
})

import type { EChartsOption } from 'echarts'
import { LineChart } from 'echarts/charts'
import {
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
} from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import monitor from '@/api/panel/monitor'

const { $gettext } = useGettext()

use([
  CanvasRenderer,
  LineChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DataZoomComponent,
])

// 监控设置
const monitorSwitch = ref(false)
const saveDay = ref(30)
const monitorInterval = ref(1)
const updateLoading = ref(false)

useRequest(monitor.setting()).onSuccess(({ data }) => {
  monitorSwitch.value = data.enabled
  saveDay.value = data.days
  monitorInterval.value = data.interval
})

// 时间预设选项
type TimePreset = 'yesterday' | 'today' | 'week' | 'custom'

interface TimeRange {
  start: number
  end: number
  preset: TimePreset
  customRange: [number, number] | null
}

// 获取时间范围
function getTimeRange(
  preset: TimePreset,
  customRange?: [number, number],
): { start: number; end: number } {
  const now = new Date()
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()

  switch (preset) {
    case 'yesterday': {
      const yesterdayStart = todayStart - 24 * 60 * 60 * 1000
      return { start: yesterdayStart, end: todayStart }
    }
    case 'today':
      return { start: todayStart, end: Date.now() }
    case 'week': {
      const weekStart = todayStart - 7 * 24 * 60 * 60 * 1000
      return { start: weekStart, end: Date.now() }
    }
    case 'custom':
      if (customRange) {
        return { start: customRange[0], end: customRange[1] }
      }
      return { start: todayStart, end: Date.now() }
    default:
      return { start: todayStart, end: Date.now() }
  }
}

// 各图表的时间范围
const loadTime = ref<TimeRange>({ start: 0, end: 0, preset: 'today', customRange: null })
const cpuTime = ref<TimeRange>({ start: 0, end: 0, preset: 'today', customRange: null })
const memTime = ref<TimeRange>({ start: 0, end: 0, preset: 'today', customRange: null })
const netTime = ref<TimeRange>({ start: 0, end: 0, preset: 'today', customRange: null })
const diskIOTime = ref<TimeRange>({ start: 0, end: 0, preset: 'today', customRange: null })

// 初始化时间范围
function initTimeRanges() {
  const todayRange = getTimeRange('today')
  loadTime.value = { ...todayRange, preset: 'today', customRange: null }
  cpuTime.value = { ...todayRange, preset: 'today', customRange: null }
  memTime.value = { ...todayRange, preset: 'today', customRange: null }
  netTime.value = { ...todayRange, preset: 'today', customRange: null }
  diskIOTime.value = { ...todayRange, preset: 'today', customRange: null }
}
initTimeRanges()

// 更新时间范围
function updateTimeRange(
  key: 'load' | 'cpu' | 'mem' | 'net' | 'diskIO',
  preset: TimePreset,
  customRange?: [number, number],
) {
  const range = getTimeRange(preset, customRange)
  const newValue = { ...range, preset, customRange: customRange || null }

  switch (key) {
    case 'load':
      loadTime.value = newValue
      break
    case 'cpu':
      cpuTime.value = newValue
      break
    case 'mem':
      memTime.value = newValue
      break
    case 'net':
      netTime.value = newValue
      break
    case 'diskIO':
      diskIOTime.value = newValue
      break
  }
}

// 自定义时间 popover 状态
const loadCustomPopover = ref(false)
const cpuCustomPopover = ref(false)
const memCustomPopover = ref(false)
const netCustomPopover = ref(false)
const diskIOCustomPopover = ref(false)

// 临时时间范围
const loadTempRange = ref<[number, number] | null>(null)
const cpuTempRange = ref<[number, number] | null>(null)
const memTempRange = ref<[number, number] | null>(null)
const netTempRange = ref<[number, number] | null>(null)
const diskIOTempRange = ref<[number, number] | null>(null)

function confirmCustomTime(key: 'load' | 'cpu' | 'mem' | 'net' | 'diskIO') {
  switch (key) {
    case 'load':
      if (loadTempRange.value) {
        updateTimeRange('load', 'custom', loadTempRange.value)
      }
      loadCustomPopover.value = false
      break
    case 'cpu':
      if (cpuTempRange.value) {
        updateTimeRange('cpu', 'custom', cpuTempRange.value)
      }
      cpuCustomPopover.value = false
      break
    case 'mem':
      if (memTempRange.value) {
        updateTimeRange('mem', 'custom', memTempRange.value)
      }
      memCustomPopover.value = false
      break
    case 'net':
      if (netTempRange.value) {
        updateTimeRange('net', 'custom', netTempRange.value)
      }
      netCustomPopover.value = false
      break
    case 'diskIO':
      if (diskIOTempRange.value) {
        updateTimeRange('diskIO', 'custom', diskIOTempRange.value)
      }
      diskIOCustomPopover.value = false
      break
  }
}

// 数据请求
interface ProcessStat {
  pid: number
  name: string
  username: string
  command: string
  value: number
  read?: number
  write?: number
}

interface TopProcesses {
  cpu: ProcessStat[]
  memory: ProcessStat[]
  disk_io: ProcessStat[]
}

interface MonitorData {
  times: string[]
  load: { load1: number[]; load5: number[]; load15: number[] }
  cpu: { percent: string[] }
  mem: { total: string; available: string[]; used: string[] }
  swap: { total: string; used: string[]; free: string[] }
  net: Array<{ name: string; sent: string[]; recv: string[]; tx: string[]; rx: string[] }>
  disk_io: Array<{
    name: string
    read_bytes: string[]
    write_bytes: string[]
    read_speed: string[]
    write_speed: string[]
  }>
  top_processes: TopProcesses[]
}

const emptyData: MonitorData = {
  times: [],
  load: { load1: [], load5: [], load15: [] },
  cpu: { percent: [] },
  mem: { total: '0', available: [], used: [] },
  swap: { total: '0', used: [], free: [] },
  net: [],
  disk_io: [],
  top_processes: [],
}

// 各图表数据
const { loading: loadLoading, data: loadData } = useWatcher(
  () => monitor.list(loadTime.value.start, loadTime.value.end),
  [loadTime],
  { initialData: emptyData, debounce: [300], immediate: true },
)

const { loading: cpuLoading, data: cpuData } = useWatcher(
  () => monitor.list(cpuTime.value.start, cpuTime.value.end),
  [cpuTime],
  { initialData: emptyData, debounce: [300], immediate: true },
)

const { loading: memLoading, data: memData } = useWatcher(
  () => monitor.list(memTime.value.start, memTime.value.end),
  [memTime],
  { initialData: emptyData, debounce: [300], immediate: true },
)

const { loading: netLoading, data: netData } = useWatcher(
  () => monitor.list(netTime.value.start, netTime.value.end),
  [netTime],
  { initialData: emptyData, debounce: [300], immediate: true },
)

const { loading: diskIOLoading, data: diskIOData } = useWatcher(
  () => monitor.list(diskIOTime.value.start, diskIOTime.value.end),
  [diskIOTime],
  { initialData: emptyData, debounce: [300], immediate: true },
)

// 网卡和磁盘筛选
const selectedNetDevices = ref<string[]>([])
const selectedDisks = ref<string[]>([])

// 可用的网卡和磁盘列表
const availableNetDevices = computed(() => {
  return netData.value?.net?.map((d: { name: string }) => d.name) || []
})

const availableDisks = computed(() => {
  return diskIOData.value?.disk_io?.map((d: { name: string }) => d.name) || []
})

// 当数据加载完成后，默认选中所有设备
watch(availableNetDevices, (devices) => {
  if (devices.length > 0 && selectedNetDevices.value.length === 0) {
    selectedNetDevices.value = [...devices]
  }
})

watch(availableDisks, (disks) => {
  if (disks.length > 0 && selectedDisks.value.length === 0) {
    selectedDisks.value = [...disks]
  }
})

// 格式化字节速率 (KB/s -> MB/s -> GB/s)
function formatSpeed(value: number | string): string {
  const num = typeof value === 'string' ? parseFloat(value) : value
  if (isNaN(num)) return '0 KB/s'
  if (num >= 1024 * 1024) {
    return `${(num / 1024 / 1024).toFixed(2)} GB/s`
  } else if (num >= 1024) {
    return `${(num / 1024).toFixed(2)} MB/s`
  }
  return `${num.toFixed(2)} KB/s`
}

// 格式化内存大小 (MB -> GB)
function formatMemory(value: number | string): string {
  const num = typeof value === 'string' ? parseFloat(value) : value
  if (isNaN(num)) return '0 MB'
  if (num >= 1024) {
    return `${(num / 1024).toFixed(2)} GB`
  }
  return `${num.toFixed(2)} MB`
}

// 格式化字节大小 (B -> KB -> MB -> GB)
function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) {
    return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
  } else if (bytes >= 1024 * 1024) {
    return `${(bytes / 1024 / 1024).toFixed(2)} MB`
  } else if (bytes >= 1024) {
    return `${(bytes / 1024).toFixed(2)} KB`
  }
  return `${bytes.toFixed(0)} B`
}

// 截断字符串
function truncateStr(str: string, maxLen: number): string {
  if (!str) return ''
  return str.length > maxLen ? str.slice(0, maxLen) + '...' : str
}

// 进程表格 tooltip 样式
const procTableStyle = `margin-top:8px;border-collapse:collapse;width:100%;font-size:11px`
const procThStyle = `padding:2px 6px;text-align:left;border-bottom:1px solid rgba(128,128,128,0.3);white-space:nowrap`
const procTdStyle = `padding:2px 6px;white-space:nowrap`

// 生成进程表格 HTML（CPU / 内存通用）
function buildProcessTable(
  procs: ProcessStat[],
  metricHeader: string,
  formatValue: (v: number) => string,
): string {
  if (!procs || procs.length === 0) return ''
  let html = `<table style="${procTableStyle}">`
  html += `<tr><th style="${procThStyle}">PID</th><th style="${procThStyle}">${$gettext('Process')}</th><th style="${procThStyle}">${metricHeader}</th><th style="${procThStyle}">${$gettext('User')}</th><th style="${procThStyle}">${$gettext('Command')}</th></tr>`
  for (const proc of procs) {
    html += `<tr><td style="${procTdStyle}">${proc.pid}</td><td style="${procTdStyle}">${truncateStr(proc.name, 16)}</td><td style="${procTdStyle}">${formatValue(proc.value)}</td><td style="${procTdStyle}">${truncateStr(proc.username, 10)}</td><td style="${procTdStyle}">${truncateStr(proc.command, 30)}</td></tr>`
  }
  html += `</table>`
  return html
}

// 生成磁盘 IO 进程表格 HTML（总读取/总写入分开显示）
function buildDiskIOProcessTable(procs: ProcessStat[]): string {
  if (!procs || procs.length === 0) return ''
  let html = `<table style="${procTableStyle}">`
  html += `<tr><th style="${procThStyle}">PID</th><th style="${procThStyle}">${$gettext('Process')}</th><th style="${procThStyle}">${$gettext('Total Read')}</th><th style="${procThStyle}">${$gettext('Total Write')}</th><th style="${procThStyle}">${$gettext('User')}</th><th style="${procThStyle}">${$gettext('Command')}</th></tr>`
  for (const proc of procs) {
    html += `<tr><td style="${procTdStyle}">${proc.pid}</td><td style="${procTdStyle}">${truncateStr(proc.name, 16)}</td><td style="${procTdStyle}">${formatBytes(proc.read || 0)}</td><td style="${procTdStyle}">${formatBytes(proc.write || 0)}</td><td style="${procTdStyle}">${truncateStr(proc.username, 10)}</td><td style="${procTdStyle}">${truncateStr(proc.command, 30)}</td></tr>`
  }
  html += `</table>`
  return html
}

// 基础图表配置
function createBaseOption(valueFormatter?: any, timeRangeMs?: number) {
  // 根据时间范围决定刻度间隔：大于1天用4小时，否则用2小时
  const oneDayMs = 24 * 60 * 60 * 1000
  const hourInterval = timeRangeMs && timeRangeMs > oneDayMs ? 4 : 2

  const xAxisConfig = {
    type: 'category' as const,
    boundaryGap: false,
    data: [] as string[],
    axisLabel: {
      interval: (index: number, value: string) => {
        // 根据时间范围动态调整刻度间隔
        const timePart = value.split(' ')[1] || ''
        const hour = parseInt(timePart.split(':')[0] || '0', 10)
        const minute = parseInt(timePart.split(':')[1] || '0', 10)
        return minute === 0 && hour % hourInterval === 0
      },
      formatter: (value: string) => {
        // 显示日期和时间两行：MM-DD\nHH:mm
        const parts = value.split(' ')
        const datePart = parts[0] || ''
        const timePart = parts[1] || value
        // 从 YYYY-MM-DD 提取 MM-DD
        const dateMatch = datePart.match(/\d{2}-\d{2}$/)
        const shortDate = dateMatch ? dateMatch[0] : datePart
        return `${shortDate}\n${timePart}`
      },
    },
  }
  return {
    tooltip: {
      trigger: 'axis' as const,
      valueFormatter: valueFormatter,
    },
    legend: {
      type: 'scroll' as const,
      left: 20,
      top: 0,
    },
    grid: {
      left: 60,
      right: 20,
      top: 50,
      bottom: 80,
    },
    xAxis: xAxisConfig,
    yAxis: [
      {
        type: 'value' as const,
      },
    ],
    dataZoom: {
      type: 'slider' as const,
      show: true,
      realtime: true,
      start: 0,
      end: 100,
      bottom: 10,
    },
  }
}

// 负载图表配置
const loadOption = computed<EChartsOption>(() => {
  const timeRange = loadTime.value.end - loadTime.value.start
  const base = createBaseOption(undefined, timeRange)
  return {
    ...base,
    xAxis: { ...base.xAxis, data: loadData.value?.times || [] },
    series: [
      {
        name: $gettext('1 minute'),
        type: 'line',
        smooth: true,
        data: loadData.value?.load?.load1 || [],
        markPoint: {
          data: [
            { type: 'max', name: $gettext('Maximum') },
            { type: 'min', name: $gettext('Minimum') },
          ],
        },
        markLine: {
          data: [{ type: 'average', name: $gettext('Average') }],
        },
      },
      {
        name: $gettext('5 minutes'),
        type: 'line',
        smooth: true,
        data: loadData.value?.load?.load5 || [],
      },
      {
        name: $gettext('15 minutes'),
        type: 'line',
        smooth: true,
        data: loadData.value?.load?.load15 || [],
      },
    ],
  }
})

// CPU图表配置
const cpuOption = computed<EChartsOption>(() => {
  const timeRange = cpuTime.value.end - cpuTime.value.start
  const base = createBaseOption(undefined, timeRange)
  return {
    ...base,
    tooltip: {
      trigger: 'axis',
      enterable: true,
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const idx = params[0].dataIndex
        let html = `<div style="font-size:12px"><div style="margin-bottom:4px;font-weight:bold">${params[0].name}</div>`
        for (const p of params) {
          html += `<div>${p.marker} ${p.seriesName}: ${p.value}%</div>`
        }
        const procs = cpuData.value?.top_processes?.[idx]?.cpu
        html += buildProcessTable(procs, 'CPU%', (v) => `${v.toFixed(1)}%`)
        html += `</div>`
        return html
      },
    },
    xAxis: { ...base.xAxis, data: cpuData.value?.times || [] },
    yAxis: [
      {
        type: 'value',
        name: $gettext('Usage %'),
        min: 0,
        max: 100,
        axisLabel: {
          formatter: '{value}%',
        },
      },
    ],
    series: [
      {
        name: $gettext('Usage'),
        type: 'line',
        smooth: true,
        areaStyle: {
          opacity: 0.3,
        },
        data: cpuData.value?.cpu?.percent || [],
        markPoint: {
          data: [
            { type: 'max', name: $gettext('Maximum') },
            { type: 'min', name: $gettext('Minimum') },
          ],
        },
        markLine: {
          data: [{ type: 'average', name: $gettext('Average') }],
        },
      },
    ],
  }
})

// 内存图表配置
const memOption = computed<EChartsOption>(() => {
  const timeRange = memTime.value.end - memTime.value.start
  const base = createBaseOption(undefined, timeRange)
  const total = parseFloat(memData.value?.mem?.total || '0')
  return {
    ...base,
    tooltip: {
      trigger: 'axis',
      enterable: true,
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const idx = params[0].dataIndex
        let html = `<div style="font-size:12px"><div style="margin-bottom:4px;font-weight:bold">${params[0].name}</div>`
        for (const p of params) {
          html += `<div>${p.marker} ${p.seriesName}: ${formatMemory(p.value)}</div>`
        }
        const procs = memData.value?.top_processes?.[idx]?.memory
        html += buildProcessTable(procs, $gettext('Memory'), (v) => formatBytes(v))
        html += `</div>`
        return html
      },
    },
    legend: {
      ...base.legend,
      data: [$gettext('Memory'), 'Swap'],
    },
    xAxis: { ...base.xAxis, data: memData.value?.times || [] },
    yAxis: [
      {
        type: 'value',
        name: $gettext('Unit MB'),
        min: 0,
        max: total > 0 ? total : undefined,
        axisLabel: {
          formatter: '{value} M',
        },
      },
    ],
    series: [
      {
        name: $gettext('Memory'),
        type: 'line',
        smooth: true,
        areaStyle: {
          opacity: 0.3,
        },
        data: memData.value?.mem?.used || [],
        markPoint: {
          data: [
            { type: 'max', name: $gettext('Maximum') },
            { type: 'min', name: $gettext('Minimum') },
          ],
        },
        markLine: {
          data: [{ type: 'average', name: $gettext('Average') }],
        },
      },
      {
        name: 'Swap',
        type: 'line',
        smooth: true,
        data: memData.value?.swap?.used || [],
      },
    ],
  }
})

// 网络图表配置
const netOption = computed<EChartsOption>(() => {
  const timeRange = netTime.value.end - netTime.value.start
  const base = createBaseOption(formatSpeed, timeRange)

  // 根据选中的网卡筛选数据
  const devices =
    netData.value?.net?.filter((d: { name: string }) =>
      selectedNetDevices.value.includes(d.name),
    ) || []

  const series: any[] = []
  devices.forEach((device: { name: string; tx: string[]; rx: string[] }) => {
    series.push({
      name: `${device.name} ${$gettext('Upload')}`,
      type: 'line',
      smooth: true,
      data: device.tx,
    })
    series.push({
      name: `${device.name} ${$gettext('Download')}`,
      type: 'line',
      smooth: true,
      data: device.rx,
    })
  })

  return {
    ...base,
    xAxis: { ...base.xAxis, data: netData.value?.times || [] },
    yAxis: [
      {
        type: 'value',
        name: 'KB/s',
        axisLabel: {
          formatter: '{value}',
        },
      },
    ],
    series,
  }
})

// 磁盘IO图表配置
const diskIOOption = computed<EChartsOption>(() => {
  const timeRange = diskIOTime.value.end - diskIOTime.value.start
  const base = createBaseOption(undefined, timeRange)

  // 根据选中的磁盘筛选数据
  const disks =
    diskIOData.value?.disk_io?.filter((d: { name: string }) =>
      selectedDisks.value.includes(d.name),
    ) || []

  const series: any[] = []
  disks.forEach((disk: { name: string; read_speed: string[]; write_speed: string[] }) => {
    series.push({
      name: `${disk.name} ${$gettext('Read')}`,
      type: 'line',
      smooth: true,
      areaStyle: {
        opacity: 0.3,
      },
      data: disk.read_speed,
    })
    series.push({
      name: `${disk.name} ${$gettext('Write')}`,
      type: 'line',
      smooth: true,
      areaStyle: {
        opacity: 0.3,
      },
      data: disk.write_speed,
    })
  })

  return {
    ...base,
    tooltip: {
      trigger: 'axis',
      enterable: true,
      formatter: (params: any) => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const idx = params[0].dataIndex
        let html = `<div style="font-size:12px"><div style="margin-bottom:4px;font-weight:bold">${params[0].name}</div>`
        for (const p of params) {
          html += `<div>${p.marker} ${p.seriesName}: ${formatSpeed(p.value)}</div>`
        }
        const procs = diskIOData.value?.top_processes?.[idx]?.disk_io
        html += buildDiskIOProcessTable(procs)
        html += `</div>`
        return html
      },
    },
    xAxis: { ...base.xAxis, data: diskIOData.value?.times || [] },
    yAxis: [
      {
        type: 'value',
        name: 'KB/s',
        axisLabel: {
          formatter: '{value}',
        },
      },
    ],
    series,
  }
})

// 操作函数
const handleUpdate = async () => {
  updateLoading.value = true
  useRequest(monitor.updateSetting(monitorSwitch.value, saveDay.value, monitorInterval.value))
    .onSuccess(() => {
      window.$message.success($gettext('Operation successful'))
    })
    .onComplete(() => {
      updateLoading.value = false
    })
}

const handleClear = async () => {
  useRequest(monitor.clear()).onSuccess(() => {
    window.$message.success($gettext('Operation successful'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <div class="py-2 flex flex-wrap gap-3 items-center justify-between">
        <div class="flex flex-wrap gap-3 items-center">
          <div class="flex gap-2 items-center">
            {{ $gettext('Enable Monitoring') }}
            <n-switch v-model:value="monitorSwitch" @update-value="handleUpdate" />
          </div>
          <div class="flex gap-2 items-center">
            {{ $gettext('Save Days') }}
            <n-input-number v-model:value="saveDay" :min="1" :max="365">
              <template #suffix>{{ $gettext('days') }}</template>
            </n-input-number>
          </div>
          <div class="flex gap-2 items-center">
            {{ $gettext('Collection Interval') }}
            <n-input-number v-model:value="monitorInterval" :min="1" :max="120">
              <template #suffix>{{ $gettext('minutes') }}</template>
            </n-input-number>
          </div>
          <n-button
            type="primary"
            :loading="updateLoading"
            :disabled="updateLoading"
            @click="handleUpdate"
          >
            {{ $gettext('Confirm') }}
          </n-button>
        </div>

        <div class="flex gap-2 items-center">
          <ConfirmDialog
            type="danger"
            :content="$gettext('Are you sure you want to clear?')"
            @confirm="handleClear"
          >
            <template #trigger>
              <n-button type="error">
                {{ $gettext('Clear Monitoring Records') }}
              </n-button>
            </template>
          </ConfirmDialog>
        </div>
      </div>
    </template>

    <div class="pt-4 flex flex-col gap-6">
      <!-- 负载 - 全宽 -->
      <n-card :bordered="false">
        <template #header>
          <div class="flex items-center justify-between">
            <span class="font-bold">{{ $gettext('Load') }}</span>
            <n-button-group size="small">
              <n-button
                :type="loadTime.preset === 'yesterday' ? 'primary' : 'default'"
                @click="updateTimeRange('load', 'yesterday')"
              >
                {{ $gettext('Yesterday') }}
              </n-button>
              <n-button
                :type="loadTime.preset === 'today' ? 'primary' : 'default'"
                @click="updateTimeRange('load', 'today')"
              >
                {{ $gettext('Today') }}
              </n-button>
              <n-button
                :type="loadTime.preset === 'week' ? 'primary' : 'default'"
                @click="updateTimeRange('load', 'week')"
              >
                {{ $gettext('Last 7 Days') }}
              </n-button>
              <n-popover
                v-model:show="loadCustomPopover"
                trigger="click"
                placement="bottom-end"
                :show-arrow="false"
              >
                <template #trigger>
                  <n-button :type="loadTime.preset === 'custom' ? 'primary' : 'default'">
                    {{ $gettext('Custom') }}
                  </n-button>
                </template>
                <n-date-picker
                  v-model:value="loadTempRange"
                  type="daterange"
                  panel
                  :default-time="['00:00:00', '23:59:59']"
                  :actions="['confirm']"
                  @confirm="confirmCustomTime('load')"
                />
              </n-popover>
            </n-button-group>
          </div>
        </template>
        <n-spin :show="loadLoading">
          <v-chart class="h-300px" :option="loadOption" autoresize />
        </n-spin>
      </n-card>

      <!-- CPU 和 内存 - 两列 -->
      <div class="gap-6 grid grid-cols-1 lg:grid-cols-2">
        <!-- CPU -->
        <n-card :bordered="false">
          <template #header>
            <div class="flex items-center justify-between">
              <span class="font-bold">CPU</span>
              <n-button-group size="small">
                <n-button
                  :type="cpuTime.preset === 'yesterday' ? 'primary' : 'default'"
                  @click="updateTimeRange('cpu', 'yesterday')"
                >
                  {{ $gettext('Yesterday') }}
                </n-button>
                <n-button
                  :type="cpuTime.preset === 'today' ? 'primary' : 'default'"
                  @click="updateTimeRange('cpu', 'today')"
                >
                  {{ $gettext('Today') }}
                </n-button>
                <n-button
                  :type="cpuTime.preset === 'week' ? 'primary' : 'default'"
                  @click="updateTimeRange('cpu', 'week')"
                >
                  {{ $gettext('Last 7 Days') }}
                </n-button>
                <n-popover
                  v-model:show="cpuCustomPopover"
                  trigger="click"
                  placement="bottom-end"
                  :show-arrow="false"
                >
                  <template #trigger>
                    <n-button :type="cpuTime.preset === 'custom' ? 'primary' : 'default'">
                      {{ $gettext('Custom') }}
                    </n-button>
                  </template>
                  <n-date-picker
                    v-model:value="cpuTempRange"
                    type="daterange"
                    panel
                    :default-time="['00:00:00', '23:59:59']"
                    :actions="['confirm']"
                    @confirm="confirmCustomTime('cpu')"
                  />
                </n-popover>
              </n-button-group>
            </div>
          </template>
          <n-spin :show="cpuLoading">
            <v-chart class="h-300px" :option="cpuOption" autoresize />
          </n-spin>
        </n-card>

        <!-- 内存 -->
        <n-card :bordered="false">
          <template #header>
            <div class="flex items-center justify-between">
              <span class="font-bold">{{ $gettext('Memory') }}</span>
              <n-button-group size="small">
                <n-button
                  :type="memTime.preset === 'yesterday' ? 'primary' : 'default'"
                  @click="updateTimeRange('mem', 'yesterday')"
                >
                  {{ $gettext('Yesterday') }}
                </n-button>
                <n-button
                  :type="memTime.preset === 'today' ? 'primary' : 'default'"
                  @click="updateTimeRange('mem', 'today')"
                >
                  {{ $gettext('Today') }}
                </n-button>
                <n-button
                  :type="memTime.preset === 'week' ? 'primary' : 'default'"
                  @click="updateTimeRange('mem', 'week')"
                >
                  {{ $gettext('Last 7 Days') }}
                </n-button>
                <n-popover
                  v-model:show="memCustomPopover"
                  trigger="click"
                  placement="bottom-end"
                  :show-arrow="false"
                >
                  <template #trigger>
                    <n-button :type="memTime.preset === 'custom' ? 'primary' : 'default'">
                      {{ $gettext('Custom') }}
                    </n-button>
                  </template>
                  <n-date-picker
                    v-model:value="memTempRange"
                    type="daterange"
                    panel
                    :default-time="['00:00:00', '23:59:59']"
                    :actions="['confirm']"
                    @confirm="confirmCustomTime('mem')"
                  />
                </n-popover>
              </n-button-group>
            </div>
          </template>
          <n-spin :show="memLoading">
            <v-chart class="h-300px" :option="memOption" autoresize />
          </n-spin>
        </n-card>
      </div>

      <!-- 磁盘IO 和 网络 - 两列 -->
      <div class="gap-6 grid grid-cols-1 lg:grid-cols-2">
        <!-- 磁盘IO -->
        <n-card :bordered="false">
          <template #header>
            <div class="flex flex-col gap-2">
              <div class="flex items-center justify-between">
                <span class="font-bold">{{ $gettext('Disk I/O') }}</span>
                <n-button-group size="small">
                  <n-button
                    :type="diskIOTime.preset === 'yesterday' ? 'primary' : 'default'"
                    @click="updateTimeRange('diskIO', 'yesterday')"
                  >
                    {{ $gettext('Yesterday') }}
                  </n-button>
                  <n-button
                    :type="diskIOTime.preset === 'today' ? 'primary' : 'default'"
                    @click="updateTimeRange('diskIO', 'today')"
                  >
                    {{ $gettext('Today') }}
                  </n-button>
                  <n-button
                    :type="diskIOTime.preset === 'week' ? 'primary' : 'default'"
                    @click="updateTimeRange('diskIO', 'week')"
                  >
                    {{ $gettext('Last 7 Days') }}
                  </n-button>
                  <n-popover
                    v-model:show="diskIOCustomPopover"
                    trigger="click"
                    placement="bottom-end"
                    :show-arrow="false"
                  >
                    <template #trigger>
                      <n-button :type="diskIOTime.preset === 'custom' ? 'primary' : 'default'">
                        {{ $gettext('Custom') }}
                      </n-button>
                    </template>
                    <n-date-picker
                      v-model:value="diskIOTempRange"
                      type="daterange"
                      panel
                      :default-time="['00:00:00', '23:59:59']"
                      :actions="['confirm']"
                      @confirm="confirmCustomTime('diskIO')"
                    />
                  </n-popover>
                </n-button-group>
              </div>
              <n-checkbox-group
                v-if="availableDisks.length > 0"
                v-model:value="selectedDisks"
                class="flex flex-wrap gap-x-1 gap-y-1"
              >
                <n-checkbox
                  v-for="disk in availableDisks"
                  :key="disk"
                  :value="disk"
                  :label="disk"
                  size="small"
                />
              </n-checkbox-group>
            </div>
          </template>
          <n-spin :show="diskIOLoading">
            <v-chart class="h-300px" :option="diskIOOption" autoresize />
          </n-spin>
        </n-card>

        <!-- 网络 -->
        <n-card :bordered="false">
          <template #header>
            <div class="flex flex-col gap-2">
              <div class="flex items-center justify-between">
                <span class="font-bold">{{ $gettext('Network') }}</span>
                <n-button-group size="small">
                  <n-button
                    :type="netTime.preset === 'yesterday' ? 'primary' : 'default'"
                    @click="updateTimeRange('net', 'yesterday')"
                  >
                    {{ $gettext('Yesterday') }}
                  </n-button>
                  <n-button
                    :type="netTime.preset === 'today' ? 'primary' : 'default'"
                    @click="updateTimeRange('net', 'today')"
                  >
                    {{ $gettext('Today') }}
                  </n-button>
                  <n-button
                    :type="netTime.preset === 'week' ? 'primary' : 'default'"
                    @click="updateTimeRange('net', 'week')"
                  >
                    {{ $gettext('Last 7 Days') }}
                  </n-button>
                  <n-popover
                    v-model:show="netCustomPopover"
                    trigger="click"
                    placement="bottom-end"
                    :show-arrow="false"
                  >
                    <template #trigger>
                      <n-button :type="netTime.preset === 'custom' ? 'primary' : 'default'">
                        {{ $gettext('Custom') }}
                      </n-button>
                    </template>
                    <n-date-picker
                      v-model:value="netTempRange"
                      type="daterange"
                      panel
                      :default-time="['00:00:00', '23:59:59']"
                      :actions="['confirm']"
                      @confirm="confirmCustomTime('net')"
                    />
                  </n-popover>
                </n-button-group>
              </div>
              <n-checkbox-group
                v-if="availableNetDevices.length > 0"
                v-model:value="selectedNetDevices"
                class="flex flex-wrap gap-x-1 gap-y-1"
              >
                <n-checkbox
                  v-for="device in availableNetDevices"
                  :key="device"
                  :value="device"
                  :label="device"
                  size="small"
                />
              </n-checkbox-group>
            </div>
          </template>
          <n-spin :show="netLoading">
            <v-chart class="h-300px" :option="netOption" autoresize />
          </n-spin>
        </n-card>
      </div>
    </div>
  </PageContainer>
</template>

<style scoped lang="scss"></style>

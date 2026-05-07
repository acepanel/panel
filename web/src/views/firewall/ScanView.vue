<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { NButton } from 'naive-ui'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { formatDateTime } from '@/utils'

import { codeToName } from '../website/stats/country-name-map'

const { $gettext, $pgettext } = useGettext()
const { confirmAction } = useConfirm()

use([CanvasRenderer, LineChart, TooltipComponent, LegendComponent, GridComponent])

const scanEnabled = ref(false)
const ready = ref(false)
const currentTab = ref('overview')

// 日期范围
const dateRange = ref<[number, number] | null>(null)

function initDateRange() {
  const now = new Date()
  const end = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const start = new Date(end.getTime() - 6 * 24 * 60 * 60 * 1000)
  dateRange.value = [start.getTime(), end.getTime()]
}
initDateRange()

function formatDate(ts: number): string {
  const d = new Date(ts)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const startDate = computed(() => (dateRange.value ? formatDate(dateRange.value[0]) : ''))
const endDate = computed(() => (dateRange.value ? formatDate(dateRange.value[1]) : ''))

// 汇总数据
const summary = ref({ total_count: 0, unique_ips: 0, unique_ports: 0 })
const summaryLoading = ref(false)
const prevSummary = ref({ total_count: 0, unique_ips: 0, unique_ports: 0 })

// 趋势数据
const trendData = ref<any[]>([])
const trendLoading = ref(false)

// Top IP
const topIPs = ref<any[]>([])
const topIPsLoading = ref(false)

// Top 端口
const topPorts = ref<any[]>([])
const topPortsLoading = ref(false)

// 事件列表
const events = ref<any[]>([])
const eventsTotal = ref(0)
const eventsPage = ref(1)
const eventsPageSize = ref(20)
const eventsLoading = ref(false)
const searchIP = ref('')
const searchPort = ref<number | null>(null)
const searchLocation = ref('')

const loadSummary = () => {
  summaryLoading.value = true
  useRequest(firewall.scanSummary(startDate.value, endDate.value))
    .onSuccess(({ data }) => {
      summary.value = data
    })
    .onComplete(() => {
      summaryLoading.value = false
    })

  // 获取上一个同长度周期的数据用于对比
  if (dateRange.value) {
    const rangeMs = dateRange.value[1] - dateRange.value[0]
    const oneDayMs = 24 * 60 * 60 * 1000
    const prevEnd = dateRange.value[0] - oneDayMs
    const prevStart = prevEnd - rangeMs
    useRequest(firewall.scanSummary(formatDate(prevStart), formatDate(prevEnd))).onSuccess(
      ({ data }) => {
        prevSummary.value = data
      },
    )
  }
}

const loadTrend = () => {
  trendLoading.value = true
  useRequest(firewall.scanTrend(startDate.value, endDate.value))
    .onSuccess(({ data }) => {
      trendData.value = data || []
    })
    .onComplete(() => {
      trendLoading.value = false
    })
}

const loadTopIPs = () => {
  topIPsLoading.value = true
  useRequest(firewall.scanTopIPs(startDate.value, endDate.value, 10))
    .onSuccess(({ data }) => {
      topIPs.value = data || []
    })
    .onComplete(() => {
      topIPsLoading.value = false
    })
}

const loadTopPorts = () => {
  topPortsLoading.value = true
  useRequest(firewall.scanTopPorts(startDate.value, endDate.value, 10))
    .onSuccess(({ data }) => {
      topPorts.value = data || []
    })
    .onComplete(() => {
      topPortsLoading.value = false
    })
}

const loadEvents = () => {
  eventsLoading.value = true
  useRequest(
    firewall.scanEvents(
      startDate.value,
      endDate.value,
      eventsPage.value,
      eventsPageSize.value,
      searchIP.value || undefined,
      searchPort.value || undefined,
      searchLocation.value || undefined,
    ),
  )
    .onSuccess(({ data }) => {
      events.value = data.items || []
      eventsTotal.value = data.total || 0
    })
    .onComplete(() => {
      eventsLoading.value = false
    })
}

const refreshOverview = () => {
  loadSummary()
  loadTrend()
  loadTopIPs()
  loadTopPorts()
}

watch(dateRange, () => {
  if (dateRange.value) {
    refreshOverview()
    if (currentTab.value === 'events') {
      loadEvents()
    }
  }
})

watch([eventsPage, eventsPageSize], () => {
  loadEvents()
})

watch(currentTab, (val) => {
  if (val === 'events' && events.value.length === 0) {
    loadEvents()
  }
})

onMounted(() => {
  useRequest(firewall.scanSetting()).onSuccess(({ data }) => {
    scanEnabled.value = data.enabled
    ready.value = true
    if (data.enabled) {
      refreshOverview()
    }
  })
})

// 趋势图表
const trendOption = computed<EChartsOption>(() => ({
  tooltip: {
    trigger: 'axis',
  },
  legend: {
    data: [$gettext('Scan Count'), $gettext('Source IPs')],
    top: 0,
  },
  grid: {
    left: '3%',
    right: '4%',
    top: '15%',
    bottom: '3%',
    outerBoundsMode: 'same',
    outerBoundsContain: 'axisLabel',
  },
  xAxis: {
    type: 'category',
    data: trendData.value.map((d: any) => d.date),
  },
  yAxis: [
    {
      type: 'value',
      name: $gettext('Scan Count'),
      axisLine: { show: false },
    },
    {
      type: 'value',
      name: $gettext('Source IPs'),
      splitLine: { lineStyle: { type: 'dashed' } },
      axisLine: { show: false },
    },
  ],
  series: [
    {
      name: $gettext('Scan Count'),
      type: 'line',
      smooth: true,
      data: trendData.value.map((d: any) => d.total_count),
    },
    {
      name: $gettext('Source IPs'),
      type: 'line',
      smooth: true,
      yAxisIndex: 1,
      data: trendData.value.map((d: any) => d.unique_ips),
    },
  ],
}))

// 拉黑 IP
const handleBlock = (ip: string) => {
  const family = ip.includes(':') ? 'ipv6' : 'ipv4'
  useRequest(
    firewall.createIpRule({
      family,
      protocol: 'tcp/udp',
      address: ip,
      strategy: 'drop',
      direction: 'in',
    }),
  ).onSuccess(() => {
    window.$message.success($gettext('%{ address } blocked successfully', { address: ip }))
  })
}

// 操作列：拉黑按钮
const blockColumn = {
  title: $gettext('Actions'),
  key: 'actions',
  width: 100,
  render: (row: any) =>
    h(
      NButton,
      {
        size: 'tiny',
        type: 'error',
        onClick: async () => {
          const ok = await confirmAction({
            type: 'warning',
            title: $pgettext('firewall', 'Block IP'),
            content: $pgettext('firewall', 'Block %{ ip }?', { ip: row.source_ip }),
          })
          if (ok) handleBlock(row.source_ip)
        },
      },
      () => $pgettext('firewall', 'Block'),
    ),
}

const topIPColumns: any = [
  { title: $gettext('Source IP'), key: 'source_ip', minWidth: 150 },
  {
    title: $gettext('Location'),
    key: 'country',
    minWidth: 120,
    render: (row: any) =>
      [codeToName[row.country] || row.country, row.region, row.city].filter(Boolean).join(' ') ||
      '-',
  },
  { title: $gettext('Scan Count'), key: 'total_count', width: 120, sorter: 'default' },
  { title: $gettext('Port Count'), key: 'port_count', width: 120 },
  {
    title: $gettext('Last Seen'),
    key: 'last_seen',
    width: 180,
    render: (row: any) => formatDateTime(row.last_seen),
  },
  blockColumn,
]

const topPortColumns: any = [
  { title: $gettext('Port'), key: 'port', width: 100 },
  { title: $gettext('Protocol'), key: 'protocol', width: 100 },
  { title: $gettext('Scan Count'), key: 'total_count', width: 120, sorter: 'default' },
  { title: $gettext('IP Count'), key: 'ip_count', width: 120 },
]

const eventColumns: any = [
  { title: $gettext('Source IP'), key: 'source_ip', minWidth: 150 },
  {
    title: $gettext('Location'),
    key: 'country',
    minWidth: 120,
    render: (row: any) =>
      [codeToName[row.country] || row.country, row.region, row.city].filter(Boolean).join(' ') ||
      '-',
  },
  { title: $gettext('Port'), key: 'port', width: 100 },
  { title: $gettext('Protocol'), key: 'protocol', width: 100 },
  { title: $gettext('Scan Count'), key: 'count', width: 120 },
  {
    title: $gettext('First Seen'),
    key: 'first_seen',
    width: 180,
    render: (row: any) => formatDateTime(row.first_seen),
  },
  {
    title: $gettext('Last Seen'),
    key: 'last_seen',
    width: 180,
    render: (row: any) => formatDateTime(row.last_seen),
  },
  blockColumn,
]

const handleClear = () => {
  useRequest(firewall.scanClear()).onSuccess(() => {
    window.$message.success($gettext('Cleared successfully'))
    refreshOverview()
    loadEvents()
  })
}
</script>

<template>
  <!-- 未启用提示 -->
  <n-result v-if="ready && !scanEnabled" status="info" :title="$gettext('Scan Awareness Disabled')">
    <template #footer>
      <n-text depth="3">
        {{ $gettext('Enable scan awareness in the Settings tab to start detecting port scans.') }}
      </n-text>
    </template>
  </n-result>

  <!-- 已启用 -->
  <n-flex v-else-if="ready" vertical :size="20">
    <n-tabs v-model:value="currentTab" type="segment">
      <n-tab name="overview" :tab="$gettext('Overview')" />
      <n-tab name="events" :tab="$gettext('Scan Events')" />
    </n-tabs>
    <!-- 工具栏 -->
    <div class="flex w-full items-center justify-between">
      <n-date-picker
        v-model:value="dateRange"
        type="daterange"
        clearable
        :default-time="['00:00:00', '23:59:59']"
      />
      <n-flex items-center :size="8">
        <n-input
          v-if="currentTab === 'events'"
          v-model:value="searchIP"
          :placeholder="$gettext('Search IP')"
          clearable
          class="!w-40"
          @clear="loadEvents()"
          @keyup.enter="loadEvents()"
        />
        <n-input-number
          v-if="currentTab === 'events'"
          v-model:value="searchPort"
          :placeholder="$gettext('Port')"
          clearable
          :show-button="false"
          class="!w-25"
          @clear="loadEvents()"
          @keyup.enter="loadEvents()"
        />
        <n-input
          v-if="currentTab === 'events'"
          v-model:value="searchLocation"
          :placeholder="$gettext('Location')"
          clearable
          class="!w-40"
          @clear="loadEvents()"
          @keyup.enter="loadEvents()"
        />
        <ConfirmDialog
          type="danger"
          :content="$gettext('Are you sure you want to clear all scan data?')"
          @confirm="handleClear"
        >
          <template #trigger>
            <n-button type="error" ghost>
              {{ $gettext('Clear Data') }}
            </n-button>
          </template>
        </ConfirmDialog>
      </n-flex>
    </div>

    <!-- 概览 -->
    <template v-if="currentTab === 'overview'">
      <!-- 统计卡片 -->
      <n-grid :cols="3" :x-gap="16">
        <n-gi>
          <n-card :bordered="false">
            <n-statistic :label="$gettext('Total Scans')" :value="summary.total_count">
              <template #suffix>
                <n-text :type="summary.total_count > prevSummary.total_count ? 'error' : 'success'">
                  {{ summary.total_count > prevSummary.total_count ? '↑' : '↓' }}
                </n-text>
              </template>
            </n-statistic>
          </n-card>
        </n-gi>
        <n-gi>
          <n-card :bordered="false">
            <n-statistic :label="$gettext('Scanned Ports')" :value="summary.unique_ports">
              <template #suffix>
                <n-text
                  :type="summary.unique_ports > prevSummary.unique_ports ? 'error' : 'success'"
                >
                  {{ summary.unique_ports > prevSummary.unique_ports ? '↑' : '↓' }}
                </n-text>
              </template>
            </n-statistic>
          </n-card>
        </n-gi>
        <n-gi>
          <n-card :bordered="false">
            <n-statistic :label="$gettext('Source IPs')" :value="summary.unique_ips">
              <template #suffix>
                <n-text :type="summary.unique_ips > prevSummary.unique_ips ? 'error' : 'success'">
                  {{ summary.unique_ips > prevSummary.unique_ips ? '↑' : '↓' }}
                </n-text>
              </template>
            </n-statistic>
          </n-card>
        </n-gi>
      </n-grid>

      <!-- 趋势图 -->
      <n-card :bordered="false" :title="$gettext('Scan Trend')">
        <n-spin :show="trendLoading">
          <v-chart class="h-300px" :option="trendOption" autoresize />
        </n-spin>
      </n-card>

      <!-- Top IP 和 Top 端口 -->
      <n-grid :cols="2" :x-gap="16">
        <n-gi>
          <n-card h-full :bordered="false" :title="$gettext('Top 10 Source IPs')">
            <n-data-table
              striped
              size="small"
              :loading="topIPsLoading"
              :columns="topIPColumns"
              :data="topIPs"
            />
          </n-card>
        </n-gi>
        <n-gi>
          <n-card h-full :bordered="false" :title="$gettext('Top 10 Scanned Ports')">
            <n-data-table
              striped
              size="small"
              :loading="topPortsLoading"
              :columns="topPortColumns"
              :data="topPorts"
            />
          </n-card>
        </n-gi>
      </n-grid>
    </template>

    <!-- 扫描记录 -->
    <template v-if="currentTab === 'events'">
      <n-card :bordered="false">
        <n-data-table
          v-model:page="eventsPage"
          v-model:pageSize="eventsPageSize"
          striped
          remote
          :scroll-x="1100"
          :loading="eventsLoading"
          :columns="eventColumns"
          :data="events"
          :row-key="(row: any) => row.id"
          :pagination="{
            page: eventsPage,
            pageSize: eventsPageSize,
            itemCount: eventsTotal,
            showQuickJumper: true,
            showSizePicker: true,
            pageSizes: [20, 50, 100],
          }"
        />
      </n-card>
    </template>
  </n-flex>
</template>

<style scoped lang="scss"></style>

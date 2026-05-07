<script lang="ts" setup>
defineOptions({
  name: 'home-index',
})

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
import { NButton, NDropdown, useThemeVars } from 'naive-ui'
import VChart from 'vue-echarts'
import { useGettext } from 'vue3-gettext'
import draggable from 'vuedraggable'

import app from '@/api/panel/app'
import home from '@/api/panel/home'
import TheIconLocal from '@/components/custom/TheIconLocal.vue'
import { router } from '@/router'
import { useTabStore } from '@/stores'
import { formatDateTime, formatDuration, toTimestamp } from '@/utils/common'
import { formatBytes, formatPercent } from '@/utils/file'

import type { ProcessStat, Realtime } from './types'

use([
  CanvasRenderer,
  LineChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DataZoomComponent,
])

const { current: locale, $gettext } = useGettext()
const themeVars = useThemeVars()
const tabStore = useTabStore()
const realtime = ref<Realtime | null>(null)

const cpuTopProcesses = ref<ProcessStat[]>([])
const memTopProcesses = ref<ProcessStat[]>([])
const cpuTopLoading = ref(false)
const memTopLoading = ref(false)
const cpuTopVisible = ref(false)
const memTopVisible = ref(false)

const loadTopProcesses = (type: 'cpu' | 'memory') => {
  if (type === 'cpu') {
    if (cpuTopVisible.value) {
      cpuTopVisible.value = false
      return
    }
    cpuTopLoading.value = true
    useRequest(home.topProcesses('cpu'))
      .onSuccess(({ data }) => {
        cpuTopProcesses.value = data
        cpuTopVisible.value = true
      })
      .onComplete(() => {
        cpuTopLoading.value = false
      })
  } else {
    if (memTopVisible.value) {
      memTopVisible.value = false
      return
    }
    memTopLoading.value = true
    useRequest(home.topProcesses('memory'))
      .onSuccess(({ data }) => {
        memTopProcesses.value = data
        memTopVisible.value = true
      })
      .onComplete(() => {
        memTopLoading.value = false
      })
  }
}

const { data: systemInfo } = useRequest(home.systemInfo, {
  initialData: {
    procs: 0,
    hostname: '',
    panel_version: '',
    commit_hash: '',
    build_id: '',
    build_time: '',
    build_user: '',
    build_host: '',
    go_version: '',
    kernel_arch: '',
    kernel_version: '',
    os_eol: false,
    os_name: '',
    os_supported: true,
    boot_time: 0,
    uptime: 0,
    nets: [],
    disks: [],
  },
})
const { data: apps, loading: appLoading } = useRequest(home.apps, {
  initialData: [],
})

const handleAppOrderChange = async () => {
  const slugs = apps.value.map((item: { slug: string }) => item.slug)
  await app.updateOrder(slugs)
  window.$message.success($gettext('Order updated'))
}
const { data: countInfo } = useRequest(home.countInfo, {
  initialData: {
    website: 0,
    database: 0,
    project: 0,
    cron: 0,
    container: -1,
  },
})

const nets = ref<Array<string>>([]) // 选择的网卡
const disks = ref<Array<string>>([]) // 选择的硬盘
const chartType = ref('net')
const unitType = ref('KB')
const units = [
  { label: 'B', value: 'B' },
  { label: 'KB', value: 'KB' },
  { label: 'MB', value: 'MB' },
  { label: 'GB', value: 'GB' },
]

const cores = ref(0)
const diskReadBytes = ref<Array<number>>([])
const diskWriteBytes = ref<Array<number>>([])
const netBytesSent = ref<Array<number>>([])
const netBytesRecv = ref<Array<number>>([])
const timeDiskData = ref<Array<string>>([])
const timeNetData = ref<Array<string>>([])
const total = reactive({
  diskReadBytes: 0,
  diskWriteBytes: 0,
  diskRWBytes: 0,
  diskRWTime: 0,
  netBytesSent: 0,
  netBytesRecv: 0,
})

const current = reactive({
  diskReadBytes: 0,
  diskWriteBytes: 0,
  diskRWBytes: 0,
  diskRWTime: 0,
  netBytesSent: 0,
  netBytesRecv: 0,
  time: 0,
})

const statusColor = (percentage: number) => {
  if (percentage >= 90) {
    return themeVars.value.errorColor
  } else if (percentage >= 80) {
    return themeVars.value.warningColor
  } else if (percentage >= 70) {
    return themeVars.value.infoColor
  }
  return themeVars.value.successColor
}

const statusText = (percentage: number) => {
  if (percentage >= 90) {
    return $gettext('Running blocked')
  } else if (percentage >= 80) {
    return $gettext('Running slowly')
  } else if (percentage >= 70) {
    return $gettext('Running normally')
  }
  return $gettext('Running smoothly')
}

const chartOptions = computed(() => {
  return {
    title: {
      text: chartType.value == 'net' ? $gettext('Network') : $gettext('Disk'),
      textAlign: 'left',
      textStyle: {
        fontSize: 20,
      },
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross',
      },
      formatter: function (params: any) {
        let res = params[0].name + '<br/>'
        params.forEach(function (item: any) {
          res += `${item.marker} ${item.seriesName}: ${item.value} ${unitType.value}<br/>`
        })
        return res
      },
    },
    legend: {
      align: 'left',
      data:
        chartType.value == 'net'
          ? [$gettext('Send'), $gettext('Receive')]
          : [$gettext('Read'), $gettext('Write')],
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: timeDiskData.value,
    },
    yAxis: {
      name: $gettext('Unit %{unit}', { unit: unitType.value }),
      type: 'value',
      axisLabel: {
        formatter: `{value} ${unitType.value}`,
      },
    },
    series: [
      {
        name: chartType.value == 'net' ? $gettext('Send') : $gettext('Read'),
        type: 'line',
        smooth: true,
        data: chartType.value == 'net' ? netBytesSent.value : diskReadBytes.value,
        markPoint: {
          data: [
            { type: 'max', name: $gettext('Maximum') },
            { type: 'min', name: $gettext('Minimum') },
          ],
        },
        markLine: {
          data: [{ type: 'average', name: $gettext('Average') }],
        },
        lineStyle: {
          color: 'rgb(247, 184, 81)',
        },
        itemStyle: {
          color: 'rgb(247, 184, 81)',
        },
        areaStyle: {
          color: 'rgb(247, 184, 81)',
        },
      },
      {
        name: chartType.value == 'net' ? $gettext('Receive') : $gettext('Write'),
        type: 'line',
        smooth: true,
        data: chartType.value == 'net' ? netBytesRecv.value : diskWriteBytes.value,
        markPoint: {
          data: [
            { type: 'max', name: $gettext('Maximum') },
            { type: 'min', name: $gettext('Minimum') },
          ],
        },
        markLine: {
          data: [{ type: 'average', name: $gettext('Average') }],
        },
        lineStyle: {
          color: 'rgb(82, 169, 255)',
        },
        itemStyle: {
          color: 'rgb(82, 169, 255)',
        },
        areaStyle: {
          color: 'rgb(82, 169, 255)',
        },
      },
    ],
  }
})

let isFetching = false

const fetchCurrent = () => {
  if (isFetching) return
  isFetching = true
  useRequest(home.current(nets.value, disks.value))
    .onSuccess(({ data }) => {
      data.percent = formatPercent(data.percent)
      data.mem.usedPercent = formatPercent(data.mem.usedPercent)
      // 计算 CPU 核心数
      if (cores.value == 0) {
        for (let i = 0; i < data.cpus.length; i++) {
          cores.value += data.cpus[i].cores
        }
      }
      // 计算实时数据
      const time = current.time == 0 ? 3 : toTimestamp(data.time) - current.time
      let netTotalSentTemp = 0
      let netTotalRecvTemp = 0
      for (let i = 0; i < data.net.length; i++) {
        if (data.net[i].name === 'lo') {
          continue
        }
        netTotalSentTemp += data.net[i].bytesSent
        netTotalRecvTemp += data.net[i].bytesRecv
      }
      current.netBytesSent =
        total.netBytesSent != 0 ? (netTotalSentTemp - total.netBytesSent) / time : 0
      current.netBytesRecv =
        total.netBytesRecv != 0 ? (netTotalRecvTemp - total.netBytesRecv) / time : 0
      total.netBytesSent = netTotalSentTemp
      total.netBytesRecv = netTotalRecvTemp
      // 计算硬盘读写
      let diskTotalReadTemp = 0
      let diskTotalWriteTemp = 0
      let diskRWTimeTemp = 0
      for (let i = 0; i < data.disk_io.length; i++) {
        diskTotalReadTemp += data.disk_io[i].readBytes
        diskTotalWriteTemp += data.disk_io[i].writeBytes
        diskRWTimeTemp += data.disk_io[i].readTime + data.disk_io[i].writeTime
      }
      current.diskReadBytes =
        total.diskReadBytes != 0 ? (diskTotalReadTemp - total.diskReadBytes) / time : 0
      current.diskWriteBytes =
        total.diskWriteBytes != 0 ? (diskTotalWriteTemp - total.diskWriteBytes) / time : 0
      current.diskRWBytes =
        total.diskRWBytes != 0
          ? (diskTotalReadTemp + diskTotalWriteTemp - total.diskRWBytes) / time
          : 0
      current.diskRWTime =
        total.diskRWTime != 0 ? Number(((diskRWTimeTemp - total.diskRWTime) / time).toFixed(2)) : 0
      current.time = toTimestamp(data.time)
      total.diskReadBytes = diskTotalReadTemp
      total.diskWriteBytes = diskTotalWriteTemp
      total.diskRWBytes = diskTotalReadTemp + diskTotalWriteTemp
      total.diskRWTime = diskRWTimeTemp

      // 图表数据填充
      netBytesSent.value.push(calculateSize(current.netBytesSent))
      if (netBytesSent.value.length > 10) {
        netBytesSent.value.splice(0, 1)
      }
      netBytesRecv.value.push(calculateSize(current.netBytesRecv))
      if (netBytesRecv.value.length > 10) {
        netBytesRecv.value.splice(0, 1)
      }
      diskReadBytes.value.push(calculateSize(current.diskReadBytes))
      if (diskReadBytes.value.length > 10) {
        diskReadBytes.value.splice(0, 1)
      }
      diskWriteBytes.value.push(calculateSize(current.diskWriteBytes))
      if (diskWriteBytes.value.length > 10) {
        diskWriteBytes.value.splice(0, 1)
      }
      timeDiskData.value.push(formatDateTime(data.time))
      if (timeDiskData.value.length > 10) {
        timeDiskData.value.splice(0, 1)
      }
      timeNetData.value.push(formatDateTime(data.time))
      if (timeNetData.value.length > 10) {
        timeNetData.value.splice(0, 1)
      }

      realtime.value = data
    })
    .onComplete(() => {
      isFetching = false
    })
}

const handleRestartPanel = () => {
  clearInterval(homeInterval)
  window.$message.loading($gettext('Panel restarting...'))
  useRequest(home.restart()).onSuccess(() => {
    window.$message.success($gettext('Panel restarted successfully'))
    setTimeout(() => {
      tabStore.reloadTab(tabStore.active)
    }, 3000)
  })
}

const showRestartServerConfirm = ref(false)

const handleRestartServer = () => {
  clearInterval(homeInterval)
  window.$message.loading($gettext('Server restarting...'))
  useRequest(home.restartServer())
}

const restartOptions = computed(() => [
  {
    label: $gettext('Restart Panel'),
    key: 'panel',
  },
  {
    label: $gettext('Restart Server'),
    key: 'server',
  },
])

const handleRestartSelect = (key: string) => {
  if (key === 'panel') {
    handleRestartPanel()
  } else if (key === 'server') {
    showRestartServerConfirm.value = true
  }
}

const checkUpdateLoading = ref(false)

const handleUpdate = () => {
  checkUpdateLoading.value = true
  useRequest(home.checkUpdate())
    .onSuccess(({ data }) => {
      if (data.update) {
        router.push({ name: 'home-update' })
      } else {
        window.$message.success($gettext('Current version is the latest'))
      }
    })
    .onComplete(() => {
      checkUpdateLoading.value = false
    })
}

const toSponsor = () => {
  window.open('https://afdian.com/a/tnborg')
}

const handleManageApp = (slug: string) => {
  router.push({ name: 'apps-' + slug + '-index' })
}

const calculateSize = (bytes: any) => {
  switch (unitType.value) {
    case 'B':
      return Number(bytes.toFixed(2))
    case 'KB':
      return Number((bytes / 1024).toFixed(2))
    case 'MB':
      return Number((bytes / 1024 / 1024).toFixed(2))
    case 'GB':
      return Number((bytes / 1024 / 1024 / 1024).toFixed(2))
    default:
      return 0
  }
}

const clearCurrent = () => {
  total.netBytesSent = 0
  total.netBytesRecv = 0
  total.diskReadBytes = 0
  total.diskWriteBytes = 0
  total.diskRWBytes = 0
  total.diskRWTime = 0
  netBytesSent.value = []
  netBytesRecv.value = []
  timeNetData.value = []
  diskReadBytes.value = []
  diskWriteBytes.value = []
  timeDiskData.value = []
}

const quantifier = computed(() => {
  switch (locale) {
    case 'zh_CN':
      return '个'
    case 'zh_TW':
      return '個'
    default:
      return ''
  }
})

let homeInterval: any = null

onMounted(() => {
  fetchCurrent()
  homeInterval = setInterval(() => {
    fetchCurrent()
  }, 3000)
})

onUnmounted(() => {
  clearInterval(homeInterval)
})

if (import.meta.hot) {
  import.meta.hot.accept()
  import.meta.hot.dispose(() => {
    clearInterval(homeInterval)
  })
}
</script>

<template>
  <PageContainer :show-footer="true" bare class="min-w-94">
    <div flex-1>
      <n-space vertical>
        <!-- 系统是否 EOL -->
        <n-alert type="info" v-if="systemInfo.os_eol">
          {{
            $gettext(
              'Your operating system %{ os_name } has reached its end-of-life. Please consider upgrading to a supported version to ensure optimal performance and security.',
              {
                os_name: systemInfo?.os_name,
              },
            )
          }}
        </n-alert>
        <!-- 系统是否不受支持 -->
        <n-alert type="warning" v-if="!systemInfo.os_supported">
          {{
            $gettext(
              'Your operating system %{ os_name } is not officially supported. Some features may not work as expected. Please consider using a supported operating system for the best experience.',
              {
                os_name: systemInfo?.os_name,
              },
            )
          }}
        </n-alert>
        <n-card :segmented="true" size="small">
          <n-page-header :subtitle="systemInfo?.panel_version">
            <n-grid :cols="countInfo.container >= 0 ? 5 : 4" pb-10>
              <n-gi v-if="countInfo.container >= 0">
                <n-statistic
                  :label="$gettext('Container')"
                  :value="countInfo.container + quantifier"
                />
              </n-gi>
              <n-gi>
                <n-statistic :label="$gettext('Website')" :value="countInfo.website + quantifier" />
              </n-gi>
              <n-gi>
                <n-statistic
                  :label="$gettext('Database')"
                  :value="countInfo.database + quantifier"
                />
              </n-gi>
              <n-gi>
                <n-statistic :label="$gettext('Project')" :value="countInfo.project + quantifier" />
              </n-gi>
              <n-gi>
                <n-statistic
                  :label="$gettext('Scheduled Tasks')"
                  :value="countInfo.cron + quantifier"
                />
              </n-gi>
            </n-grid>
            <template #title>{{ $gettext('AcePanel') }}</template>
            <template #extra>
              <n-flex>
                <n-button type="primary" @click="toSponsor">
                  {{ $gettext('Sponsor Support') }}
                </n-button>
                <n-dropdown :options="restartOptions" @select="handleRestartSelect">
                  <n-button type="warning"> {{ $gettext('Restart') }} </n-button>
                </n-dropdown>
                <n-button
                  type="info"
                  :loading="checkUpdateLoading"
                  :disabled="checkUpdateLoading"
                  @click="handleUpdate"
                >
                  {{ $gettext('Update') }}
                </n-button>
              </n-flex>
            </template>
          </n-page-header>
        </n-card>

        <n-card :segmented="true" size="small" :title="$gettext('Resource Overview')">
          <n-flex v-if="realtime" size="large">
            <n-popover placement="bottom" trigger="hover">
              <template #trigger>
                <n-flex vertical p-5 pl-10 pr-10 flex items-center>
                  <p>{{ $gettext('Load Status') }}</p>
                  <n-progress
                    type="dashboard"
                    :percentage="Math.round(formatPercent((realtime.load.load1 / cores) * 100))"
                    :color="statusColor((realtime.load.load1 / cores) * 100)"
                  >
                  </n-progress>
                  <p>{{ statusText((realtime.load.load1 / cores) * 100) }}</p>
                </n-flex>
              </template>
              <n-scrollbar max-h-85>
                <n-descriptions label-placement="top" :column="3" bordered size="small">
                  <n-descriptions-item :label="$gettext('Last 1 minute')">
                    {{ formatPercent((realtime.load.load1 / cores) * 100) }}% /
                    {{ realtime.load.load1 }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Last 5 minutes')">
                    {{ formatPercent((realtime.load.load5 / cores) * 100) }}% /
                    {{ realtime.load.load5 }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Last 15 minutes')">
                    {{ formatPercent((realtime.load.load15 / cores) * 100) }}% /
                    {{ realtime.load.load15 }}
                  </n-descriptions-item>
                </n-descriptions>
              </n-scrollbar>
            </n-popover>
            <n-popover placement="bottom" trigger="hover">
              <template #trigger>
                <n-flex vertical p-5 pl-10 pr-10 flex items-center>
                  <p>CPU</p>
                  <n-progress
                    type="dashboard"
                    :percentage="realtime.percent"
                    :color="statusColor(realtime.percent)"
                  >
                  </n-progress>
                  <p>{{ cores }} {{ $gettext('cores') }}</p>
                </n-flex>
              </template>
              <n-scrollbar max-h-110>
                <n-descriptions label-placement="top" :column="3" bordered size="small">
                  <n-descriptions-item :label="$gettext('Model')" :span="3">
                    {{ realtime.cpus[0]?.modelName }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('CPU Count')">
                    {{ realtime.cpus.length }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Cores')">
                    {{ cores }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Cache')">
                    {{ formatBytes((realtime.cpus[0]?.cacheSize ?? 0) * 1024) }}
                  </n-descriptions-item>
                  <n-descriptions-item
                    v-for="item in realtime.cpus"
                    :key="item.modelName"
                    :label="'CPU-' + item.cpu"
                  >
                    {{ formatPercent(realtime.percents[item.cpu]) }}% / {{ item.mhz }} MHz
                  </n-descriptions-item>
                </n-descriptions>
                <n-divider style="margin: 8px 0" />
                <n-flex justify="center">
                  <n-button
                    text
                    type="primary"
                    size="small"
                    :loading="cpuTopLoading"
                    @click="loadTopProcesses('cpu')"
                  >
                    {{ $gettext('CPU Top5 Processes') }} {{ cpuTopVisible ? '∧' : '>' }}
                  </n-button>
                </n-flex>
                <n-table
                  v-if="cpuTopVisible && cpuTopProcesses.length"
                  :single-line="false"
                  size="small"
                  striped
                  class="mt-2"
                >
                  <thead>
                    <tr>
                      <th>PID</th>
                      <th>{{ $gettext('Name') }}</th>
                      <th>CPU</th>
                      <th>{{ $gettext('User') }}</th>
                      <th>{{ $gettext('Command') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="proc in cpuTopProcesses" :key="proc.pid">
                      <td>{{ proc.pid }}</td>
                      <td>
                        <n-ellipsis style="max-width: 120px">
                          {{ proc.name }}
                        </n-ellipsis>
                      </td>
                      <td>{{ proc.value.toFixed(1) }}%</td>
                      <td>{{ proc.username }}</td>
                      <td>
                        <n-ellipsis style="max-width: 200px">
                          {{ proc.command }}
                        </n-ellipsis>
                      </td>
                    </tr>
                  </tbody>
                </n-table>
              </n-scrollbar>
            </n-popover>
            <n-popover placement="bottom" trigger="hover">
              <template #trigger>
                <n-flex vertical p-5 pl-10 pr-10 flex items-center>
                  <p>{{ $gettext('Memory') }}</p>
                  <n-progress
                    type="dashboard"
                    :percentage="realtime.mem.usedPercent"
                    :color="statusColor(realtime.mem.usedPercent)"
                  >
                  </n-progress>
                  <p>{{ formatBytes(realtime.mem.total) }}</p>
                </n-flex>
              </template>
              <n-scrollbar max-h-110>
                <n-descriptions label-placement="top" :column="4" bordered size="small">
                  <n-descriptions-item :label="$gettext('Physical Memory Size')">
                    {{ formatBytes(realtime.mem.total) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Physical Memory Used')">
                    {{ formatBytes(realtime.mem.used) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Physical Memory Available')">
                    {{ formatBytes(realtime.mem.available) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Free')">
                    {{ formatBytes(realtime.mem.free) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Active')">
                    {{ formatBytes(realtime.mem.active) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Inactive')">
                    {{ formatBytes(realtime.mem.inactive) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Shared')">
                    {{ formatBytes(realtime.mem.shared) }}
                  </n-descriptions-item>
                  <n-descriptions-item label="buffers/cached">
                    {{ formatBytes(realtime.mem.buffers) }} /
                    {{ formatBytes(realtime.mem.cached) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Committed')">
                    {{ formatBytes(realtime.mem.committedAS) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Commit Limit')">
                    {{ formatBytes(realtime.mem.commitLimit) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('SWAP Size')">
                    {{ formatBytes(realtime.mem.swapTotal) }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('SWAP Used')">
                    {{ formatBytes(realtime.mem.swapCached) }}
                  </n-descriptions-item>
                </n-descriptions>
                <n-divider style="margin: 8px 0" />
                <n-flex justify="center">
                  <n-button
                    text
                    type="primary"
                    size="small"
                    :loading="memTopLoading"
                    @click="loadTopProcesses('memory')"
                  >
                    {{ $gettext('Memory Top5 Processes') }} {{ memTopVisible ? '∧' : '>' }}
                  </n-button>
                </n-flex>
                <n-table
                  v-if="memTopVisible && memTopProcesses.length"
                  :single-line="false"
                  size="small"
                  striped
                  class="mt-2"
                >
                  <thead>
                    <tr>
                      <th>PID</th>
                      <th>{{ $gettext('Name') }}</th>
                      <th>{{ $gettext('Memory') }}</th>
                      <th>{{ $gettext('User') }}</th>
                      <th>{{ $gettext('Command') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="proc in memTopProcesses" :key="proc.pid">
                      <td>{{ proc.pid }}</td>
                      <td>
                        <n-ellipsis style="max-width: 120px">
                          {{ proc.name }}
                        </n-ellipsis>
                      </td>
                      <td>{{ formatBytes(proc.value) }}</td>
                      <td>{{ proc.username }}</td>
                      <td>
                        <n-ellipsis style="max-width: 200px">
                          {{ proc.command }}
                        </n-ellipsis>
                      </td>
                    </tr>
                  </tbody>
                </n-table>
              </n-scrollbar>
            </n-popover>
            <n-popover
              v-for="item in realtime.disk_usage"
              :key="item.path"
              placement="bottom"
              trigger="hover"
            >
              <template #trigger>
                <n-flex vertical p-5 pl-10 pr-10 flex items-center>
                  <p>{{ item.path }}</p>
                  <n-progress
                    type="dashboard"
                    :percentage="formatPercent(item.usedPercent)"
                    :color="statusColor(item.usedPercent)"
                  >
                  </n-progress>
                  <p>{{ formatBytes(item.used) }} / {{ formatBytes(item.total) }}</p>
                </n-flex>
              </template>
              <n-scrollbar max-h-85>
                <n-descriptions label-placement="top" :column="3" bordered size="small">
                  <n-descriptions-item :label="$gettext('Mount Point')">
                    {{ item.path }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('File System')">
                    {{ item.fstype }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Inodes Usage')">
                    {{ formatPercent(item.inodesUsedPercent) }}%
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Inodes Total')">
                    {{ item.inodesTotal }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Inodes Used')">
                    {{ item.inodesUsed }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Inodes Available')">
                    {{ item.inodesFree }}
                  </n-descriptions-item>
                </n-descriptions>
              </n-scrollbar>
            </n-popover>
          </n-flex>
          <n-skeleton v-else text :repeat="10" />
        </n-card>
        <n-grid
          x-gap="12"
          y-gap="12"
          cols="1 s:1 m:1 l:2 xl:2 2xl:2"
          item-responsive
          responsive="screen"
        >
          <n-gi>
            <n-flex vertical>
              <n-card :segmented="true" size="small" :title="$gettext('Quick Apps')" min-h-85>
                <n-scrollbar class="max-h-68">
                  <draggable
                    v-if="!appLoading"
                    v-model="apps"
                    item-key="slug"
                    handle=".drag-handle"
                    class="app-grid"
                    @end="handleAppOrderChange"
                  >
                    <template #item="{ element: item }">
                      <div relative>
                        <n-card
                          :segmented="true"
                          size="small"
                          cursor-pointer
                          class="app-card"
                          @click="handleManageApp(item.slug)"
                        >
                          <div class="drag-handle" cursor-grab>
                            <the-icon icon="mdi:drag" :size="20" />
                          </div>
                          <n-flex>
                            <n-thing>
                              <template #avatar>
                                <div class="mt-2">
                                  <the-icon-local type="app" :size="30" :icon="item.slug" />
                                </div>
                              </template>
                              <template #header>
                                {{ item.name }}
                              </template>
                              <template #description>
                                {{ item.version }}
                              </template>
                            </n-thing>
                          </n-flex>
                        </n-card>
                      </div>
                    </template>
                  </draggable>
                </n-scrollbar>
                <n-text v-if="!appLoading && !apps.length">
                  {{ $gettext('You have not set any apps to display here!') }}
                </n-text>
                <n-skeleton v-if="appLoading" text :repeat="12" />
              </n-card>
              <n-card :segmented="true" size="small" :title="$gettext('Environment Information')">
                <n-table v-if="systemInfo" :single-line="false">
                  <tr>
                    <th>{{ $gettext('System Hostname') }}</th>
                    <td>
                      {{ systemInfo?.hostname || $gettext('Loading...') }}
                    </td>
                  </tr>
                  <tr>
                    <th>{{ $gettext('System Version') }}</th>
                    <td>
                      {{
                        `${systemInfo?.os_name} ${systemInfo?.kernel_arch}` ||
                        $gettext('Loading...')
                      }}
                    </td>
                  </tr>
                  <tr>
                    <th>{{ $gettext('System Kernel Version') }}</th>
                    <td>
                      {{ systemInfo?.kernel_version || $gettext('Loading...') }}
                    </td>
                  </tr>
                  <tr>
                    <th>{{ $gettext('System Uptime') }}</th>
                    <td>
                      {{ formatDuration(Number(systemInfo?.uptime)) || $gettext('Loading...') }}
                    </td>
                  </tr>
                  <tr>
                    <th>{{ $gettext('Panel Internal Version') }}</th>
                    <td>
                      {{
                        systemInfo?.commit_hash +
                          ' ' +
                          systemInfo?.go_version +
                          ' ' +
                          systemInfo?.build_time || $gettext('Loading...')
                      }}
                    </td>
                  </tr>
                  <tr>
                    <th>{{ $gettext('Panel Compile Information') }}</th>
                    <td>
                      {{
                        systemInfo?.build_id +
                          ' ' +
                          systemInfo?.build_user +
                          '/' +
                          systemInfo?.build_host || $gettext('Loading...')
                      }}
                    </td>
                  </tr>
                </n-table>
                <n-skeleton v-else text :repeat="9" />
              </n-card>
            </n-flex>
          </n-gi>
          <n-gi>
            <n-card :segmented="true" size="small" :title="$gettext('Real-time Monitoring')">
              <n-flex vertical v-if="systemInfo">
                <n-form
                  inline
                  label-placement="left"
                  label-width="auto"
                  require-mark-placement="right-hanging"
                >
                  <n-form-item>
                    <n-radio-group v-model:value="chartType">
                      <n-radio-button value="net" :label="$gettext('Network')" />
                      <n-radio-button value="disk" :label="$gettext('Disk')" />
                    </n-radio-group>
                  </n-form-item>
                  <n-form-item :label="$gettext('Unit')" ml-auto>
                    <n-select
                      v-model:value="unitType"
                      :options="units"
                      @update-value="clearCurrent"
                      w-20
                    ></n-select>
                  </n-form-item>
                  <n-form-item v-if="chartType == 'net'" :label="$gettext('Network Card')">
                    <n-select
                      multiple
                      v-model:value="nets"
                      :options="systemInfo.nets"
                      @update-value="clearCurrent"
                      w-50
                    ></n-select>
                  </n-form-item>
                  <n-form-item v-if="chartType == 'disk'" :label="$gettext('Disk')">
                    <n-select
                      multiple
                      v-model:value="disks"
                      :options="systemInfo.disks"
                      @update-value="clearCurrent"
                      w-50
                    ></n-select>
                  </n-form-item>
                </n-form>
                <n-flex v-if="chartType == 'net'">
                  <n-tag>{{ $gettext('Total Sent') }} {{ formatBytes(total.netBytesSent) }} </n-tag>
                  <n-tag>
                    {{ $gettext('Total Received') }} {{ formatBytes(total.netBytesRecv) }}
                  </n-tag>
                  <n-tag>
                    {{ $gettext('Real-time Sent') }}
                    {{ formatBytes(current.netBytesSent) }}/s
                  </n-tag>
                  <n-tag
                    >{{ $gettext('Real-time Received') }} {{ formatBytes(current.netBytesRecv) }}/s
                  </n-tag>
                </n-flex>
                <n-flex v-if="chartType == 'disk'">
                  <n-tag>{{ $gettext('Read') }} {{ formatBytes(total.diskReadBytes) }}</n-tag>
                  <n-tag>{{ $gettext('Write') }} {{ formatBytes(total.diskWriteBytes) }}</n-tag>
                  <n-tag
                    >{{ $gettext('Real-time Read/Write') }}
                    {{ formatBytes(current.diskRWBytes) }}/s</n-tag
                  >
                  <n-tag>{{ $gettext('Read/Write Latency') }} {{ current.diskRWTime }}ms</n-tag>
                </n-flex>
                <n-card :bordered="false" class="pt-10 h-132">
                  <v-chart class="chart" :option="chartOptions" autoresize />
                </n-card>
              </n-flex>
              <n-skeleton v-else text :repeat="25" />
            </n-card>
          </n-gi>
        </n-grid>
      </n-space>
    </div>
  </PageContainer>

  <!-- 服务器重启确认弹窗 -->
  <n-modal
    v-model:show="showRestartServerConfirm"
    preset="dialog"
    type="warning"
    :title="$gettext('Restart Server')"
    :content="
      $gettext('Are you sure you want to restart the server? This will disconnect all connections.')
    "
    :positive-text="$gettext('Confirm')"
    :negative-text="$gettext('Cancel')"
    @positive-click="handleRestartServer"
  />
</template>

<style scoped>
.app-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  padding: 10px;
}

@media (max-width: 1200px) {
  .app-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 900px) {
  .app-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 600px) {
  .app-grid {
    grid-template-columns: 1fr;
  }
}

.app-card {
  transition: box-shadow 0.3s ease;
}

.app-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.drag-handle {
  position: absolute;
  top: 8px;
  right: 8px;
  opacity: 0;
  transition: opacity 0.2s ease;
  z-index: 10;
  color: #999;
}

.app-card:hover .drag-handle {
  opacity: 1;
}
</style>

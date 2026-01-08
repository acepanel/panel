<script setup lang="ts">
defineOptions({
  name: 'monitor-index'
})

import { LineChart } from 'echarts/charts'
import {
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent
} from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { NButton } from 'naive-ui'
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
  DataZoomComponent
])

const start = ref(Math.floor(new Date(new Date().setHours(0, 0, 0, 0)).getTime()))
const end = ref(Math.floor(Date.now()))

useRequest(monitor.setting()).onSuccess(({ data }) => {
  monitorSwitch.value = data.enabled
  saveDay.value = data.days
})

const { loading, data } = useWatcher(monitor.list(start.value, end.value), [start, end], {
  initialData: {
    times: [],
    load: {},
    cpu: {},
    mem: {},
    swap: {},
    net: {}
  },
  debounce: [500],
  immediate: true
})

const monitorSwitch = ref(false)
const saveDay = ref(30)

const load = ref<any>({
  title: {
    text: $gettext('Load'),
    textAlign: 'left',
    textStyle: {
      fontSize: 20
    }
  },
  tooltip: {
    trigger: 'axis'
  },
  legend: {
    align: 'left',
    data: [$gettext('1 minute'), $gettext('5 minutes'), $gettext('15 minutes')]
  },
  xAxis: [{ type: 'category', boundaryGap: false, data: data.value.times }],
  yAxis: [
    {
      type: 'value'
    }
  ],
  dataZoom: {
    show: true,
    realtime: true,
    start: 0,
    end: 100
  },
  series: [
    {
      name: $gettext('1 minute'),
      type: 'line',
      smooth: true,
      data: data.value.load.load1,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    },
    {
      name: $gettext('5 minutes'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.load.load5,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    },
    {
      name: $gettext('15 minutes'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.load.load15,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    }
  ]
})

const cpu = ref<any>({
  title: {
    text: 'CPU',
    textAlign: 'left',
    textStyle: {
      fontSize: 20
    }
  },
  tooltip: {
    trigger: 'axis'
  },
  xAxis: [{ type: 'category', boundaryGap: false, data: data.value.times }],
  yAxis: [
    {
      name: $gettext('Unit %'),
      min: 0,
      max: 100,
      type: 'value',
      axisLabel: {
        formatter: '{value} %'
      }
    }
  ],
  dataZoom: {
    show: true,
    realtime: true,
    start: 0,
    end: 100
  },
  series: [
    {
      name: $gettext('Usage'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.cpu.percent,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    }
  ]
})

const mem = ref<any>({
  title: {
    text: $gettext('Memory'),
    textAlign: 'left',
    textStyle: {
      fontSize: 20
    }
  },
  tooltip: {
    trigger: 'axis'
  },
  legend: {
    align: 'left',
    data: [$gettext('Memory'), 'Swap']
  },
  xAxis: [{ type: 'category', boundaryGap: false, data: data.value.times }],
  yAxis: [
    {
      name: $gettext('Unit MB'),
      min: 0,
      max: data.value.mem.total,
      type: 'value',
      axisLabel: {
        formatter: '{value} M'
      }
    }
  ],
  dataZoom: {
    show: true,
    realtime: true,
    start: 0,
    end: 100
  },
  series: [
    {
      name: $gettext('Memory'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.mem.used,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    },
    {
      name: 'Swap',
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.swap.used,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    }
  ]
})

const net = ref<any>({
  title: {
    text: $gettext('Network'),
    textAlign: 'left',
    textStyle: {
      fontSize: 20
    }
  },
  tooltip: {
    trigger: 'axis'
  },
  legend: {
    align: 'left',
    data: [
      $gettext('Total Out'),
      $gettext('Total In'),
      $gettext('Per Second Out'),
      $gettext('Per Second In')
    ]
  },
  xAxis: [{ type: 'category', boundaryGap: false, data: data.value.times }],
  yAxis: [
    {
      name: $gettext('Unit MB'),
      type: 'value',
      axisLabel: {
        formatter: '{value} MB'
      }
    }
  ],
  dataZoom: {
    show: true,
    realtime: true,
    start: 0,
    end: 100
  },
  series: [
    {
      name: $gettext('Total Out'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.net.sent,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    },
    {
      name: $gettext('Total In'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.net.recv,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    },
    {
      name: $gettext('Per Second Out'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.net.tx,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    },
    {
      name: $gettext('Per Second In'),
      type: 'line',
      smooth: true,
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      },
      data: data.value.net.rx,
      markPoint: {
        data: [
          { type: 'max', name: $gettext('Maximum') },
          { type: 'min', name: $gettext('Minimum') }
        ]
      },
      markLine: {
        data: [{ type: 'average', name: $gettext('Average') }]
      }
    }
  ]
})

const handleUpdate = async () => {
  useRequest(monitor.updateSetting(monitorSwitch.value, saveDay.value)).onSuccess(() => {
    window.$message.success($gettext('Operation successful'))
  })
}

const handleClear = async () => {
  useRequest(monitor.clear()).onSuccess(() => {
    window.$message.success($gettext('Operation successful'))
  })
}

// 监听 data 的变化
watch(data, () => {
  load.value.xAxis[0].data = data.value.times
  load.value.series[0].data = data.value.load.load1
  load.value.series[1].data = data.value.load.load5
  load.value.series[2].data = data.value.load.load15
  cpu.value.xAxis[0].data = data.value.times
  cpu.value.series[0].data = data.value.cpu.percent
  mem.value.xAxis[0].data = data.value.times
  mem.value.yAxis[0].max = data.value.mem.total
  mem.value.series[0].data = data.value.mem.used
  mem.value.series[1].data = data.value.swap.used
  net.value.xAxis[0].data = data.value.times
  net.value.series[0].data = data.value.net.sent
  net.value.series[1].data = data.value.net.recv
  net.value.series[2].data = data.value.net.tx
  net.value.series[3].data = data.value.net.rx
})
</script>

<template>
  <common-page show-header show-footer>
    <template #tabbar>
      <div class="py-4 flex gap-8 items-center justify-between">
        <div class="flex gap-6 items-center">
          <div class="flex gap-10 items-center">
            {{ $gettext('Enable Monitoring') }}
            <n-switch v-model:value="monitorSwitch" @update-value="handleUpdate" />
          </div>
          <div class="pl-20 flex gap-10 items-center">
            {{ $gettext('Save Days') }}
            <n-input-number v-model:value="saveDay">
              <template #suffix> {{ $gettext('days') }} </template>
            </n-input-number>
          </div>
          <div>
            <n-button type="primary" @click="handleUpdate">{{ $gettext('Confirm') }}</n-button>
          </div>
        </div>

        <div class="flex gap-10 items-center">
          <span>{{ $gettext('Time Selection') }}</span>
          <div class="flex gap-2 items-center">
            <n-date-picker v-model:value="start" type="datetime" />
            <span class="mx-1">-</span>
            <n-date-picker v-model:value="end" type="datetime" />
          </div>
          <n-popconfirm @positive-click="handleClear">
            <template #trigger>
              <n-button type="error">
                {{ $gettext('Clear Monitoring Records') }}
              </n-button>
            </template>
            {{ $gettext('Are you sure you want to clear?') }}
          </n-popconfirm>
        </div>
      </div>
    </template>
    <n-grid
      v-if="!loading"
      cols="1 s:1 m:1 l:2 xl:2 2xl:2"
      item-responsive
      responsive="screen"
      pt-20
    >
      <n-gi m-10>
        <n-card :bordered="false" style="height: 40vh">
          <v-chart class="chart" :option="load" autoresize />
        </n-card>
      </n-gi>
      <n-gi m-10>
        <n-card :bordered="false" style="height: 40vh">
          <v-chart class="chart" :option="cpu" autoresize />
        </n-card>
      </n-gi>
      <n-gi m-10>
        <n-card :bordered="false" style="height: 40vh">
          <v-chart class="chart" :option="mem" autoresize />
        </n-card>
      </n-gi>
      <n-gi m-10>
        <n-card :bordered="false" style="height: 40vh">
          <v-chart class="chart" :option="net" autoresize />
        </n-card>
      </n-gi>
    </n-grid>
    <n-skeleton v-else text :repeat="40" />
  </common-page>
</template>

<style scoped lang="scss"></style>

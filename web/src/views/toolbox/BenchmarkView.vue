<script setup lang="ts">
defineOptions({
  name: 'toolbox-benchmark',
})

import { useGettext } from 'vue3-gettext'

import benchmark from '@/api/panel/toolbox-benchmark'
import type {
  BenchmarkCpu,
  BenchmarkDisk,
  BenchmarkMachine,
  BenchmarkMemory,
} from '@/api/panel/toolbox-benchmark'
import { formatBytes } from '@/utils/file'

const { $gettext } = useGettext()

// 1000 分对应主流云服务器的水平，条形图上画一道参考线
const BASELINE = 1000

const cpuTests = [
  { name: 'image', title: $gettext('Image Processing') },
  { name: 'matrix', title: $gettext('Matrix Multiplication') },
  { name: 'sort', title: $gettext('Data Sorting') },
  { name: 'encryption', title: $gettext('AES Encryption') },
  { name: 'compression', title: $gettext('Compression/Decompression') },
  { name: 'physics', title: $gettext('Physics Simulation') },
  { name: 'json', title: $gettext('JSON Parsing') },
]
const tests = [
  ...cpuTests,
  { name: 'memory', title: $gettext('Memory') },
  { name: 'disk', title: $gettext('Disk') },
]

const { data: machineData } = useRequest(benchmark.machine)
const machine = computed(() => machineData.value as BenchmarkMachine | undefined)

const running = ref(false)
const current = ref('')
const done = ref(0)

const cpu = ref<Record<string, BenchmarkCpu>>({})
const memory = ref<BenchmarkMemory>()
const disk = ref<BenchmarkDisk>()
// 没测出来的项目及原因，键为测试名
const failures = ref<Record<string, string>>({})

// 等权几何平均：各项分数量级一致，任何一项都压不倒总分
const geomean = (scores: number[]) =>
  Math.round(Math.exp(scores.reduce((sum, score) => sum + Math.log(score), 0) / scores.length))

// 七项都测完才有总分
const cpuTotal = computed(() => {
  const results = cpuTests.flatMap((test) => cpu.value[test.name] ?? [])
  const [first] = results
  if (!first || results.length < cpuTests.length) return undefined
  return {
    single: geomean(results.map((result) => result.single)),
    multi: geomean(results.map((result) => result.multi)),
    threads: first.threads,
  }
})
const cpuFailure = computed(
  () => cpuTests.map((test) => failures.value[test.name]).find((reason) => reason) ?? '',
)

const threads = computed(() => cpuTotal.value?.threads ?? machine.value?.threads)

const summary = computed(() => [
  {
    key: 'single',
    label: $gettext('Single-Core'),
    icon: 'mdi:cpu-64-bit',
    score: cpuTotal.value?.single,
    failure: cpuFailure.value,
  },
  {
    key: 'multi',
    label: threads.value
      ? $gettext('Multi-Core (×%{ threads })', { threads: threads.value })
      : $gettext('Multi-Core'),
    icon: 'mdi:chip',
    score: cpuTotal.value?.multi,
    failure: cpuFailure.value,
  },
  {
    key: 'memory',
    label: $gettext('Memory'),
    icon: 'mdi:memory',
    score: memory.value?.score,
    failure: failures.value.memory,
  },
  {
    key: 'disk',
    label: $gettext('Disk'),
    icon: 'mdi:harddisk',
    score: disk.value?.score,
    failure: failures.value.disk,
  },
])

// 条形图满格至少是两倍基准，有更高的成绩时按最高分放大
const barScale = computed(() =>
  Math.max(
    2 * BASELINE,
    ...cpuTests.flatMap((test) => {
      const result = cpu.value[test.name]
      return result ? [result.single, result.multi] : []
    }),
  ),
)
const barWidth = (score?: number) => `${score ? (score / barScale.value) * 100 : 0}%`
const baselineLeft = computed(() => `${(BASELINE / barScale.value) * 100}%`)

// 数值与单位分开排版，单位用小一号的字
const splitBytes = (bytes?: number) => {
  if (bytes === undefined) return { value: '-', unit: '' }
  const [value = '-', unit = ''] = String(formatBytes(bytes)).split(' ')
  return { value, unit: `${unit}/s` }
}
const memoryMetrics = computed(() => [
  { label: $gettext('Memory Bandwidth'), ...splitBytes(memory.value?.bandwidth) },
  {
    label: $gettext('Memory Latency'),
    value: memory.value ? memory.value.latency.toFixed(1) : '-',
    unit: memory.value ? 'ns' : '',
  },
])
const iops = (value?: number) => ({
  value: value === undefined ? '-' : Math.round(value).toLocaleString(),
  unit: value === undefined ? '' : 'IOPS',
})
const diskMetrics = computed(() => [
  { label: $gettext('Sequential Read'), ...splitBytes(disk.value?.seq_read) },
  { label: $gettext('Sequential Write'), ...splitBytes(disk.value?.seq_write) },
  { label: $gettext('4K Random Read'), ...iops(disk.value?.rand_read) },
  { label: $gettext('4K Random Write'), ...iops(disk.value?.rand_write) },
])

let mounted = true
onBeforeUnmount(() => {
  mounted = false
})

const handleTest = async () => {
  running.value = true
  done.value = 0
  cpu.value = {}
  memory.value = undefined
  disk.value = undefined
  failures.value = {}

  for (const test of tests) {
    // 离开页面后不再让服务器白跑剩下的项目
    if (!mounted) break
    current.value = test.title
    try {
      const data = await benchmark.test(test.name)
      if (test.name === 'memory') memory.value = data
      else if (test.name === 'disk') disk.value = data
      else cpu.value[test.name] = data
    } catch (error: any) {
      failures.value[test.name] = error.message
    }
    done.value++
  }
  running.value = false
}
</script>

<template>
  <n-flex vertical :size="16">
    <n-card size="small">
      <n-flex align="center" justify="space-between" :size="16">
        <n-flex align="center" :size="12" :wrap="false" class="min-w-0">
          <div
            class="text-brand rounded-lg bg-brand-subtle flex flex-none size-11 items-center justify-center"
          >
            <the-icon icon="mdi:speedometer" :size="24" />
          </div>
          <div class="min-w-0">
            <div class="text-15px font-medium truncate">
              {{ machine?.model || $gettext('Unknown') }}
            </div>
            <n-flex :size="[16, 0]" class="text-13px text-text-tertiary">
              <span v-if="machine?.cores">{{ $gettext('Cores') }} {{ machine.cores }}</span>
              <span v-if="machine?.threads">{{ $gettext('Threads') }} {{ machine.threads }}</span>
              <span v-if="machine?.memory">
                {{ $gettext('Memory') }} {{ formatBytes(machine.memory) }}
              </span>
            </n-flex>
          </div>
        </n-flex>
        <n-button type="primary" :loading="running" :disabled="running" @click="handleTest">
          {{ running ? $gettext('Benchmarking...') : $gettext('Start Benchmark') }}
        </n-button>
      </n-flex>
      <div v-if="running" class="mt-4">
        <n-flex justify="space-between" class="text-13px text-text-secondary mb-1.5">
          <span>{{ $gettext('Current project: %{ current }', { current }) }}</span>
          <span class="tabular-nums">{{ done }} / {{ tests.length }}</span>
        </n-flex>
        <n-progress
          :percentage="Math.round((done / tests.length) * 100)"
          :show-indicator="false"
          :height="6"
          color="var(--color-brand)"
          processing
        />
      </div>
      <div class="text-12px text-text-tertiary mt-3">
        {{ $gettext('A score of 1000 is roughly the level of a mainstream cloud server.') }}
        {{
          $gettext(
            'Benchmark results are for reference only and may differ from actual performance due to system resource scheduling, caching, and other factors!',
          )
        }}
      </div>
    </n-card>

    <n-grid cols="2 l:4" responsive="screen" :x-gap="16" :y-gap="16">
      <n-gi v-for="item in summary" :key="item.key">
        <n-card size="small" class="h-full">
          <n-flex align="center" :size="8" :wrap="false">
            <div
              class="text-brand rounded-md bg-brand-subtle flex flex-none size-7 items-center justify-center"
            >
              <the-icon :icon="item.icon" :size="16" />
            </div>
            <span class="text-13px text-text-secondary truncate">{{ item.label }}</span>
          </n-flex>
          <div class="text-28px leading-9 font-semibold mt-2 tabular-nums">
            <n-number-animation v-if="item.score" :from="0" :to="item.score" show-separator />
            <span v-else class="text-text-disabled">-</span>
          </div>
          <div v-if="item.failure" class="text-12px text-error mt-1 truncate" :title="item.failure">
            {{ item.failure }}
          </div>
          <div v-else-if="!item.score" class="text-12px text-text-tertiary mt-1">
            {{ $gettext('Pending benchmark') }}
          </div>
        </n-card>
      </n-gi>
    </n-grid>

    <n-grid cols="1 l:5" responsive="screen" :x-gap="16" :y-gap="16">
      <n-gi span="1 l:3">
        <n-card size="small" :title="$gettext('CPU')" class="h-full">
          <n-alert v-if="cpuFailure" type="error" class="mb-3">{{ cpuFailure }}</n-alert>
          <div class="flex flex-col gap-3.5">
            <!-- 表头与下面的分项共用列宽，图例对齐各自的分数列，1000 标在参考线正上方 -->
            <div
              class="text-12px text-text-tertiary gap-x-4 grid grid-cols-[1fr_5rem_5rem] items-center md:grid-cols-[8rem_1fr_5rem_5rem]"
            >
              <span />
              <div class="h-4 hidden relative md:block">
                <span class="leading-4 absolute -translate-x-1/2" :style="{ left: baselineLeft }">
                  {{ BASELINE }}
                </span>
              </div>
              <n-flex align="center" justify="end" :size="6" :wrap="false">
                <span class="rounded-full bg-brand flex-none size-2" />
                {{ $gettext('Single-Core') }}
              </n-flex>
              <n-flex align="center" justify="end" :size="6" :wrap="false">
                <span class="rounded-full bg-brand opacity-40 flex-none size-2" />
                {{ $gettext('Multi-Core') }}
              </n-flex>
            </div>
            <div
              v-for="test in cpuTests"
              :key="test.name"
              class="gap-x-4 gap-y-1.5 grid grid-cols-[1fr_5rem_5rem] items-center md:grid-cols-[8rem_1fr_5rem_5rem]"
            >
              <span class="text-13px truncate">{{ test.title }}</span>
              <div class="col-span-3 row-start-2 relative md:col-span-1 md:row-start-auto">
                <div class="rounded-full bg-bg-subtle h-1.5 overflow-hidden">
                  <div
                    class="rounded-full bg-brand h-full transition-all duration-500"
                    :style="{ width: barWidth(cpu[test.name]?.single) }"
                  />
                </div>
                <div class="mt-1 rounded-full bg-bg-subtle h-1.5 overflow-hidden">
                  <div
                    class="rounded-full bg-brand opacity-40 h-full transition-all duration-500"
                    :style="{ width: barWidth(cpu[test.name]?.multi) }"
                  />
                </div>
                <span
                  class="border-l border-text-tertiary border-dashed absolute -inset-y-0.5"
                  :style="{ left: baselineLeft }"
                />
              </div>
              <span class="text-13px text-right tabular-nums">
                {{ cpu[test.name]?.single.toLocaleString() ?? '-' }}
              </span>
              <span class="text-13px text-text-secondary text-right tabular-nums">
                {{ cpu[test.name]?.multi.toLocaleString() ?? '-' }}
              </span>
            </div>
          </div>
        </n-card>
      </n-gi>
      <n-gi span="1 l:2">
        <n-flex vertical :size="16" class="h-full">
          <n-card
            v-for="group in [
              { key: 'memory', title: $gettext('Memory'), metrics: memoryMetrics, cols: 2 },
              { key: 'disk', title: $gettext('Disk'), metrics: diskMetrics, cols: 2 },
            ]"
            :key="group.key"
            size="small"
            :title="group.title"
          >
            <n-alert v-if="failures[group.key]" type="error">{{ failures[group.key] }}</n-alert>
            <div v-else class="gap-3 grid grid-cols-2">
              <div
                v-for="metric in group.metrics"
                :key="metric.label"
                class="px-3 py-2.5 rounded-md bg-bg-subtle"
              >
                <div class="text-12px text-text-tertiary truncate">{{ metric.label }}</div>
                <div class="mt-1 truncate tabular-nums">
                  <span class="text-18px font-medium">{{ metric.value }}</span>
                  <span v-if="metric.unit" class="text-12px text-text-secondary ml-1">
                    {{ metric.unit }}
                  </span>
                </div>
              </div>
            </div>
          </n-card>
        </n-flex>
      </n-gi>
    </n-grid>
  </n-flex>
</template>

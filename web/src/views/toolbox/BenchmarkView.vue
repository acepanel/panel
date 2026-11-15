<script setup lang="ts">
defineOptions({
  name: 'toolbox-benchmark',
})

import { useGettext } from 'vue3-gettext'

import benchmark from '@/api/panel/toolbox-benchmark'
import type { BenchmarkCpu, BenchmarkDisk, BenchmarkMemory } from '@/api/panel/toolbox-benchmark'
import { formatBytes } from '@/utils/file'

const { $gettext } = useGettext()

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

const running = ref(false)
const current = ref('')
const progress = ref(0)

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

const formatScore = (score?: number) => (score === undefined ? '-' : score.toLocaleString())
const formatSpeed = (speed?: number) => (speed === undefined ? '-' : `${formatBytes(speed)}/s`)
const formatIops = (iops?: number) =>
  iops === undefined ? '-' : `${Math.round(iops).toLocaleString()} IOPS`

let mounted = true
onBeforeUnmount(() => {
  mounted = false
})

const handleTest = async () => {
  running.value = true
  progress.value = 0
  cpu.value = {}
  memory.value = undefined
  disk.value = undefined
  failures.value = {}

  for (const [index, test] of tests.entries()) {
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
    progress.value = Math.round(((index + 1) / tests.length) * 100)
  }
  running.value = false
}
</script>

<template>
  <n-flex vertical :size="16">
    <n-alert type="warning">
      {{
        $gettext(
          'Benchmark results are for reference only and may differ from actual performance due to system resource scheduling, caching, and other factors!',
        )
      }}
    </n-alert>
    <template v-if="running">
      <n-alert :title="$gettext('Benchmarking in progress, it may take some time...')" type="info">
        {{ $gettext('Current project: %{ current }', { current: current }) }}
      </n-alert>
      <n-progress :percentage="progress" processing />
    </template>

    <n-card size="small" :title="$gettext('CPU')">
      <template #header-extra>
        <the-icon :size="22" icon="mdi:cpu-64-bit" />
      </template>
      <n-flex vertical :size="16">
        <n-alert v-if="cpuFailure" type="error">{{ cpuFailure }}</n-alert>
        <n-grid v-else :cols="2" :x-gap="16">
          <n-gi>
            <n-statistic :label="$gettext('Single-Core')">
              <n-number-animation v-if="cpuTotal" :from="0" :to="cpuTotal.single" show-separator />
              <span v-else class="text-base text-text-tertiary">
                {{ $gettext('Pending benchmark') }}
              </span>
            </n-statistic>
          </n-gi>
          <n-gi>
            <n-statistic
              :label="
                cpuTotal
                  ? $gettext('Multi-Core (×%{ threads })', { threads: cpuTotal.threads })
                  : $gettext('Multi-Core')
              "
            >
              <n-number-animation v-if="cpuTotal" :from="0" :to="cpuTotal.multi" show-separator />
              <span v-else class="text-base text-text-tertiary">
                {{ $gettext('Pending benchmark') }}
              </span>
            </n-statistic>
          </n-gi>
        </n-grid>
        <n-table :single-line="false" size="small" striped>
          <thead>
            <tr>
              <th>{{ $gettext('Test') }}</th>
              <th>{{ $gettext('Single-Core') }}</th>
              <th>{{ $gettext('Multi-Core') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="test in cpuTests" :key="test.name">
              <td>{{ test.title }}</td>
              <td>{{ formatScore(cpu[test.name]?.single) }}</td>
              <td>{{ formatScore(cpu[test.name]?.multi) }}</td>
            </tr>
          </tbody>
        </n-table>
      </n-flex>
    </n-card>

    <n-grid cols="1 m:2" responsive="screen" :x-gap="16" :y-gap="16">
      <n-gi>
        <n-card size="small" :title="$gettext('Memory')" h-full>
          <template #header-extra>
            <the-icon :size="22" icon="mdi:memory" />
          </template>
          <n-alert v-if="failures.memory" type="error">{{ failures.memory }}</n-alert>
          <n-flex v-else vertical :size="16">
            <n-statistic :label="$gettext('Score')">
              <n-number-animation v-if="memory" :from="0" :to="memory.score" show-separator />
              <span v-else class="text-base text-text-tertiary">
                {{ $gettext('Pending benchmark') }}
              </span>
            </n-statistic>
            <n-table :single-line="false" size="small" striped>
              <tbody>
                <tr>
                  <td>{{ $gettext('Memory Bandwidth') }}</td>
                  <td>{{ formatSpeed(memory?.bandwidth) }}</td>
                </tr>
                <tr>
                  <td>{{ $gettext('Memory Latency') }}</td>
                  <td>{{ memory ? `${memory.latency.toFixed(1)} ns` : '-' }}</td>
                </tr>
              </tbody>
            </n-table>
          </n-flex>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small" :title="$gettext('Disk')" h-full>
          <template #header-extra>
            <the-icon :size="22" icon="mdi:harddisk" />
          </template>
          <n-alert v-if="failures.disk" type="error">{{ failures.disk }}</n-alert>
          <n-flex v-else vertical :size="16">
            <n-statistic :label="$gettext('Score')">
              <n-number-animation v-if="disk" :from="0" :to="disk.score" show-separator />
              <span v-else class="text-base text-text-tertiary">
                {{ $gettext('Pending benchmark') }}
              </span>
            </n-statistic>
            <n-table :single-line="false" size="small" striped>
              <tbody>
                <tr>
                  <td>{{ $gettext('Sequential Read') }}</td>
                  <td>{{ formatSpeed(disk?.seq_read) }}</td>
                </tr>
                <tr>
                  <td>{{ $gettext('Sequential Write') }}</td>
                  <td>{{ formatSpeed(disk?.seq_write) }}</td>
                </tr>
                <tr>
                  <td>{{ $gettext('4K Random Read') }}</td>
                  <td>{{ formatIops(disk?.rand_read) }}</td>
                </tr>
                <tr>
                  <td>{{ $gettext('4K Random Write') }}</td>
                  <td>{{ formatIops(disk?.rand_write) }}</td>
                </tr>
              </tbody>
            </n-table>
          </n-flex>
        </n-card>
      </n-gi>
    </n-grid>

    <n-flex justify="center">
      <n-button
        type="primary"
        size="large"
        :disabled="running"
        :loading="running"
        @click="handleTest"
      >
        {{ running ? $gettext('Benchmarking...') : $gettext('Start Benchmark') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>

<script setup lang="ts">
defineOptions({
  name: 'toolbox-benchmark'
})

import benchmark from '@/api/panel/toolbox-benchmark'
import TheIcon from '@/components/custom/TheIcon.vue'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const inTest = ref(false)
const current = ref($gettext('CPU'))
const progress = ref(0)

const tests = [
  'image',
  'machine',
  'compile',
  'encryption',
  'compression',
  'physics',
  'json',
  'memory',
  'disk'
]

const cpu = ref({
  image: 0,
  machine: 0,
  compile: 0,
  encryption: 0,
  compression: 0,
  physics: 0,
  json: 0
})

const cpuTotal = computed(() => {
  return Object.values(cpu.value).reduce((a, b) => a + b, 0)
})

const memory = ref({
  score: 0,
  bandwidth: $gettext('Pending benchmark'),
  latency: $gettext('Pending benchmark')
})

const disk = ref({
  score: 0,
  1024: {
    read_speed: $gettext('Pending benchmark'),
    write_speed: $gettext('Pending benchmark')
  },
  4: {
    read_speed: $gettext('Pending benchmark'),
    write_speed: $gettext('Pending benchmark')
  },
  64: {
    read_speed: $gettext('Pending benchmark'),
    write_speed: $gettext('Pending benchmark')
  }
})

const handleTest = async () => {
  inTest.value = true
  progress.value = 0
  for (let i = 0; i < tests.length; i++) {
    const test = tests[i]
    current.value = test
    if (test != 'memory' && test != 'disk') {
      cpu.value[test as keyof typeof cpu.value] = await benchmark.test(test)
    } else {
      const data = await benchmark.test(test)
      if (test === 'memory') {
        memory.value = data
      } else {
        disk.value = data
      }
    }
    progress.value = Math.round(((i + 1) / tests.length) * 100)
  }
  inTest.value = false
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="warning">
      {{
        $gettext(
          'Benchmark results are for reference only and may differ from actual performance due to system resource scheduling, caching, and other factors!'
        )
      }}
    </n-alert>
    <n-alert
      v-if="inTest"
      :title="$gettext('Benchmarking in progress, it may take some time...')"
      type="info"
    >
      {{ $gettext('Current project: %{ current }', { current: current }) }}
    </n-alert>
    <n-progress v-if="inTest" :percentage="progress" processing />
  </n-flex>
  <n-flex    vertical pt-40 items-center >
    <div w-800>
      <n-grid :cols="3">
        <n-gi>
          <n-popover trigger="hover">
            <template #trigger>
              <n-flex vertical items-center>
                <div v-if="cpuTotal !== 0">
                  <n-number-animation :from="0" :to="cpuTotal" show-separator />
                </div>
                <div v-else>{{ $gettext('Pending benchmark') }}</div>
                <n-progress type="circle" :percentage="100" :stroke-width="3">
                  <the-icon :size="50" icon="mdi:cpu-64-bit" />
                </n-progress>
                {{ $gettext('CPU') }}
              </n-flex>
            </template>
            <n-table :single-line="false" striped>
              <tr>
                <th>{{ $gettext('Image Processing') }}</th>
                <td>
                  {{ cpu.image }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('Machine Learning') }}</th>
                <td>
                  {{ cpu.machine }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('Program Compilation') }}</th>
                <td>
                  {{ cpu.compile }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('AES Encryption') }}</th>
                <td>
                  {{ cpu.encryption }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('Compression/Decompression') }}</th>
                <td>
                  {{ cpu.compression }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('Physics Simulation') }}</th>
                <td>
                  {{ cpu.physics }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('JSON Parsing') }}</th>
                <td>
                  {{ cpu.json }}
                </td>
              </tr>
            </n-table>
          </n-popover>
        </n-gi>
        <n-gi>
          <n-popover trigger="hover">
            <template #trigger>
              <n-flex vertical items-center>
                <div v-if="memory.score !== 0">
                  <n-number-animation :from="0" :to="memory.score" show-separator />
                </div>
                <div v-else>{{ $gettext('Pending benchmark') }}</div>
                <n-progress type="circle" :percentage="100" :stroke-width="3">
                  <the-icon :size="50" icon="mdi:memory" />
                </n-progress>
                {{ $gettext('Memory') }}
              </n-flex>
            </template>
            <n-table :single-line="false" striped>
              <tr>
                <th>{{ $gettext('Memory Bandwidth') }}</th>
                <td>{{ memory.bandwidth }}</td>
              </tr>
              <tr>
                <th>{{ $gettext('Memory Latency') }}</th>
                <td>{{ memory.latency }}</td>
              </tr>
            </n-table>
          </n-popover>
        </n-gi>
        <n-gi>
          <n-popover trigger="hover">
            <template #trigger>
              <n-flex vertical items-center>
                <div v-if="disk.score !== 0">
                  <n-number-animation :from="0" :to="disk.score" show-separator />
                </div>
                <div v-else>{{ $gettext('Pending benchmark') }}</div>
                <n-progress type="circle" :percentage="100" :stroke-width="3">
                  <the-icon :size="50" icon="mdi:harddisk" />
                </n-progress>
                {{ $gettext('Disk') }}
              </n-flex>
            </template>
            <n-table :single-line="false" striped>
              <tr>
                <th>{{ $gettext('4KB Read') }}</th>
                <td>
                  {{ disk['4'].read_speed }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('4KB Write') }}</th>
                <td>
                  {{ disk['4'].write_speed }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('64KB Read') }}</th>
                <td>
                  {{ disk['64'].read_speed }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('64KB Write') }}</th>
                <td>
                  {{ disk['64'].write_speed }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('1MB Read') }}</th>
                <td>
                  {{ disk['1024'].read_speed }}
                </td>
              </tr>
              <tr>
                <th>{{ $gettext('1MB Write') }}</th>
                <td>
                  {{ disk['1024'].write_speed }}
                </td>
              </tr>
            </n-table>
          </n-popover>
        </n-gi>
      </n-grid>
    </div>
    <n-button
      type="primary"
      size="large"
      :disabled="inTest"
      :loading="inTest"
      @click="handleTest"
      mt-40
      w-200
    >
      {{ inTest ? $gettext('Benchmarking...') : $gettext('Start Benchmark') }}
    </n-button>
  </n-flex>
</template>

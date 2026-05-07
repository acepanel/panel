<script setup lang="ts">
defineOptions({
  name: 'toolbox-panel',
})

import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'

const { $gettext } = useGettext()
const currentTab = ref('runtime')

// 运行时信息
const runtimeData = ref<any>({})
const runtimeLoading = ref(true)

const loadRuntimeInfo = () => {
  runtimeLoading.value = true
  useRequest(home.runtimeInfo())
    .onSuccess(({ data }) => {
      runtimeData.value = data
    })
    .onComplete(() => {
      runtimeLoading.value = false
    })
}
loadRuntimeInfo()

// Goroutine 信息
const goroutines = ref<any[]>([])
const goroutineLoading = ref(false)

const loadGoroutines = () => {
  goroutineLoading.value = true
  useRequest(home.goroutines())
    .onSuccess(({ data }) => {
      goroutines.value = data || []
    })
    .onComplete(() => {
      goroutineLoading.value = false
    })
}

// 格式化字节
const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

// 格式化运行时间
const formatDuration = (seconds: number): string => {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  const parts: string[] = []
  if (d > 0) parts.push(`${d} ${$gettext('days')}`)
  if (h > 0) parts.push(`${h} ${$gettext('hours')}`)
  if (m > 0) parts.push(`${m} ${$gettext('min')}`)
  parts.push(`${s} ${$gettext('sec')}`)
  return parts.join(' ')
}

// 格式化纳秒
const formatNs = (ns: number): string => {
  if (ns < 1000) return `${ns} ns`
  if (ns < 1e6) return `${(ns / 1000).toFixed(2)} us`
  if (ns < 1e9) return `${(ns / 1e6).toFixed(2)} ms`
  return `${(ns / 1e9).toFixed(2)} s`
}

// 格式化纳秒时间戳为可读时间
const formatNsTimestamp = (ns: number): string => {
  if (!ns) return '-'
  return new Date(ns / 1e6).toLocaleString()
}

// 切换到 goroutine 时自动加载
watch(currentTab, (val) => {
  if (val === 'goroutines' && goroutines.value.length === 0) {
    loadGoroutines()
  }
})
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <n-tab-pane name="runtime" :tab="$gettext('Runtime')">
      <n-spin :show="runtimeLoading">
        <n-flex vertical :size="24">
          <!-- 基础信息 -->
          <n-descriptions :column="2" label-placement="left" bordered>
            <template #header>
              <n-flex justify="space-between" align="center">
                {{ $gettext('Basic Info') }}
                <n-button size="small" :loading="runtimeLoading" @click="loadRuntimeInfo">
                  {{ $gettext('Refresh') }}
                </n-button>
              </n-flex>
            </template>
            <n-descriptions-item :label="$gettext('Uptime')">
              {{ formatDuration(runtimeData.uptime || 0) }}
            </n-descriptions-item>
            <n-descriptions-item label="Go">
              {{ runtimeData.go_version }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Goroutines')">
              {{ runtimeData.goroutines }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('CPU Cores')">
              {{ runtimeData.num_cpu }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Cgo Calls')">
              {{ runtimeData.num_cgo_call }}
            </n-descriptions-item>
          </n-descriptions>

          <!-- 内存统计 -->
          <n-descriptions :title="$gettext('Memory')" :column="2" label-placement="left" bordered>
            <n-descriptions-item :label="$gettext('Allocated')">
              {{ formatBytes(runtimeData.memory_alloc || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Total Allocated')">
              {{ formatBytes(runtimeData.memory_total || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('System')">
              {{ formatBytes(runtimeData.memory_sys || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Lookups')">
              {{ runtimeData.memory_lookups }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Mallocs')">
              {{ runtimeData.memory_mallocs }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Frees')">
              {{ runtimeData.memory_frees }}
            </n-descriptions-item>
          </n-descriptions>

          <!-- Heap 统计 -->
          <n-descriptions title="Heap" :column="2" label-placement="left" bordered>
            <n-descriptions-item :label="$gettext('Allocated')">
              {{ formatBytes(runtimeData.heap_alloc || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('System')">
              {{ formatBytes(runtimeData.heap_sys || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Idle')">
              {{ formatBytes(runtimeData.heap_idle || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('In Use')">
              {{ formatBytes(runtimeData.heap_inuse || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Released')">
              {{ formatBytes(runtimeData.heap_released || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Objects')">
              {{ runtimeData.heap_objects }}
            </n-descriptions-item>
          </n-descriptions>

          <!-- Stack / MSpan / MCache -->
          <n-descriptions
            title="Stack / MSpan / MCache"
            :column="2"
            label-placement="left"
            bordered
          >
            <n-descriptions-item :label="$gettext('Stack In Use')">
              {{ formatBytes(runtimeData.stack_inuse || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Stack Sys')">
              {{ formatBytes(runtimeData.stack_sys || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('MSpan In Use')">
              {{ formatBytes(runtimeData.mspan_inuse || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('MSpan Sys')">
              {{ formatBytes(runtimeData.mspan_sys || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('MCache In Use')">
              {{ formatBytes(runtimeData.mcache_inuse || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('MCache Sys')">
              {{ formatBytes(runtimeData.mcache_sys || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('BuckHash Sys')">
              {{ formatBytes(runtimeData.buck_hash_sys || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Other Sys')">
              {{ formatBytes(runtimeData.other_sys || 0) }}
            </n-descriptions-item>
          </n-descriptions>

          <!-- GC 统计 -->
          <n-descriptions title="GC" :column="2" label-placement="left" bordered>
            <n-descriptions-item :label="$gettext('GC Runs')">
              {{ runtimeData.gc_num }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Forced GC Runs')">
              {{ runtimeData.gc_num_forced }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('GC Pause Total')">
              {{ formatNs(runtimeData.gc_pause_total || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Last GC')">
              {{ formatNsTimestamp(runtimeData.gc_last) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Next GC Target')">
              {{ formatBytes(runtimeData.gc_next || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('GC Sys')">
              {{ formatBytes(runtimeData.gc_sys || 0) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('GC CPU Fraction')">
              {{ ((runtimeData.gc_cpu_fraction || 0) * 100).toFixed(4) }}%
            </n-descriptions-item>
          </n-descriptions>
        </n-flex>
      </n-spin>
    </n-tab-pane>

    <n-tab-pane name="goroutines" tab="Goroutines">
      <n-flex vertical>
        <n-flex justify="space-between" align="center">
          <n-text>
            {{ $gettext('Total: %{ count }', { count: String(goroutines.length) }) }}
          </n-text>
          <n-button :loading="goroutineLoading" @click="loadGoroutines">
            {{ $gettext('Refresh') }}
          </n-button>
        </n-flex>

        <n-spin :show="goroutineLoading">
          <n-collapse>
            <n-collapse-item v-for="g in goroutines" :key="g.id" :name="g.id">
              <template #header>
                <n-flex :size="8" align="center">
                  <n-tag size="small" :type="g.state === 'running' ? 'success' : 'default'">
                    {{ g.state }}
                  </n-tag>
                  <n-text depth="3">goroutine {{ g.id }}</n-text>
                </n-flex>
              </template>
              <n-code :code="g.stack" language="go" word-wrap />
            </n-collapse-item>
          </n-collapse>
        </n-spin>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

<script setup lang="ts">
defineOptions({
  name: 'toolbox-log'
})

import { useGettext } from 'vue3-gettext'

import toolboxLog from '@/api/panel/toolbox-log'

const { $gettext } = useGettext()

// 日志类型
interface LogType {
  key: string
  name: string
  description: string
  icon: string
}

// 日志项
interface LogItem {
  name: string
  path: string
  size: string
}

// 扫描结果
interface ScanResult {
  loading: boolean
  items: LogItem[]
  scanned: boolean
  cleaning: boolean
}

const logTypes: LogType[] = [
  {
    key: 'panel',
    name: $gettext('Panel Logs'),
    description: $gettext('Panel runtime logs'),
    icon: 'mdi:view-dashboard-outline'
  },
  {
    key: 'website',
    name: $gettext('Website Logs'),
    description: $gettext('Website access and error logs'),
    icon: 'mdi:web'
  },
  {
    key: 'mysql',
    name: $gettext('MySQL Logs'),
    description: $gettext('MySQL slow query logs and binary logs'),
    icon: 'mdi:database'
  },
  {
    key: 'docker',
    name: $gettext('Docker'),
    description: $gettext('Docker container logs and unused images'),
    icon: 'mdi:docker'
  },
  {
    key: 'system',
    name: $gettext('System Logs'),
    description: $gettext('System logs and journal logs'),
    icon: 'mdi:server'
  }
]

// 扫描结果状态
const scanResults = ref<Record<string, ScanResult>>({
  panel: { loading: false, items: [], scanned: false, cleaning: false },
  website: { loading: false, items: [], scanned: false, cleaning: false },
  mysql: { loading: false, items: [], scanned: false, cleaning: false },
  docker: { loading: false, items: [], scanned: false, cleaning: false },
  system: { loading: false, items: [], scanned: false, cleaning: false }
})

// 扫描日志
const handleScan = async (type: string) => {
  scanResults.value[type].loading = true
  scanResults.value[type].scanned = false
  scanResults.value[type].items = []

  try {
    const { data } = await useRequest(toolboxLog.scan(type))
    scanResults.value[type].items = data || []
    scanResults.value[type].scanned = true
  } catch (e) {
    window.$message.error($gettext('Scan failed'))
  } finally {
    scanResults.value[type].loading = false
  }
}

// 清理日志
const handleClean = async (type: string) => {
  scanResults.value[type].cleaning = true

  try {
    const { data } = await useRequest(toolboxLog.clean(type))
    window.$message.success($gettext('Cleaned: %{ size }', { size: data.cleaned }))
    // 重新扫描
    await handleScan(type)
  } catch (e) {
    window.$message.error($gettext('Clean failed'))
  } finally {
    scanResults.value[type].cleaning = false
  }
}

// 扫描所有
const handleScanAll = async () => {
  for (const logType of logTypes) {
    await handleScan(logType.key)
  }
}

// 清理所有
const handleCleanAll = async () => {
  for (const logType of logTypes) {
    if (scanResults.value[logType.key].items.length > 0) {
      await handleClean(logType.key)
    }
  }
}

// 计算总数
const totalItems = computed(() => {
  return Object.values(scanResults.value).reduce((acc, cur) => acc + cur.items.length, 0)
})

// 计算是否有任何正在加载
const anyLoading = computed(() => {
  return Object.values(scanResults.value).some((r) => r.loading || r.cleaning)
})
</script>

<template>
  <n-flex vertical>
    <n-flex justify="end">
      <n-button type="primary" :loading="anyLoading" @click="handleScanAll">
        <template #icon>
          <i-mdi-magnify />
        </template>
        {{ $gettext('Scan All') }}
      </n-button>
      <n-button
        type="warning"
        :loading="anyLoading"
        :disabled="totalItems === 0"
        @click="handleCleanAll"
      >
        <template #icon>
          <i-mdi-delete-sweep />
        </template>
        {{ $gettext('Clean All') }}
      </n-button>
    </n-flex>

    <n-grid :cols="1" :x-gap="12" :y-gap="12">
      <n-grid-item v-for="logType in logTypes" :key="logType.key">
        <n-card :title="logType.name">
          <template #header-extra>
            <n-flex :size="8">
              <n-button
                size="small"
                :loading="scanResults[logType.key].loading"
                @click="handleScan(logType.key)"
              >
                <template #icon>
                  <i-mdi-magnify />
                </template>
                {{ $gettext('Scan') }}
              </n-button>
              <n-button
                size="small"
                type="warning"
                :loading="scanResults[logType.key].cleaning"
                :disabled="scanResults[logType.key].items.length === 0"
                @click="handleClean(logType.key)"
              >
                <template #icon>
                  <i-mdi-delete />
                </template>
                {{ $gettext('Clean') }}
              </n-button>
            </n-flex>
          </template>

          <n-flex vertical>
            <n-text depth="3">{{ logType.description }}</n-text>

            <template v-if="scanResults[logType.key].loading">
              <n-flex justify="center" align="center" style="min-height: 60px">
                <n-spin size="small" />
                <n-text>{{ $gettext('Scanning...') }}</n-text>
              </n-flex>
            </template>

            <template v-else-if="scanResults[logType.key].scanned">
              <template v-if="scanResults[logType.key].items.length === 0">
                <n-empty :description="$gettext('No logs found')" size="small" />
              </template>
              <template v-else>
                <n-data-table
                  :columns="[
                    { title: $gettext('Name'), key: 'name', ellipsis: { tooltip: true } },
                    { title: $gettext('Size'), key: 'size', width: 120 }
                  ]"
                  :data="scanResults[logType.key].items"
                  :bordered="false"
                  size="small"
                  :max-height="200"
                />
              </template>
            </template>

            <template v-else>
              <n-flex justify="center" align="center" style="min-height: 60px">
                <n-text depth="3">{{ $gettext('Click Scan to check logs') }}</n-text>
              </n-flex>
            </template>
          </n-flex>
        </n-card>
      </n-grid-item>
    </n-grid>
  </n-flex>
</template>

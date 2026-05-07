<script setup lang="ts">
defineOptions({
  name: 'toolbox-log',
})

import { useGettext } from 'vue3-gettext'

import toolboxLog from '@/api/panel/toolbox-log'
import TheIcon from '@/components/custom/TheIcon.vue'

const { $gettext } = useGettext()

interface LogType {
  key: string
  name: string
  description: string
  icon: string
}

interface LogItem {
  name: string
  path: string
  size: string
}

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
    icon: 'mdi:view-dashboard-outline',
  },
  {
    key: 'website',
    name: $gettext('Website Logs'),
    description: $gettext('Website access and error logs'),
    icon: 'mdi:web',
  },
  {
    key: 'mysql',
    name: $gettext('MySQL Logs'),
    description: $gettext('MySQL slow query logs and binary logs'),
    icon: 'mdi:database',
  },
  {
    key: 'docker',
    name: $gettext('Docker'),
    description: $gettext('Docker container logs and unused images'),
    icon: 'mdi:docker',
  },
  {
    key: 'system',
    name: $gettext('System Logs'),
    description: $gettext('System logs and journal logs'),
    icon: 'mdi:server',
  },
]

const scanResults = ref<Record<string, ScanResult>>({
  panel: { loading: false, items: [], scanned: false, cleaning: false },
  website: { loading: false, items: [], scanned: false, cleaning: false },
  mysql: { loading: false, items: [], scanned: false, cleaning: false },
  docker: { loading: false, items: [], scanned: false, cleaning: false },
  system: { loading: false, items: [], scanned: false, cleaning: false },
})

const handleScan = async (type: string) => {
  const result = scanResults.value[type]
  if (!result) return
  result.loading = true
  result.scanned = false
  result.items = []

  try {
    const { data } = await useRequest(toolboxLog.scan(type))
    result.items = data || []
    result.scanned = true
  } catch (e) {
    window.$message.error($gettext('Scan failed'))
  } finally {
    result.loading = false
  }
}

const handleClean = async (type: string) => {
  const result = scanResults.value[type]
  if (!result) return
  result.cleaning = true

  try {
    const { data } = await useRequest(toolboxLog.clean(type))
    window.$message.success($gettext('Cleaned: %{ size }', { size: data.cleaned }))
    await handleScan(type)
  } catch (e) {
    window.$message.error($gettext('Clean failed'))
  } finally {
    result.cleaning = false
  }
}

const handleScanAll = async () => {
  for (const logType of logTypes) {
    await handleScan(logType.key)
  }
}

const handleCleanAll = async () => {
  for (const logType of logTypes) {
    const result = scanResults.value[logType.key]
    if (result && result.items.length > 0) {
      await handleClean(logType.key)
    }
  }
}

const totalItems = computed(() => {
  return Object.values(scanResults.value).reduce((acc, cur) => acc + cur.items.length, 0)
})

const anyLoading = computed(() => {
  return Object.values(scanResults.value).some((r) => r.loading || r.cleaning)
})

const getResult = (key: string): ScanResult => {
  return scanResults.value[key] ?? { loading: false, items: [], scanned: false, cleaning: false }
}
</script>

<template>
  <n-flex vertical :size="16">
    <!-- 顶部操作栏 -->
    <div class="log-toolbar">
      <div class="log-toolbar__info">
        <div class="log-toolbar__title">
          {{ $gettext('Log Cleaner') }}
        </div>
        <div class="log-toolbar__desc">
          {{
            $gettext(
              'Scan and clean redundant logs to free up disk space. Cleaning is irreversible.',
            )
          }}
        </div>
      </div>
      <n-flex :size="8">
        <n-button
          type="primary"
          :loading="anyLoading"
          :disabled="anyLoading"
          @click="handleScanAll"
        >
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
    </div>

    <!-- 日志类型卡片 -->
    <n-grid :x-gap="16" :y-gap="16" cols="1 m:2" responsive="screen">
      <n-gi v-for="logType in logTypes" :key="logType.key">
        <div class="log-card">
          <div class="log-card__head">
            <div class="log-card__icon">
              <the-icon :icon="logType.icon" :size="22" />
            </div>
            <div class="log-card__info">
              <div class="log-card__name">{{ logType.name }}</div>
              <div class="log-card__desc">{{ logType.description }}</div>
            </div>
            <n-tag
              v-if="getResult(logType.key).scanned"
              :type="getResult(logType.key).items.length > 0 ? 'warning' : 'success'"
              size="small"
              :bordered="false"
            >
              {{ getResult(logType.key).items.length }}
            </n-tag>
          </div>

          <div class="log-card__body">
            <template v-if="getResult(logType.key).loading">
              <n-flex align="center" :size="8" class="text-text-tertiary">
                <n-spin size="small" />
                <span>{{ $gettext('Scanning...') }}</span>
              </n-flex>
            </template>
            <template v-else-if="getResult(logType.key).scanned">
              <template v-if="getResult(logType.key).items.length === 0">
                <span class="text-sm text-text-tertiary">{{ $gettext('No logs found') }}</span>
              </template>
              <template v-else>
                <n-collapse>
                  <n-collapse-item
                    :title="
                      $gettext('Found %{ count } items', {
                        count: getResult(logType.key).items.length.toString(),
                      })
                    "
                    name="1"
                  >
                    <n-data-table
                      :columns="[
                        { title: $gettext('Name'), key: 'name', ellipsis: { tooltip: true } },
                        { title: $gettext('Size'), key: 'size', width: 100 },
                      ]"
                      :data="getResult(logType.key).items"
                      :bordered="false"
                      size="small"
                      :max-height="220"
                    />
                  </n-collapse-item>
                </n-collapse>
              </template>
            </template>
            <template v-else>
              <span class="text-sm text-text-tertiary">
                {{ $gettext('Click Scan to check logs') }}
              </span>
            </template>
          </div>

          <div class="log-card__actions">
            <n-button
              size="small"
              :loading="getResult(logType.key).loading"
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
              :loading="getResult(logType.key).cleaning"
              :disabled="getResult(logType.key).items.length === 0"
              @click="handleClean(logType.key)"
            >
              <template #icon>
                <i-mdi-delete />
              </template>
              {{ $gettext('Clean') }}
            </n-button>
          </div>
        </div>
      </n-gi>
    </n-grid>
  </n-flex>
</template>

<style scoped lang="scss">
.log-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-default);
  border-radius: 3px;
}

.log-toolbar__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.log-toolbar__desc {
  margin-top: 4px;
  font-size: 13px;
  color: var(--color-text-tertiary);
}

.log-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-default);
  border-radius: 3px;
  height: 100%;
  transition: border-color 150ms ease;

  &:hover {
    border-color: var(--color-border-strong);
  }
}

.log-card__head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.log-card__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 3px;
  background: var(--color-brand-subtle);
  color: var(--color-brand);
  flex-shrink: 0;
}

.log-card__info {
  flex: 1;
  min-width: 0;
}

.log-card__name {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.log-card__desc {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.log-card__body {
  min-height: 36px;
  display: flex;
  align-items: center;
}

.log-card__actions {
  display: flex;
  gap: 8px;
  margin-top: auto;
}
</style>

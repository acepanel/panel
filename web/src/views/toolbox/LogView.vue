<script setup lang="ts">
defineOptions({
  name: 'toolbox-log',
})

import { useGettext } from 'vue3-gettext'

import toolboxLog from '@/api/panel/toolbox-log'

const { $gettext } = useGettext()

interface LogItem {
  name: string
  path: string
  size: string
}

interface LogType {
  key: string
  name: string
  description: string
  icon: string
  loading: boolean
  scanned: boolean
  cleaning: boolean
  items: LogItem[]
  checked: string[]
}

const logTypes = ref<LogType[]>(
  [
    {
      key: 'panel',
      name: $gettext('Panel Logs'),
      description: $gettext('Panel runtime logs'),
      icon: 'mdi:view-dashboard-outline',
    },
    {
      key: 'cron',
      name: $gettext('Scheduled Task Logs'),
      description: $gettext('Scheduled task execution logs'),
      icon: 'mdi:timetable',
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
  ].map((type) => ({
    ...type,
    loading: false,
    scanned: false,
    cleaning: false,
    items: [],
    checked: [],
  })),
)

const columns: any = [
  { type: 'selection' },
  { title: $gettext('Name'), key: 'name', ellipsis: { tooltip: true } },
  { title: $gettext('Size'), key: 'size', width: 100 },
]

const rowKey = (row: LogItem) => row.path

const handleScan = (logType: LogType) => {
  logType.loading = true
  logType.scanned = false
  logType.items = []
  logType.checked = []
  useRequest(toolboxLog.scan(logType.key))
    .onSuccess(({ data }) => {
      logType.items = data || []
      // 默认全选，不挑的话仍是一键清理
      logType.checked = logType.items.map((item) => item.path)
      logType.scanned = true
    })
    .onComplete(() => {
      logType.loading = false
    })
}

const handleClean = (logType: LogType) => {
  logType.cleaning = true
  useRequest(toolboxLog.clean(logType.key, logType.checked))
    .onSuccess(({ data }) => {
      window.$message.success($gettext('Cleaned: %{ size }', { size: data.cleaned }))
    })
    .onComplete(() => {
      logType.cleaning = false
      handleScan(logType)
    })
}

const handleScanAll = () => {
  logTypes.value.forEach(handleScan)
}

const handleCleanAll = () => {
  logTypes.value.filter((logType) => logType.checked.length > 0).forEach(handleClean)
}

const totalChecked = computed(() => {
  return logTypes.value.reduce((acc, cur) => acc + cur.checked.length, 0)
})

const anyLoading = computed(() => {
  return logTypes.value.some((logType) => logType.loading || logType.cleaning)
})
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
          :disabled="totalChecked === 0"
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
              v-if="logType.scanned"
              :type="logType.items.length > 0 ? 'warning' : 'success'"
              size="small"
              :bordered="false"
            >
              {{ logType.items.length }}
            </n-tag>
          </div>

          <div class="log-card__body">
            <template v-if="logType.loading">
              <n-flex align="center" :size="8" class="text-text-tertiary">
                <n-spin size="small" />
                <span>{{ $gettext('Scanning...') }}</span>
              </n-flex>
            </template>
            <template v-else-if="logType.scanned">
              <template v-if="logType.items.length === 0">
                <span class="text-sm text-text-tertiary">{{ $gettext('No logs found') }}</span>
              </template>
              <template v-else>
                <n-collapse>
                  <n-collapse-item
                    :title="
                      $gettext('Found %{ count } items', {
                        count: logType.items.length.toString(),
                      })
                    "
                    name="1"
                  >
                    <template #header-extra>
                      <span class="text-xs text-text-tertiary">
                        {{
                          $gettext('%{count} item(s) selected', {
                            count: logType.checked.length.toString(),
                          })
                        }}
                      </span>
                    </template>
                    <n-data-table
                      v-model:checked-row-keys="logType.checked"
                      :columns="columns"
                      :data="logType.items"
                      :row-key="rowKey"
                      :bordered="false"
                      size="small"
                      :max-height="220"
                      :scroll-x="340"
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
            <n-button size="small" :loading="logType.loading" @click="handleScan(logType)">
              <template #icon>
                <i-mdi-magnify />
              </template>
              {{ $gettext('Scan') }}
            </n-button>
            <n-button
              size="small"
              type="warning"
              :loading="logType.cleaning"
              :disabled="logType.checked.length === 0"
              @click="handleClean(logType)"
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

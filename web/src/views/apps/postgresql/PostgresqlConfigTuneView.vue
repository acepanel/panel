<script setup lang="ts">
defineOptions({
  name: 'postgresql-config-tune'
})

import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'

const { $gettext } = useGettext()
const currentTab = ref('connection')

// 连接设置
const listenAddresses = ref('')
const port = ref('')
const maxConnections = ref('')
const superuserReservedConnections = ref('')

// 内存设置
const sharedBuffers = ref('')
const workMem = ref('')
const maintenanceWorkMem = ref('')
const effectiveCacheSize = ref('')
const hugePages = ref('')

// WAL 设置
const walLevel = ref('')
const walBuffers = ref('')
const maxWalSize = ref('')
const minWalSize = ref('')
const checkpointCompletionTarget = ref('')

// 查询优化
const defaultStatisticsTarget = ref('')
const randomPageCost = ref('')
const effectiveIoConcurrency = ref('')

// 日志设置
const logDestination = ref('')
const logMinDurationStatement = ref('')
const logTimezone = ref('')

// IO 设置
const ioMethod = ref('')

const saveLoading = ref(false)

const walLevelOptions = [
  { label: 'minimal', value: 'minimal' },
  { label: 'replica', value: 'replica' },
  { label: 'logical', value: 'logical' }
]

const hugePagesOptions = [
  { label: 'off', value: 'off' },
  { label: 'on', value: 'on' },
  { label: 'try', value: 'try' }
]

const ioMethodOptions = [
  { label: 'sync', value: 'sync' },
  { label: 'worker', value: 'worker' },
  { label: 'io_uring', value: 'io_uring' }
]

useRequest(postgresql.configTune()).onSuccess(({ data }: any) => {
  listenAddresses.value = data.listen_addresses ?? ''
  port.value = data.port ?? ''
  maxConnections.value = data.max_connections ?? ''
  superuserReservedConnections.value = data.superuser_reserved_connections ?? ''
  sharedBuffers.value = data.shared_buffers ?? ''
  workMem.value = data.work_mem ?? ''
  maintenanceWorkMem.value = data.maintenance_work_mem ?? ''
  effectiveCacheSize.value = data.effective_cache_size ?? ''
  hugePages.value = data.huge_pages ?? ''
  walLevel.value = data.wal_level ?? ''
  walBuffers.value = data.wal_buffers ?? ''
  maxWalSize.value = data.max_wal_size ?? ''
  minWalSize.value = data.min_wal_size ?? ''
  checkpointCompletionTarget.value = data.checkpoint_completion_target ?? ''
  defaultStatisticsTarget.value = data.default_statistics_target ?? ''
  randomPageCost.value = data.random_page_cost ?? ''
  effectiveIoConcurrency.value = data.effective_io_concurrency ?? ''
  logDestination.value = data.log_destination ?? ''
  logMinDurationStatement.value = data.log_min_duration_statement ?? ''
  logTimezone.value = data.log_timezone ?? ''
  ioMethod.value = data.io_method ?? ''
})

const getConfigData = () => ({
  listen_addresses: listenAddresses.value,
  port: port.value,
  max_connections: maxConnections.value,
  superuser_reserved_connections: superuserReservedConnections.value,
  shared_buffers: sharedBuffers.value,
  work_mem: workMem.value,
  maintenance_work_mem: maintenanceWorkMem.value,
  effective_cache_size: effectiveCacheSize.value,
  huge_pages: hugePages.value,
  wal_level: walLevel.value,
  wal_buffers: walBuffers.value,
  max_wal_size: maxWalSize.value,
  min_wal_size: minWalSize.value,
  checkpoint_completion_target: checkpointCompletionTarget.value,
  default_statistics_target: defaultStatisticsTarget.value,
  random_page_cost: randomPageCost.value,
  effective_io_concurrency: effectiveIoConcurrency.value,
  log_destination: logDestination.value,
  log_min_duration_statement: logMinDurationStatement.value,
  log_timezone: logTimezone.value,
  io_method: ioMethod.value
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(postgresql.saveConfigTune(getConfigData()))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <n-tab-pane name="connection" :tab="$gettext('Connection')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('PostgreSQL connection and authentication settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Listen Addresses (listen_addresses)">
            <n-input
              v-model:value="listenAddresses"
              :placeholder="$gettext('e.g. localhost or *')"
            />
          </n-form-item>
          <n-form-item label="Port (port)">
            <n-input v-model:value="port" :placeholder="$gettext('e.g. 5432')" />
          </n-form-item>
          <n-form-item label="Max Connections (max_connections)">
            <n-input v-model:value="maxConnections" :placeholder="$gettext('e.g. 200')" />
          </n-form-item>
          <n-form-item label="Superuser Reserved Connections">
            <n-input
              v-model:value="superuserReservedConnections"
              :placeholder="$gettext('e.g. 3')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="memory" :tab="$gettext('Memory')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('PostgreSQL memory allocation settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Shared Buffers (shared_buffers)">
            <n-input v-model:value="sharedBuffers" :placeholder="$gettext('e.g. 256MB')" />
          </n-form-item>
          <n-form-item label="Work Mem (work_mem)">
            <n-input v-model:value="workMem" :placeholder="$gettext('e.g. 1260kB')" />
          </n-form-item>
          <n-form-item label="Maintenance Work Mem (maintenance_work_mem)">
            <n-input v-model:value="maintenanceWorkMem" :placeholder="$gettext('e.g. 64MB')" />
          </n-form-item>
          <n-form-item label="Effective Cache Size (effective_cache_size)">
            <n-input v-model:value="effectiveCacheSize" :placeholder="$gettext('e.g. 768MB')" />
          </n-form-item>
          <n-form-item label="Huge Pages (huge_pages)">
            <n-select v-model:value="hugePages" :options="hugePagesOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="wal" tab="WAL">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Write-Ahead Logging (WAL) settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="WAL Level (wal_level)">
            <n-select v-model:value="walLevel" :options="walLevelOptions" />
          </n-form-item>
          <n-form-item label="WAL Buffers (wal_buffers)">
            <n-input v-model:value="walBuffers" :placeholder="$gettext('e.g. 7864kB')" />
          </n-form-item>
          <n-form-item label="Max WAL Size (max_wal_size)">
            <n-input v-model:value="maxWalSize" :placeholder="$gettext('e.g. 4GB')" />
          </n-form-item>
          <n-form-item label="Min WAL Size (min_wal_size)">
            <n-input v-model:value="minWalSize" :placeholder="$gettext('e.g. 1GB')" />
          </n-form-item>
          <n-form-item label="Checkpoint Completion Target">
            <n-input
              v-model:value="checkpointCompletionTarget"
              :placeholder="$gettext('e.g. 0.9')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="query" :tab="$gettext('Query Optimization')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Query planner and optimization settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Default Statistics Target">
            <n-input
              v-model:value="defaultStatisticsTarget"
              :placeholder="$gettext('e.g. 100')"
            />
          </n-form-item>
          <n-form-item label="Random Page Cost (random_page_cost)">
            <n-input v-model:value="randomPageCost" :placeholder="$gettext('e.g. 1.1')" />
          </n-form-item>
          <n-form-item label="Effective IO Concurrency (effective_io_concurrency)">
            <n-input
              v-model:value="effectiveIoConcurrency"
              :placeholder="$gettext('e.g. 200')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="logging" :tab="$gettext('Logging')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('PostgreSQL logging settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Log Destination (log_destination)">
            <n-input v-model:value="logDestination" :placeholder="$gettext('e.g. stderr')" />
          </n-form-item>
          <n-form-item label="Log Min Duration Statement (log_min_duration_statement)">
            <n-input
              v-model:value="logMinDurationStatement"
              :placeholder="$gettext('e.g. -1 (disabled) or milliseconds')"
            />
          </n-form-item>
          <n-form-item label="Log Timezone (log_timezone)">
            <n-input v-model:value="logTimezone" :placeholder="$gettext('e.g. Asia/Shanghai')" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="io" tab="IO">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('IO method settings. Requires PostgreSQL restart to take effect.') }}
        </n-alert>
        <n-form>
          <n-form-item label="IO Method (io_method)">
            <n-select v-model:value="ioMethod" :options="ioMethodOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

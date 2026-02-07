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
const port = ref<number | null>(null)
const maxConnections = ref<number | null>(null)
const superuserReservedConnections = ref<number | null>(null)

// 内存设置
const sharedBuffersNum = ref<number | null>(null)
const sharedBuffersUnit = ref('MB')
const workMemNum = ref<number | null>(null)
const workMemUnit = ref('kB')
const maintenanceWorkMemNum = ref<number | null>(null)
const maintenanceWorkMemUnit = ref('MB')
const effectiveCacheSizeNum = ref<number | null>(null)
const effectiveCacheSizeUnit = ref('MB')
const hugePages = ref('')

// WAL 设置
const walLevel = ref('')
const walBuffersNum = ref<number | null>(null)
const walBuffersUnit = ref('kB')
const maxWalSizeNum = ref<number | null>(null)
const maxWalSizeUnit = ref('GB')
const minWalSizeNum = ref<number | null>(null)
const minWalSizeUnit = ref('GB')
const checkpointCompletionTarget = ref('')

// 查询优化
const defaultStatisticsTarget = ref<number | null>(null)
const randomPageCost = ref('')
const effectiveIoConcurrency = ref<number | null>(null)

// 日志设置
const logDestination = ref('')
const logMinDurationStatement = ref<number | null>(null)
const logTimezone = ref('')

// IO 设置
const ioMethod = ref('')

const saveLoading = ref(false)

// PostgreSQL 容量单位选项
const sizeUnitOptions = [
  { label: 'kB', value: 'kB' },
  { label: 'MB', value: 'MB' },
  { label: 'GB', value: 'GB' }
]

// 解析带单位的值，如 "256MB" -> { num: 256, unit: "MB" }
const parseSizeValue = (val: string): { num: number | null; unit: string } => {
  if (!val) return { num: null, unit: 'MB' }
  const match = val.match(/^(\d+)\s*(kB|MB|GB)$/i)
  if (match) {
    const unit = match[2]
    const normalized = unit === 'kb' ? 'kB' : unit === 'mb' ? 'MB' : unit === 'gb' ? 'GB' : unit
    return { num: Number(match[1]), unit: normalized }
  }
  return { num: Number(val) || null, unit: 'MB' }
}

// 组合数值和单位
const composeSizeValue = (num: number | null, unit: string): string => {
  if (num == null) return ''
  return `${num}${unit}`
}

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
  port.value = Number(data.port) || null
  maxConnections.value = Number(data.max_connections) || null
  superuserReservedConnections.value = Number(data.superuser_reserved_connections) || null
  const sb = parseSizeValue(data.shared_buffers ?? '')
  sharedBuffersNum.value = sb.num
  sharedBuffersUnit.value = sb.unit
  const wm = parseSizeValue(data.work_mem ?? '')
  workMemNum.value = wm.num
  workMemUnit.value = wm.unit
  const mwm = parseSizeValue(data.maintenance_work_mem ?? '')
  maintenanceWorkMemNum.value = mwm.num
  maintenanceWorkMemUnit.value = mwm.unit
  const ecs = parseSizeValue(data.effective_cache_size ?? '')
  effectiveCacheSizeNum.value = ecs.num
  effectiveCacheSizeUnit.value = ecs.unit
  hugePages.value = data.huge_pages ?? ''
  walLevel.value = data.wal_level ?? ''
  const wb = parseSizeValue(data.wal_buffers ?? '')
  walBuffersNum.value = wb.num
  walBuffersUnit.value = wb.unit
  const mxw = parseSizeValue(data.max_wal_size ?? '')
  maxWalSizeNum.value = mxw.num
  maxWalSizeUnit.value = mxw.unit
  const mnw = parseSizeValue(data.min_wal_size ?? '')
  minWalSizeNum.value = mnw.num
  minWalSizeUnit.value = mnw.unit
  checkpointCompletionTarget.value = data.checkpoint_completion_target ?? ''
  defaultStatisticsTarget.value = Number(data.default_statistics_target) || null
  randomPageCost.value = data.random_page_cost ?? ''
  effectiveIoConcurrency.value = Number(data.effective_io_concurrency) || null
  logDestination.value = data.log_destination ?? ''
  logMinDurationStatement.value = Number(data.log_min_duration_statement) ?? null
  logTimezone.value = data.log_timezone ?? ''
  ioMethod.value = data.io_method ?? ''
})

const getConfigData = () => ({
  listen_addresses: listenAddresses.value,
  port: String(port.value ?? ''),
  max_connections: String(maxConnections.value ?? ''),
  superuser_reserved_connections: String(superuserReservedConnections.value ?? ''),
  shared_buffers: composeSizeValue(sharedBuffersNum.value, sharedBuffersUnit.value),
  work_mem: composeSizeValue(workMemNum.value, workMemUnit.value),
  maintenance_work_mem: composeSizeValue(maintenanceWorkMemNum.value, maintenanceWorkMemUnit.value),
  effective_cache_size: composeSizeValue(effectiveCacheSizeNum.value, effectiveCacheSizeUnit.value),
  huge_pages: hugePages.value,
  wal_level: walLevel.value,
  wal_buffers: composeSizeValue(walBuffersNum.value, walBuffersUnit.value),
  max_wal_size: composeSizeValue(maxWalSizeNum.value, maxWalSizeUnit.value),
  min_wal_size: composeSizeValue(minWalSizeNum.value, minWalSizeUnit.value),
  checkpoint_completion_target: checkpointCompletionTarget.value,
  default_statistics_target: String(defaultStatisticsTarget.value ?? ''),
  random_page_cost: randomPageCost.value,
  effective_io_concurrency: String(effectiveIoConcurrency.value ?? ''),
  log_destination: logDestination.value,
  log_min_duration_statement: String(logMinDurationStatement.value ?? ''),
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
            <n-input-number
              class="w-full"
              v-model:value="port"
              :placeholder="$gettext('e.g. 5432')"
              :min="1"
              :max="65535"
            />
          </n-form-item>
          <n-form-item label="Max Connections (max_connections)">
            <n-input-number
              class="w-full"
              v-model:value="maxConnections"
              :placeholder="$gettext('e.g. 200')"
              :min="1"
            />
          </n-form-item>
          <n-form-item label="Superuser Reserved Connections (superuser_reserved_connections)">
            <n-input-number
              class="w-full"
              v-model:value="superuserReservedConnections"
              :placeholder="$gettext('e.g. 3')"
              :min="0"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
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
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="sharedBuffersNum"
                :placeholder="$gettext('e.g. 256')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="sharedBuffersUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Work Mem (work_mem)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="workMemNum"
                :placeholder="$gettext('e.g. 1260')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="workMemUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Maintenance Work Mem (maintenance_work_mem)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="maintenanceWorkMemNum"
                :placeholder="$gettext('e.g. 64')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="maintenanceWorkMemUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Effective Cache Size (effective_cache_size)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="effectiveCacheSizeNum"
                :placeholder="$gettext('e.g. 768')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="effectiveCacheSizeUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Huge Pages (huge_pages)">
            <n-select v-model:value="hugePages" :options="hugePagesOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
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
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="walBuffersNum"
                :placeholder="$gettext('e.g. 7864')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="walBuffersUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Max WAL Size (max_wal_size)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="maxWalSizeNum"
                :placeholder="$gettext('e.g. 4')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="maxWalSizeUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Min WAL Size (min_wal_size)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="minWalSizeNum"
                :placeholder="$gettext('e.g. 1')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="minWalSizeUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Checkpoint Completion Target (checkpoint_completion_target)">
            <n-input
              v-model:value="checkpointCompletionTarget"
              :placeholder="$gettext('e.g. 0.9')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
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
          <n-form-item label="Default Statistics Target (default_statistics_target)">
            <n-input-number
              class="w-full"
              v-model:value="defaultStatisticsTarget"
              :placeholder="$gettext('e.g. 100')"
              :min="1"
            />
          </n-form-item>
          <n-form-item label="Random Page Cost (random_page_cost)">
            <n-input v-model:value="randomPageCost" :placeholder="$gettext('e.g. 1.1')" />
          </n-form-item>
          <n-form-item label="Effective IO Concurrency (effective_io_concurrency)">
            <n-input-number
              class="w-full"
              v-model:value="effectiveIoConcurrency"
              :placeholder="$gettext('e.g. 200')"
              :min="0"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
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
            <n-input-number
              class="w-full"
              v-model:value="logMinDurationStatement"
              :placeholder="$gettext('e.g. -1 (disabled) or milliseconds')"
              :min="-1"
            />
          </n-form-item>
          <n-form-item label="Log Timezone (log_timezone)">
            <n-input v-model:value="logTimezone" :placeholder="$gettext('e.g. Asia/Shanghai')" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
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
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

<script setup lang="ts">
defineOptions({
  name: 'mysql-config-tune'
})

import { useGettext } from 'vue3-gettext'

const props = defineProps<{
  api: any
}>()

const { $gettext } = useGettext()
const currentTab = ref('general')

// 常规设置
const port = ref<number | null>(null)
const maxConnections = ref<number | null>(null)
const maxConnectErrors = ref<number | null>(null)
const defaultStorageEngine = ref('')
const tableOpenCache = ref<number | null>(null)
const maxAllowedPacketNum = ref<number | null>(null)
const maxAllowedPacketUnit = ref('M')
const openFilesLimit = ref<number | null>(null)

// 性能调整
const keyBufferSizeNum = ref<number | null>(null)
const keyBufferSizeUnit = ref('M')
const sortBufferSizeNum = ref<number | null>(null)
const sortBufferSizeUnit = ref('K')
const readBufferSizeNum = ref<number | null>(null)
const readBufferSizeUnit = ref('K')
const readRndBufferSizeNum = ref<number | null>(null)
const readRndBufferSizeUnit = ref('K')
const joinBufferSizeNum = ref<number | null>(null)
const joinBufferSizeUnit = ref('K')
const threadCacheSize = ref<number | null>(null)
const threadStackNum = ref<number | null>(null)
const threadStackUnit = ref('K')
const tmpTableSizeNum = ref<number | null>(null)
const tmpTableSizeUnit = ref('M')
const maxHeapTableSizeNum = ref<number | null>(null)
const maxHeapTableSizeUnit = ref('M')
const myisamSortBufferSizeNum = ref<number | null>(null)
const myisamSortBufferSizeUnit = ref('M')

// InnoDB
const innodbBufferPoolSizeNum = ref<number | null>(null)
const innodbBufferPoolSizeUnit = ref('M')
const innodbLogBufferSizeNum = ref<number | null>(null)
const innodbLogBufferSizeUnit = ref('M')
const innodbFlushLogAtTrxCommit = ref('')
const innodbLockWaitTimeout = ref<number | null>(null)
const innodbMaxDirtyPagesPct = ref<number | null>(null)
const innodbReadIoThreads = ref<number | null>(null)
const innodbWriteIoThreads = ref<number | null>(null)

// 日志
const slowQueryLog = ref('')
const longQueryTime = ref<number | null>(null)

const saveLoading = ref(false)

// 容量单位选项
const sizeUnitOptions = [
  { label: 'K', value: 'K' },
  { label: 'M', value: 'M' },
  { label: 'G', value: 'G' }
]

// 解析带单位的值，如 "50M" -> { num: 50, unit: "M" }
const parseSizeValue = (val: string): { num: number | null; unit: string } => {
  if (!val) return { num: null, unit: 'M' }
  const match = val.match(/^(\d+)\s*([KMG])$/i)
  if (match) {
    return { num: Number(match[1]), unit: match[2].toUpperCase() }
  }
  return { num: Number(val) || null, unit: 'M' }
}

// 组合数值和单位
const composeSizeValue = (num: number | null, unit: string): string => {
  if (num == null) return ''
  return `${num}${unit}`
}

const storageEngineOptions = [
  { label: 'InnoDB', value: 'InnoDB' },
  { label: 'MyISAM', value: 'MyISAM' },
  { label: 'MEMORY', value: 'MEMORY' },
  { label: 'CSV', value: 'CSV' },
  { label: 'RocksDB', value: 'ROCKSDB' }
]

const flushLogOptions = [
  { label: '0', value: '0' },
  { label: '1', value: '1' },
  { label: '2', value: '2' }
]

const slowQueryLogOptions = [
  { label: $gettext('On'), value: '1' },
  { label: $gettext('Off'), value: '0' }
]

useRequest(props.api.configTune()).onSuccess(({ data }: any) => {
  port.value = Number(data.port) || null
  maxConnections.value = Number(data.max_connections) || null
  maxConnectErrors.value = Number(data.max_connect_errors) || null
  defaultStorageEngine.value = data.default_storage_engine ?? ''
  tableOpenCache.value = Number(data.table_open_cache) || null
  const map = parseSizeValue(data.max_allowed_packet ?? '')
  maxAllowedPacketNum.value = map.num
  maxAllowedPacketUnit.value = map.unit
  openFilesLimit.value = Number(data.open_files_limit) || null
  const kbs = parseSizeValue(data.key_buffer_size ?? '')
  keyBufferSizeNum.value = kbs.num
  keyBufferSizeUnit.value = kbs.unit
  const sbs = parseSizeValue(data.sort_buffer_size ?? '')
  sortBufferSizeNum.value = sbs.num
  sortBufferSizeUnit.value = sbs.unit
  const rbs = parseSizeValue(data.read_buffer_size ?? '')
  readBufferSizeNum.value = rbs.num
  readBufferSizeUnit.value = rbs.unit
  const rrbs = parseSizeValue(data.read_rnd_buffer_size ?? '')
  readRndBufferSizeNum.value = rrbs.num
  readRndBufferSizeUnit.value = rrbs.unit
  const jbs = parseSizeValue(data.join_buffer_size ?? '')
  joinBufferSizeNum.value = jbs.num
  joinBufferSizeUnit.value = jbs.unit
  threadCacheSize.value = Number(data.thread_cache_size) || null
  const ts = parseSizeValue(data.thread_stack ?? '')
  threadStackNum.value = ts.num
  threadStackUnit.value = ts.unit
  const tts = parseSizeValue(data.tmp_table_size ?? '')
  tmpTableSizeNum.value = tts.num
  tmpTableSizeUnit.value = tts.unit
  const mhts = parseSizeValue(data.max_heap_table_size ?? '')
  maxHeapTableSizeNum.value = mhts.num
  maxHeapTableSizeUnit.value = mhts.unit
  const msbs = parseSizeValue(data.myisam_sort_buffer_size ?? '')
  myisamSortBufferSizeNum.value = msbs.num
  myisamSortBufferSizeUnit.value = msbs.unit
  const ibps = parseSizeValue(data.innodb_buffer_pool_size ?? '')
  innodbBufferPoolSizeNum.value = ibps.num
  innodbBufferPoolSizeUnit.value = ibps.unit
  const ilbs = parseSizeValue(data.innodb_log_buffer_size ?? '')
  innodbLogBufferSizeNum.value = ilbs.num
  innodbLogBufferSizeUnit.value = ilbs.unit
  innodbFlushLogAtTrxCommit.value = data.innodb_flush_log_at_trx_commit ?? ''
  innodbLockWaitTimeout.value = Number(data.innodb_lock_wait_timeout) || null
  innodbMaxDirtyPagesPct.value = Number(data.innodb_max_dirty_pages_pct) || null
  innodbReadIoThreads.value = Number(data.innodb_read_io_threads) || null
  innodbWriteIoThreads.value = Number(data.innodb_write_io_threads) || null
  slowQueryLog.value = data.slow_query_log ?? ''
  longQueryTime.value = Number(data.long_query_time) || null
})

const getConfigData = () => ({
  port: String(port.value ?? ''),
  max_connections: String(maxConnections.value ?? ''),
  max_connect_errors: String(maxConnectErrors.value ?? ''),
  default_storage_engine: defaultStorageEngine.value,
  table_open_cache: String(tableOpenCache.value ?? ''),
  max_allowed_packet: composeSizeValue(maxAllowedPacketNum.value, maxAllowedPacketUnit.value),
  open_files_limit: String(openFilesLimit.value ?? ''),
  key_buffer_size: composeSizeValue(keyBufferSizeNum.value, keyBufferSizeUnit.value),
  sort_buffer_size: composeSizeValue(sortBufferSizeNum.value, sortBufferSizeUnit.value),
  read_buffer_size: composeSizeValue(readBufferSizeNum.value, readBufferSizeUnit.value),
  read_rnd_buffer_size: composeSizeValue(readRndBufferSizeNum.value, readRndBufferSizeUnit.value),
  join_buffer_size: composeSizeValue(joinBufferSizeNum.value, joinBufferSizeUnit.value),
  thread_cache_size: String(threadCacheSize.value ?? ''),
  thread_stack: composeSizeValue(threadStackNum.value, threadStackUnit.value),
  tmp_table_size: composeSizeValue(tmpTableSizeNum.value, tmpTableSizeUnit.value),
  max_heap_table_size: composeSizeValue(maxHeapTableSizeNum.value, maxHeapTableSizeUnit.value),
  myisam_sort_buffer_size: composeSizeValue(myisamSortBufferSizeNum.value, myisamSortBufferSizeUnit.value),
  innodb_buffer_pool_size: composeSizeValue(innodbBufferPoolSizeNum.value, innodbBufferPoolSizeUnit.value),
  innodb_log_buffer_size: composeSizeValue(innodbLogBufferSizeNum.value, innodbLogBufferSizeUnit.value),
  innodb_flush_log_at_trx_commit: innodbFlushLogAtTrxCommit.value,
  innodb_lock_wait_timeout: String(innodbLockWaitTimeout.value ?? ''),
  innodb_max_dirty_pages_pct: String(innodbMaxDirtyPagesPct.value ?? ''),
  innodb_read_io_threads: String(innodbReadIoThreads.value ?? ''),
  innodb_write_io_threads: String(innodbWriteIoThreads.value ?? ''),
  slow_query_log: slowQueryLog.value,
  long_query_time: String(longQueryTime.value ?? '')
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(props.api.saveConfigTune(getConfigData()))
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
    <n-tab-pane name="general" :tab="$gettext('General')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Common MySQL general settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Port (port)">
            <n-input-number class="w-full" v-model:value="port" :placeholder="$gettext('e.g. 3306')" :min="1" :max="65535" />
          </n-form-item>
          <n-form-item label="Max Connections (max_connections)">
            <n-input-number class="w-full" v-model:value="maxConnections" :placeholder="$gettext('e.g. 50')" :min="1" />
          </n-form-item>
          <n-form-item label="Max Connect Errors (max_connect_errors)">
            <n-input-number class="w-full" v-model:value="maxConnectErrors" :placeholder="$gettext('e.g. 100')" :min="1" />
          </n-form-item>
          <n-form-item label="Default Storage Engine (default_storage_engine)">
            <n-select v-model:value="defaultStorageEngine" :options="storageEngineOptions" />
          </n-form-item>
          <n-form-item label="Table Open Cache (table_open_cache)">
            <n-input-number class="w-full" v-model:value="tableOpenCache" :placeholder="$gettext('e.g. 64')" :min="1" />
          </n-form-item>
          <n-form-item label="Max Allowed Packet (max_allowed_packet)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="maxAllowedPacketNum" :placeholder="$gettext('e.g. 1')" :min="0" style="flex: 1" />
              <n-select v-model:value="maxAllowedPacketUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Open Files Limit (open_files_limit)">
            <n-input-number class="w-full" v-model:value="openFilesLimit" :placeholder="$gettext('e.g. 65535')" :min="1" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="performance" :tab="$gettext('Performance Tuning')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('MySQL performance buffer and cache settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Key Buffer Size (key_buffer_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="keyBufferSizeNum" :placeholder="$gettext('e.g. 8')" :min="0" style="flex: 1" />
              <n-select v-model:value="keyBufferSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Sort Buffer Size (sort_buffer_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="sortBufferSizeNum" :placeholder="$gettext('e.g. 256')" :min="0" style="flex: 1" />
              <n-select v-model:value="sortBufferSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Read Buffer Size (read_buffer_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="readBufferSizeNum" :placeholder="$gettext('e.g. 256')" :min="0" style="flex: 1" />
              <n-select v-model:value="readBufferSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Read Rnd Buffer Size (read_rnd_buffer_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="readRndBufferSizeNum" :placeholder="$gettext('e.g. 256')" :min="0" style="flex: 1" />
              <n-select v-model:value="readRndBufferSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Join Buffer Size (join_buffer_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="joinBufferSizeNum" :placeholder="$gettext('e.g. 128')" :min="0" style="flex: 1" />
              <n-select v-model:value="joinBufferSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Thread Cache Size (thread_cache_size)">
            <n-input-number class="w-full" v-model:value="threadCacheSize" :placeholder="$gettext('e.g. 16')" :min="0" />
          </n-form-item>
          <n-form-item label="Thread Stack (thread_stack)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="threadStackNum" :placeholder="$gettext('e.g. 192')" :min="0" style="flex: 1" />
              <n-select v-model:value="threadStackUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Tmp Table Size (tmp_table_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="tmpTableSizeNum" :placeholder="$gettext('e.g. 16')" :min="0" style="flex: 1" />
              <n-select v-model:value="tmpTableSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Max Heap Table Size (max_heap_table_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="maxHeapTableSizeNum" :placeholder="$gettext('e.g. 16')" :min="0" style="flex: 1" />
              <n-select v-model:value="maxHeapTableSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="MyISAM Sort Buffer Size (myisam_sort_buffer_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="myisamSortBufferSizeNum" :placeholder="$gettext('e.g. 8')" :min="0" style="flex: 1" />
              <n-select v-model:value="myisamSortBufferSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="innodb" tab="InnoDB">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('InnoDB storage engine settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Buffer Pool Size (innodb_buffer_pool_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="innodbBufferPoolSizeNum" :placeholder="$gettext('e.g. 64')" :min="0" style="flex: 1" />
              <n-select v-model:value="innodbBufferPoolSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Log Buffer Size (innodb_log_buffer_size)">
            <n-input-group>
              <n-input-number class="w-full" v-model:value="innodbLogBufferSizeNum" :placeholder="$gettext('e.g. 16')" :min="0" style="flex: 1" />
              <n-select v-model:value="innodbLogBufferSizeUnit" :options="sizeUnitOptions" style="width: 80px" />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Flush Log At Trx Commit (innodb_flush_log_at_trx_commit)">
            <n-select v-model:value="innodbFlushLogAtTrxCommit" :options="flushLogOptions" />
          </n-form-item>
          <n-form-item label="Lock Wait Timeout (innodb_lock_wait_timeout)">
            <n-input-number class="w-full" v-model:value="innodbLockWaitTimeout" :placeholder="$gettext('e.g. 50')" :min="0" />
          </n-form-item>
          <n-form-item label="Max Dirty Pages Pct (innodb_max_dirty_pages_pct)">
            <n-input-number class="w-full" v-model:value="innodbMaxDirtyPagesPct" :placeholder="$gettext('e.g. 90')" :min="0" :max="100" />
          </n-form-item>
          <n-form-item label="Read IO Threads (innodb_read_io_threads)">
            <n-input-number class="w-full" v-model:value="innodbReadIoThreads" :placeholder="$gettext('e.g. 1')" :min="1" />
          </n-form-item>
          <n-form-item label="Write IO Threads (innodb_write_io_threads)">
            <n-input-number class="w-full" v-model:value="innodbWriteIoThreads" :placeholder="$gettext('e.g. 1')" :min="1" />
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
          {{ $gettext('MySQL logging settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Slow Query Log (slow_query_log)">
            <n-select v-model:value="slowQueryLog" :options="slowQueryLogOptions" />
          </n-form-item>
          <n-form-item label="Long Query Time (long_query_time)">
            <n-input-number
              class="w-full"
              v-model:value="longQueryTime"
              :placeholder="$gettext('e.g. 3 (seconds)')"
              :min="0"
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
  </n-tabs>
</template>

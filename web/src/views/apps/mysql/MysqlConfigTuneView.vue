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
const port = ref('')
const maxConnections = ref('')
const maxConnectErrors = ref('')
const defaultStorageEngine = ref('')
const tableOpenCache = ref('')
const maxAllowedPacket = ref('')
const openFilesLimit = ref('')

// 性能调整
const keyBufferSize = ref('')
const sortBufferSize = ref('')
const readBufferSize = ref('')
const readRndBufferSize = ref('')
const joinBufferSize = ref('')
const threadCacheSize = ref('')
const threadStack = ref('')
const tmpTableSize = ref('')
const maxHeapTableSize = ref('')
const myisamSortBufferSize = ref('')

// InnoDB
const innodbBufferPoolSize = ref('')
const innodbLogBufferSize = ref('')
const innodbFlushLogAtTrxCommit = ref('')
const innodbLockWaitTimeout = ref('')
const innodbMaxDirtyPagesPct = ref('')
const innodbReadIoThreads = ref('')
const innodbWriteIoThreads = ref('')

// 日志
const slowQueryLog = ref('')
const longQueryTime = ref('')

const saveLoading = ref(false)

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
  port.value = data.port ?? ''
  maxConnections.value = data.max_connections ?? ''
  maxConnectErrors.value = data.max_connect_errors ?? ''
  defaultStorageEngine.value = data.default_storage_engine ?? ''
  tableOpenCache.value = data.table_open_cache ?? ''
  maxAllowedPacket.value = data.max_allowed_packet ?? ''
  openFilesLimit.value = data.open_files_limit ?? ''
  keyBufferSize.value = data.key_buffer_size ?? ''
  sortBufferSize.value = data.sort_buffer_size ?? ''
  readBufferSize.value = data.read_buffer_size ?? ''
  readRndBufferSize.value = data.read_rnd_buffer_size ?? ''
  joinBufferSize.value = data.join_buffer_size ?? ''
  threadCacheSize.value = data.thread_cache_size ?? ''
  threadStack.value = data.thread_stack ?? ''
  tmpTableSize.value = data.tmp_table_size ?? ''
  maxHeapTableSize.value = data.max_heap_table_size ?? ''
  myisamSortBufferSize.value = data.myisam_sort_buffer_size ?? ''
  innodbBufferPoolSize.value = data.innodb_buffer_pool_size ?? ''
  innodbLogBufferSize.value = data.innodb_log_buffer_size ?? ''
  innodbFlushLogAtTrxCommit.value = data.innodb_flush_log_at_trx_commit ?? ''
  innodbLockWaitTimeout.value = data.innodb_lock_wait_timeout ?? ''
  innodbMaxDirtyPagesPct.value = data.innodb_max_dirty_pages_pct ?? ''
  innodbReadIoThreads.value = data.innodb_read_io_threads ?? ''
  innodbWriteIoThreads.value = data.innodb_write_io_threads ?? ''
  slowQueryLog.value = data.slow_query_log ?? ''
  longQueryTime.value = data.long_query_time ?? ''
})

const getConfigData = () => ({
  port: port.value,
  max_connections: maxConnections.value,
  max_connect_errors: maxConnectErrors.value,
  default_storage_engine: defaultStorageEngine.value,
  table_open_cache: tableOpenCache.value,
  max_allowed_packet: maxAllowedPacket.value,
  open_files_limit: openFilesLimit.value,
  key_buffer_size: keyBufferSize.value,
  sort_buffer_size: sortBufferSize.value,
  read_buffer_size: readBufferSize.value,
  read_rnd_buffer_size: readRndBufferSize.value,
  join_buffer_size: joinBufferSize.value,
  thread_cache_size: threadCacheSize.value,
  thread_stack: threadStack.value,
  tmp_table_size: tmpTableSize.value,
  max_heap_table_size: maxHeapTableSize.value,
  myisam_sort_buffer_size: myisamSortBufferSize.value,
  innodb_buffer_pool_size: innodbBufferPoolSize.value,
  innodb_log_buffer_size: innodbLogBufferSize.value,
  innodb_flush_log_at_trx_commit: innodbFlushLogAtTrxCommit.value,
  innodb_lock_wait_timeout: innodbLockWaitTimeout.value,
  innodb_max_dirty_pages_pct: innodbMaxDirtyPagesPct.value,
  innodb_read_io_threads: innodbReadIoThreads.value,
  innodb_write_io_threads: innodbWriteIoThreads.value,
  slow_query_log: slowQueryLog.value,
  long_query_time: longQueryTime.value
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
            <n-input v-model:value="port" :placeholder="$gettext('e.g. 3306')" />
          </n-form-item>
          <n-form-item label="Max Connections (max_connections)">
            <n-input v-model:value="maxConnections" :placeholder="$gettext('e.g. 50')" />
          </n-form-item>
          <n-form-item label="Max Connect Errors (max_connect_errors)">
            <n-input v-model:value="maxConnectErrors" :placeholder="$gettext('e.g. 100')" />
          </n-form-item>
          <n-form-item label="Default Storage Engine">
            <n-input v-model:value="defaultStorageEngine" :placeholder="$gettext('e.g. InnoDB')" />
          </n-form-item>
          <n-form-item label="Table Open Cache (table_open_cache)">
            <n-input v-model:value="tableOpenCache" :placeholder="$gettext('e.g. 64')" />
          </n-form-item>
          <n-form-item label="Max Allowed Packet (max_allowed_packet)">
            <n-input v-model:value="maxAllowedPacket" :placeholder="$gettext('e.g. 1G')" />
          </n-form-item>
          <n-form-item label="Open Files Limit (open_files_limit)">
            <n-input v-model:value="openFilesLimit" :placeholder="$gettext('e.g. 65535')" />
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
            <n-input v-model:value="keyBufferSize" :placeholder="$gettext('e.g. 8M')" />
          </n-form-item>
          <n-form-item label="Sort Buffer Size (sort_buffer_size)">
            <n-input v-model:value="sortBufferSize" :placeholder="$gettext('e.g. 256K')" />
          </n-form-item>
          <n-form-item label="Read Buffer Size (read_buffer_size)">
            <n-input v-model:value="readBufferSize" :placeholder="$gettext('e.g. 256K')" />
          </n-form-item>
          <n-form-item label="Read Rnd Buffer Size (read_rnd_buffer_size)">
            <n-input v-model:value="readRndBufferSize" :placeholder="$gettext('e.g. 256K')" />
          </n-form-item>
          <n-form-item label="Join Buffer Size (join_buffer_size)">
            <n-input v-model:value="joinBufferSize" :placeholder="$gettext('e.g. 128K')" />
          </n-form-item>
          <n-form-item label="Thread Cache Size (thread_cache_size)">
            <n-input v-model:value="threadCacheSize" :placeholder="$gettext('e.g. 16')" />
          </n-form-item>
          <n-form-item label="Thread Stack (thread_stack)">
            <n-input v-model:value="threadStack" :placeholder="$gettext('e.g. 192K')" />
          </n-form-item>
          <n-form-item label="Tmp Table Size (tmp_table_size)">
            <n-input v-model:value="tmpTableSize" :placeholder="$gettext('e.g. 16M')" />
          </n-form-item>
          <n-form-item label="Max Heap Table Size (max_heap_table_size)">
            <n-input v-model:value="maxHeapTableSize" :placeholder="$gettext('e.g. 16M')" />
          </n-form-item>
          <n-form-item label="MyISAM Sort Buffer Size (myisam_sort_buffer_size)">
            <n-input v-model:value="myisamSortBufferSize" :placeholder="$gettext('e.g. 8M')" />
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
            <n-input v-model:value="innodbBufferPoolSize" :placeholder="$gettext('e.g. 64M')" />
          </n-form-item>
          <n-form-item label="Log Buffer Size (innodb_log_buffer_size)">
            <n-input v-model:value="innodbLogBufferSize" :placeholder="$gettext('e.g. 16M')" />
          </n-form-item>
          <n-form-item label="Flush Log At Trx Commit (innodb_flush_log_at_trx_commit)">
            <n-select v-model:value="innodbFlushLogAtTrxCommit" :options="flushLogOptions" />
          </n-form-item>
          <n-form-item label="Lock Wait Timeout (innodb_lock_wait_timeout)">
            <n-input v-model:value="innodbLockWaitTimeout" :placeholder="$gettext('e.g. 50')" />
          </n-form-item>
          <n-form-item label="Max Dirty Pages Pct (innodb_max_dirty_pages_pct)">
            <n-input v-model:value="innodbMaxDirtyPagesPct" :placeholder="$gettext('e.g. 90')" />
          </n-form-item>
          <n-form-item label="Read IO Threads (innodb_read_io_threads)">
            <n-input v-model:value="innodbReadIoThreads" :placeholder="$gettext('e.g. 1')" />
          </n-form-item>
          <n-form-item label="Write IO Threads (innodb_write_io_threads)">
            <n-input v-model:value="innodbWriteIoThreads" :placeholder="$gettext('e.g. 1')" />
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
            <n-input
              v-model:value="longQueryTime"
              :placeholder="$gettext('e.g. 3 (seconds)')"
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

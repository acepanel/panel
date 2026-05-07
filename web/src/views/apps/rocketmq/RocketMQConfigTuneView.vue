<script setup lang="ts">
defineOptions({
  name: 'rocketmq-config-tune',
})

import { useGettext } from 'vue3-gettext'

import rocketmq from '@/api/apps/rocketmq'

const { $gettext } = useGettext()
const currentTab = ref('broker')

const brokerName = ref('')
const listenPort = ref('')
const namesrvAddr = ref('')
const brokerRole = ref('')
const flushDiskType = ref('')
const storePathRootDir = ref('')
const storePathCommitLog = ref('')
const maxMessageSize = ref('')
const namesrvHeapInitSize = ref('')
const namesrvHeapMaxSize = ref('')
const brokerHeapInitSize = ref('')
const brokerHeapMaxSize = ref('')

const saveLoading = ref(false)

const brokerRoleOptions = [
  { label: 'ASYNC_MASTER', value: 'ASYNC_MASTER' },
  { label: 'SYNC_MASTER', value: 'SYNC_MASTER' },
  { label: 'SLAVE', value: 'SLAVE' },
]

const flushDiskTypeOptions = [
  { label: 'ASYNC_FLUSH', value: 'ASYNC_FLUSH' },
  { label: 'SYNC_FLUSH', value: 'SYNC_FLUSH' },
]

useRequest(rocketmq.configTune()).onSuccess(({ data }: any) => {
  brokerName.value = data.broker_name ?? ''
  listenPort.value = data.listen_port ?? ''
  namesrvAddr.value = data.namesrv_addr ?? ''
  brokerRole.value = data.broker_role ?? ''
  flushDiskType.value = data.flush_disk_type ?? ''
  storePathRootDir.value = data.store_path_root_dir ?? ''
  storePathCommitLog.value = data.store_path_commit_log ?? ''
  maxMessageSize.value = data.max_message_size ?? ''
  namesrvHeapInitSize.value = data.namesrv_heap_init_size ?? ''
  namesrvHeapMaxSize.value = data.namesrv_heap_max_size ?? ''
  brokerHeapInitSize.value = data.broker_heap_init_size ?? ''
  brokerHeapMaxSize.value = data.broker_heap_max_size ?? ''
})

const getConfigData = () => ({
  broker_name: brokerName.value,
  listen_port: listenPort.value,
  namesrv_addr: namesrvAddr.value,
  broker_role: brokerRole.value,
  flush_disk_type: flushDiskType.value,
  store_path_root_dir: storePathRootDir.value,
  store_path_commit_log: storePathCommitLog.value,
  max_message_size: maxMessageSize.value,
  namesrv_heap_init_size: namesrvHeapInitSize.value,
  namesrv_heap_max_size: namesrvHeapMaxSize.value,
  broker_heap_init_size: brokerHeapInitSize.value,
  broker_heap_max_size: brokerHeapMaxSize.value,
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(rocketmq.saveConfigTune(getConfigData()))
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
    <n-tab-pane name="broker" :tab="$gettext('Broker')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('RocketMQ broker basic settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Broker Name (brokerName)')">
            <n-input v-model:value="brokerName" placeholder="broker-a" />
          </n-form-item>
          <n-form-item :label="$gettext('Listen Port (listenPort)')">
            <n-input v-model:value="listenPort" placeholder="10911" />
          </n-form-item>
          <n-form-item :label="$gettext('NameServer Address (namesrvAddr)')">
            <n-input v-model:value="namesrvAddr" placeholder="127.0.0.1:9876" />
          </n-form-item>
          <n-form-item :label="$gettext('Broker Role (brokerRole)')">
            <n-select v-model:value="brokerRole" :options="brokerRoleOptions" clearable />
          </n-form-item>
          <n-form-item :label="$gettext('Flush Disk Type (flushDiskType)')">
            <n-select v-model:value="flushDiskType" :options="flushDiskTypeOptions" clearable />
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
    <n-tab-pane name="storage" :tab="$gettext('Storage')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('RocketMQ storage path and message size settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Store Root Dir (storePathRootDir)')">
            <n-input v-model:value="storePathRootDir" placeholder="/data/rocketmq/store" />
          </n-form-item>
          <n-form-item :label="$gettext('CommitLog Path (storePathCommitLog)')">
            <n-input
              v-model:value="storePathCommitLog"
              placeholder="/data/rocketmq/store/commitlog"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Max Message Size (maxMessageSize)')">
            <n-input v-model:value="maxMessageSize" placeholder="4194304" />
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
    <n-tab-pane name="jvm" :tab="$gettext('JVM')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('JVM heap memory settings for NameServer and Broker.') }}
        </n-alert>
        <n-form>
          <n-h3 prefix="bar">NameServer</n-h3>
          <n-form-item :label="$gettext('Initial Heap Size (-Xms)')">
            <n-input v-model:value="namesrvHeapInitSize" placeholder="512m" />
          </n-form-item>
          <n-form-item :label="$gettext('Max Heap Size (-Xmx)')">
            <n-input v-model:value="namesrvHeapMaxSize" placeholder="512m" />
          </n-form-item>
          <n-h3 prefix="bar">Broker</n-h3>
          <n-form-item :label="$gettext('Initial Heap Size (-Xms)')">
            <n-input v-model:value="brokerHeapInitSize" placeholder="1g" />
          </n-form-item>
          <n-form-item :label="$gettext('Max Heap Size (-Xmx)')">
            <n-input v-model:value="brokerHeapMaxSize" placeholder="1g" />
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

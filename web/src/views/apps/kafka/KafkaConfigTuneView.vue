<script setup lang="ts">
defineOptions({
  name: 'kafka-config-tune',
})

import { useGettext } from 'vue3-gettext'

import kafka from '@/api/apps/kafka'

const { $gettext } = useGettext()
const currentTab = ref('broker')

const nodeId = ref('')
const listeners = ref('')
const logDirs = ref('')
const numPartitions = ref('')
const retentionHours = ref('')
const logSegmentBytes = ref('')
const heapInitSize = ref('')
const heapMaxSize = ref('')

const saveLoading = ref(false)

useRequest(kafka.configTune()).onSuccess(({ data }: any) => {
  nodeId.value = data.node_id ?? ''
  listeners.value = data.listeners ?? ''
  logDirs.value = data.log_dirs ?? ''
  numPartitions.value = data.num_partitions ?? ''
  retentionHours.value = data.retention_hours ?? ''
  logSegmentBytes.value = data.log_segment_bytes ?? ''
  heapInitSize.value = data.heap_init_size ?? ''
  heapMaxSize.value = data.heap_max_size ?? ''
})

const getConfigData = () => ({
  node_id: nodeId.value,
  listeners: listeners.value,
  log_dirs: logDirs.value,
  num_partitions: numPartitions.value,
  retention_hours: retentionHours.value,
  log_segment_bytes: logSegmentBytes.value,
  heap_init_size: heapInitSize.value,
  heap_max_size: heapMaxSize.value,
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(kafka.saveConfigTune(getConfigData()))
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
          {{ $gettext('Kafka broker and network settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Node ID (node.id)')">
            <n-input v-model:value="nodeId" placeholder="1" />
          </n-form-item>
          <n-form-item :label="$gettext('Listeners (listeners)')">
            <n-input v-model:value="listeners" placeholder="PLAINTEXT://:9092" />
          </n-form-item>
          <n-form-item :label="$gettext('Log Dirs (log.dirs)')">
            <n-input v-model:value="logDirs" placeholder="/data/kafka-logs" />
          </n-form-item>
          <n-form-item :label="$gettext('Num Partitions (num.partitions)')">
            <n-input v-model:value="numPartitions" placeholder="1" />
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
    <n-tab-pane name="storage-jvm" :tab="$gettext('Storage & JVM')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Log retention and JVM heap memory settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Log Retention Hours (log.retention.hours)')">
            <n-input v-model:value="retentionHours" placeholder="168" />
          </n-form-item>
          <n-form-item :label="$gettext('Log Segment Bytes (log.segment.bytes)')">
            <n-input v-model:value="logSegmentBytes" placeholder="1073741824" />
          </n-form-item>
          <n-form-item :label="$gettext('Initial Heap Size (-Xms)')">
            <n-input v-model:value="heapInitSize" placeholder="1g" />
          </n-form-item>
          <n-form-item :label="$gettext('Max Heap Size (-Xmx)')">
            <n-input v-model:value="heapMaxSize" placeholder="1g" />
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

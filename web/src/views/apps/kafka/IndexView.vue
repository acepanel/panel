<script setup lang="ts">
defineOptions({
  name: 'apps-kafka-index',
})

import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import kafka from '@/api/apps/kafka'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

import KafkaConfigTuneView from './KafkaConfigTuneView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveConfigLoading = ref(false)

const { data: config, send: refreshConfig } = useRequest(kafka.config, {
  initialData: '',
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  }
})
const { data: load } = useRequest(kafka.load, {
  initialData: [],
})

const loadColumns: any = [
  {
    title: $gettext('Property'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Current Value'),
    key: 'value',
    minWidth: 200,
    ellipsis: { tooltip: true },
  },
]

const handleSaveConfig = () => {
  saveConfigLoading.value = true
  useRequest(kafka.saveConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="kafka" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the Kafka main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button
              type="primary"
              :loading="saveConfigLoading"
              :disabled="saveConfigLoading"
              @click="handleSaveConfig"
            >
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <kafka-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <n-data-table
          striped
          remote
          :scroll-x="1000"
          :loading="false"
          :columns="loadColumns"
          :data="load"
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="kafka" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

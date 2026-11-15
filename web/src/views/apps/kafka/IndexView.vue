<script setup lang="ts">
defineOptions({
  name: 'apps-kafka-index',
})

import kafka from '@/api/apps/kafka'

import KafkaConfigTuneView from './KafkaConfigTuneView.vue'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="kafka" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="kafka.config"
          :save="kafka.saveConfig"
          :tip="
            $gettext(
              'This modifies the Kafka main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <kafka-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="kafka.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="kafka" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

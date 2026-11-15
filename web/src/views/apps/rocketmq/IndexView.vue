<script setup lang="ts">
defineOptions({
  name: 'apps-rocketmq-index',
})

import rocketmq from '@/api/apps/rocketmq'

import RocketMQConfigTuneView from './RocketMQConfigTuneView.vue'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical :size="16">
          <service-status service="rocketmq-namesrv" title="NameServer" />
          <service-status service="rocketmq-broker" title="Broker" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="rocketmq.config"
          :save="rocketmq.saveConfig"
          :tip="
            $gettext(
              'This modifies the RocketMQ broker configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <rocket-m-q-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="rocketmq.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="rocketmq-broker" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

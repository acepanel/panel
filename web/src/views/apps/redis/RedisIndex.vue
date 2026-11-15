<script setup lang="ts">
import type redis from '@/api/apps/redis'

import RedisConfigTuneView from './RedisConfigTuneView.vue'
import RedisPerformanceView from './RedisPerformanceView.vue'

defineProps<{
  api: typeof redis
  name: 'Redis' | 'Valkey'
  service: 'redis' | 'valkey'
}>()

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status :service="service" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="api.config"
          :save="api.saveConfig"
          :tip="
            $gettext(
              'This modifies the %{ name } main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              { name },
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <redis-config-tune-view :api="api" :name="name" />
      </n-tab-pane>
      <n-tab-pane name="performance" :tab="$gettext('Performance')">
        <redis-performance-view :api="api" />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="api.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log :service="service" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

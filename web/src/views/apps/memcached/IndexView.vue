<script setup lang="ts">
defineOptions({
  name: 'apps-memcached-index',
})

import memcached from '@/api/apps/memcached'

import MemcachedConfigTuneView from './MemcachedConfigTuneView.vue'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="memcached" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Service Configuration')">
        <config-editor-pane :fetch="memcached.config" :save="memcached.updateConfig" />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <memcached-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="memcached.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="memcached" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

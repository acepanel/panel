<script setup lang="ts">
defineOptions({
  name: 'apps-memcached-index'
})

import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import memcached from '@/api/apps/memcached'
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import MemcachedConfigTuneView from './MemcachedConfigTuneView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')

const loadColumns: any = [
  {
    title: $gettext('Property'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Current Value'),
    key: 'value',
    minWidth: 200,
    ellipsis: { tooltip: true }
  }
]

const { data: load } = useRequest(memcached.load, {
  initialData: []
})

const { data: config, send: refreshConfig } = useRequest(memcached.config, {
  initialData: {
    config: ''
  }
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  }
})

const handleSaveConfig = () => {
  useRequest(memcached.updateConfig(config.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="memcached" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Service Configuration')">
        <n-flex vertical>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button type="primary" @click="handleSaveConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <memcached-config-tune-view />
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
        <realtime-log service="memcached" />
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>

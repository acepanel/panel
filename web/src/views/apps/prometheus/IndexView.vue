<script setup lang="ts">
defineOptions({
  name: 'apps-prometheus-index'
})

import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import prometheus from '@/api/apps/prometheus'
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import PrometheusConfigTuneView from './PrometheusConfigTuneView.vue'
import PrometheusExportersView from './PrometheusExportersView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveConfigLoading = ref(false)

const { data: config, send: refreshConfig } = useRequest(prometheus.config, {
  initialData: ''
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  }
})
const { data: load } = useRequest(prometheus.load, {
  initialData: []
})

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

const handleSaveConfig = () => {
  saveConfigLoading.value = true
  useRequest(prometheus.saveConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="prometheus" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the Prometheus main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!'
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button type="primary" :loading="saveConfigLoading" :disabled="saveConfigLoading" @click="handleSaveConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <prometheus-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="exporters" :tab="$gettext('Exporters')">
        <prometheus-exporters-view />
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
        <realtime-log service="prometheus" />
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>

<script setup lang="ts">
defineOptions({
  name: 'apps-prometheus-index',
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
const saveAlertmanagerLoading = ref(false)

const { data: config, send: refreshConfig } = useRequest(prometheus.config, {
  initialData: '',
})
const { data: alertmanagerConfig, send: refreshAlertmanagerConfig } = useRequest(
  prometheus.alertmanagerConfig,
  {
    initialData: '',
  },
)

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  }
  if (val === 'alertmanager') {
    refreshAlertmanagerConfig()
  }
})
const { data: load } = useRequest(prometheus.load, {
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
  useRequest(prometheus.saveConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

const handleSaveAlertmanagerConfig = () => {
  saveAlertmanagerLoading.value = true
  useRequest(prometheus.saveAlertmanagerConfig(alertmanagerConfig.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveAlertmanagerLoading.value = false
    })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="prometheus" />
          <n-alert type="info">
            {{
              $gettext(
                'By default, the firewall does not allow access to the Prometheus port (9090). If you need external access, please manually open the port in the firewall settings.',
              )
            }}
          </n-alert>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the Prometheus main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
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
        <prometheus-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="alertmanager" :tab="$gettext('Alertmanager')">
        <n-flex vertical>
          <service-status service="alertmanager" />
          <n-alert type="info">
            {{
              $gettext(
                'Configure Alertmanager notification channels (email, webhook, Slack, etc). To enable alert rules, uncomment the rule_files section in the main configuration and create rule files in the rules directory.',
              )
            }}
          </n-alert>
          <common-editor v-model:value="alertmanagerConfig" height="50vh" />
          <n-flex>
            <n-button
              type="primary"
              :loading="saveAlertmanagerLoading"
              :disabled="saveAlertmanagerLoading"
              @click="handleSaveAlertmanagerConfig"
            >
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
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
  </PageContainer>
</template>

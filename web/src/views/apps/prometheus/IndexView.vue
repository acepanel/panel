<script setup lang="ts">
defineOptions({
  name: 'apps-prometheus-index',
})

import prometheus from '@/api/apps/prometheus'

import PrometheusConfigTuneView from './PrometheusConfigTuneView.vue'
import PrometheusExportersView from './PrometheusExportersView.vue'

const currentTab = ref('status')
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
        <config-editor-pane
          :fetch="prometheus.config"
          :save="prometheus.saveConfig"
          :tip="
            $gettext(
              'This modifies the Prometheus main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
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
          <config-editor-pane
            :fetch="prometheus.alertmanagerConfig"
            :save="prometheus.saveAlertmanagerConfig"
            height="50vh"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="exporters" :tab="$gettext('Exporters')">
        <prometheus-exporters-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="prometheus.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="prometheus" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

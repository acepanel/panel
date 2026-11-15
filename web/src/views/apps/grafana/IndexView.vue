<script setup lang="ts">
defineOptions({
  name: 'apps-grafana-index',
})

import grafana from '@/api/apps/grafana'

import GrafanaConfigTuneView from './GrafanaConfigTuneView.vue'
import GrafanaDataSourcesView from './GrafanaDataSourcesView.vue'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="grafana" />
          <n-alert :title="$gettext('Recommended Dashboards')" type="info">
            <n-flex vertical :size="2">
              <span>{{ $gettext('Import dashboards in Grafana via Dashboard ID:') }}</span>
              <n-ul>
                <n-li>Node Exporter — <n-text code>1860</n-text></n-li>
                <n-li>Nginx — <n-text code>12708</n-text></n-li>
                <n-li>MySQL — <n-text code>14057</n-text></n-li>
                <n-li>PostgreSQL — <n-text code>9628</n-text></n-li>
                <n-li>MongoDB — <n-text code>20867</n-text></n-li>
                <n-li>Elasticsearch — <n-text code>14191</n-text></n-li>
                <n-li>Redis — <n-text code>11835</n-text> / <n-text code>14091</n-text></n-li>
                <n-li>Memcached — <n-text code>37</n-text></n-li>
                <n-li>Kafka — <n-text code>15465</n-text></n-li>
                <n-li>RocketMQ — <n-text code>10477</n-text></n-li>
              </n-ul>
              <span>
                {{
                  $gettext(
                    'Go to Grafana → Dashboards → New → Import, enter the ID above and select the Prometheus data source.',
                  )
                }}
              </span>
            </n-flex>
          </n-alert>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="grafana.config"
          :save="grafana.saveConfig"
          :tip="
            $gettext(
              'This modifies the Grafana main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <grafana-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="datasources" :tab="$gettext('Data Sources')">
        <grafana-data-sources-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="grafana.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="grafana" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

<script setup lang="ts">
defineOptions({
  name: 'apps-clickhouse-index',
})

import clickhouse from '@/api/apps/clickhouse'

import ClickHouseConfigTuneView from './ClickHouseConfigTuneView.vue'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="clickhouse-server" />
          <password-card
            :title="$gettext('Default User Password')"
            :tip="
              $gettext(
                'The ClickHouse default user password is used to access the database. Keep it safe!',
              )
            "
            :fetch="clickhouse.defaultPassword"
            :save="clickhouse.setDefaultPassword"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="clickhouse.config"
          :save="clickhouse.saveConfig"
          :tip="
            $gettext(
              'This modifies the ClickHouse configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <click-house-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="clickhouse.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="clickhouse-server" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

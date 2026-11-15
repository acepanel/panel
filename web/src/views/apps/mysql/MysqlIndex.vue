<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import type mysql from '@/api/apps/mysql'
import file from '@/api/panel/file'

import MysqlConfigTuneView from './MysqlConfigTuneView.vue'
import MysqlMaintenanceView from './MysqlMaintenanceView.vue'
import MysqlPerformanceView from './MysqlPerformanceView.vue'

const props = defineProps<{
  api: typeof mysql
  name: string
}>()

const { $gettext } = useGettext()
const currentTab = ref('status')

const { data: slowLog } = useRequest(props.api.slowLog, {
  initialData: '',
})

const clearSlowLog = () => file.truncate(slowLog.value)

const rootPasswordTip = $gettext(
  'The "root" user password is used to manage the database system. Keep it safe!',
)
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="mysqld" />
          <password-card
            :title="$gettext('Root Password')"
            :tip="rootPasswordTip"
            :fetch="props.api.rootPassword"
            :save="props.api.setRootPassword"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <config-editor-pane
          :fetch="props.api.config"
          :save="props.api.saveConfig"
          :tip="
            $gettext(
              'This modifies the %{ name } main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              { name },
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <mysql-config-tune-view :api="props.api" />
      </n-tab-pane>
      <n-tab-pane name="performance" :tab="$gettext('Performance')">
        <mysql-performance-view :api="props.api" />
      </n-tab-pane>
      <n-tab-pane name="maintenance" :tab="$gettext('Maintenance')">
        <mysql-maintenance-view :api="props.api" />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="props.api.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <clearable-log service="mysqld" />
      </n-tab-pane>
      <n-tab-pane name="slow-log" :tab="$gettext('Slow Query Log')">
        <clearable-log :path="slowLog" :clear="clearSlowLog" :label="$gettext('Clear Slow Log')" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

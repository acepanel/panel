<script setup lang="ts">
defineOptions({
  name: 'apps-postgresql-index',
})

import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'
import file from '@/api/panel/file'

import PostgresqlConfigTuneView from './PostgresqlConfigTuneView.vue'
import PostgresqlExtensionsView from './PostgresqlExtensionsView.vue'
import PostgresqlMaintenanceView from './PostgresqlMaintenanceView.vue'
import PostgresqlPerformanceView from './PostgresqlPerformanceView.vue'

const { $gettext, $pgettext } = useGettext()
const currentTab = ref('status')

const selectedLog = ref('')
const { data: logList } = useRequest(postgresql.log, {
  initialData: [],
}).onSuccess(({ data }) => {
  if (!data?.length) {
    selectedLog.value = ''
    return
  }
  // log_filename='postgresql-%a.log' 按 weekday 缩写循环；优先选中今天对应的文件
  const weekdays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
  const today = `postgresql-${weekdays[new Date().getDay()]}.log`
  selectedLog.value = data.find((p: string) => p.endsWith(today)) ?? data[data.length - 1]
})

const logOptions = computed(() =>
  (logList.value as string[]).map((p) => ({
    label: p.split('/').pop() ?? p,
    value: p,
  })),
)

const clearAppLog = () => file.truncate(selectedLog.value)

const postgresPasswordTip = $gettext(
  'The "postgres" superuser password is used to manage the database system. Keep it safe!',
)
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="postgresql" show-reload />
          <password-card
            :title="$gettext('Super Password')"
            :tip="postgresPasswordTip"
            :fetch="postgresql.postgresPassword"
            :save="postgresql.setPostgresPassword"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="postgresql.config"
          :save="postgresql.saveConfig"
          :tip="
            $gettext(
              'This modifies the PostgreSQL main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <postgresql-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="extensions" :tab="$pgettext('postgresql', 'Extensions')">
        <postgresql-extensions-view />
      </n-tab-pane>
      <n-tab-pane name="performance" :tab="$gettext('Performance')">
        <postgresql-performance-view />
      </n-tab-pane>
      <n-tab-pane name="maintenance" :tab="$gettext('Maintenance')">
        <postgresql-maintenance-view />
      </n-tab-pane>
      <n-tab-pane name="user-config" :tab="$gettext('User Configuration')">
        <config-editor-pane
          :fetch="postgresql.userConfig"
          :save="postgresql.saveUserConfig"
          :tip="
            $gettext(
              'This modifies the PostgreSQL user configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="postgresql.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <clearable-log service="postgresql" />
      </n-tab-pane>
      <n-tab-pane name="app-log" :tab="$gettext('Application Logs')">
        <clearable-log :path="selectedLog" :clear="clearAppLog">
          <n-select v-model:value="selectedLog" :options="logOptions" class="md:!w-60" />
        </clearable-log>
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

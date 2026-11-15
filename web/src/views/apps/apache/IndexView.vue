<script setup lang="ts">
defineOptions({
  name: 'apps-apache-index',
})

import apache from '@/api/apps/apache'

import ApacheConfigTuneView from './ApacheConfigTuneView.vue'

const currentTab = ref('status')

const { data: errorLog } = useRequest(apache.errorLog, {
  initialData: '',
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="apache" show-reload />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <config-editor-pane
          :fetch="apache.config"
          :save="apache.saveConfig"
          :tip="
            $gettext(
              'This modifies the %{name} main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              { name: 'Apache' },
            )
          "
          lang="apache"
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <apache-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="apache.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="apache" />
      </n-tab-pane>
      <n-tab-pane name="error-log" :tab="$gettext('Error Logs')">
        <clearable-log :path="errorLog" :clear="apache.clearErrorLog" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

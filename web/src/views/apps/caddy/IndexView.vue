<script setup lang="ts">
defineOptions({
  name: 'apps-caddy-index',
})

import caddy from '@/api/apps/caddy'

const currentTab = ref('status')

const { data: errorLog } = useRequest(caddy.errorLog, {
  initialData: '',
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="caddy" show-reload />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <config-editor-pane
          :fetch="caddy.config"
          :save="caddy.saveConfig"
          :tip="
            $gettext(
              'This modifies the %{name} main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              { name: 'Caddy' },
            )
          "
          lang="plaintext"
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="caddy" />
      </n-tab-pane>
      <n-tab-pane name="error-log" :tab="$gettext('Error Logs')">
        <clearable-log :path="errorLog" :clear="caddy.clearErrorLog" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

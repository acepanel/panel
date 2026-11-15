<script setup lang="ts">
defineOptions({
  name: 'apps-codeserver-index',
})

import codeserver from '@/api/apps/codeserver'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="code-server" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <config-editor-pane
          :fetch="codeserver.config"
          :save="codeserver.saveConfig"
          :tip="
            $gettext(
              'This modifies the Code Server configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="code-server" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

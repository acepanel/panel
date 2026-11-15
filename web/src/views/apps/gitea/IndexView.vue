<script setup lang="ts">
defineOptions({
  name: 'apps-gitea-index',
})

import gitea from '@/api/apps/gitea'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="gitea" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <config-editor-pane
          :fetch="gitea.config"
          :save="gitea.saveConfig"
          :tip="
            $gettext(
              'This modifies the Gitea configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="gitea" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

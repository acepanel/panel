<script setup lang="ts">
defineOptions({
  name: 'apps-minio-index',
})

import minio from '@/api/apps/minio'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="minio" />
      </n-tab-pane>
      <n-tab-pane name="env" :tab="$gettext('Environment Variables')">
        <config-editor-pane
          :fetch="minio.env"
          :save="minio.saveEnv"
          :tip="
            $gettext(
              'This is modifying the MinIO environment variable file /etc/default/minio. If you do not understand the meaning of each parameter, please do not modify it arbitrarily!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="minio" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

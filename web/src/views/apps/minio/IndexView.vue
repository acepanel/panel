<script setup lang="ts">
defineOptions({
  name: 'apps-minio-index'
})

import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import minio from '@/api/apps/minio'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveEnvLoading = ref(false)

const { data: env } = useRequest(minio.env, {
  initialData: ''
})

const handleSaveEnv = () => {
  saveEnvLoading.value = true
  useRequest(minio.saveEnv(env.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveEnvLoading.value = false
    })
}
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="minio" />
      </n-tab-pane>
      <n-tab-pane name="env" :tab="$gettext('Environment Variables')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This is modifying the MinIO environment variable file /etc/default/minio. If you do not understand the meaning of each parameter, please do not modify it arbitrarily!'
              )
            }}
          </n-alert>
          <common-editor v-model:value="env" height="60vh" />
          <n-flex>
            <n-button type="primary" :loading="saveEnvLoading" :disabled="saveEnvLoading" @click="handleSaveEnv">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="minio" />
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>

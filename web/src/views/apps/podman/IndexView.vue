<script setup lang="ts">
defineOptions({
  name: 'apps-podman-index'
})

import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import podman from '@/api/apps/podman'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveRegistryConfigLoading = ref(false)
const saveStorageConfigLoading = ref(false)

const { data: registryConfig } = useRequest(podman.registryConfig, {
  initialData: ''
})

const { data: storageConfig } = useRequest(podman.storageConfig, {
  initialData: ''
})

const handleSaveRegistryConfig = () => {
  saveRegistryConfigLoading.value = true
  useRequest(podman.saveRegistryConfig(registryConfig.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveRegistryConfigLoading.value = false
    })
}

const handleSaveStorageConfig = () => {
  saveStorageConfigLoading.value = true
  useRequest(podman.saveStorageConfig(storageConfig.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveStorageConfigLoading.value = false
    })
}
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'Podman is a daemonless container management tool. Being in a stopped state is normal and does not affect usage!'
              )
            }}
          </n-alert>
          <service-status service="podman" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="registryConfig" :tab="$gettext('Registry Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the Podman registry configuration file (/etc/containers/registries.conf)'
              )
            }}
          </n-alert>
          <common-editor v-model:value="registryConfig" height="60vh" />
          <n-flex>
            <n-button type="primary" :loading="saveRegistryConfigLoading" :disabled="saveRegistryConfigLoading" @click="handleSaveRegistryConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="storageConfig" :tab="$gettext('Storage Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the Podman storage configuration file (/etc/containers/storage.conf)'
              )
            }}
          </n-alert>
          <common-editor v-model:value="storageConfig" height="60vh" />
          <n-flex>
            <n-button type="primary" :loading="saveStorageConfigLoading" :disabled="saveStorageConfigLoading" @click="handleSaveStorageConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="podman" />
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>

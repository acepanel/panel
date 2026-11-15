<script setup lang="ts">
defineOptions({
  name: 'apps-podman-index',
})

import podman from '@/api/apps/podman'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'Podman is a daemonless container management tool. Being in a stopped state is normal and does not affect usage!',
              )
            }}
          </n-alert>
          <service-status service="podman" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="registryConfig" :tab="$gettext('Registry Configuration')">
        <config-editor-pane
          :fetch="podman.registryConfig"
          :save="podman.saveRegistryConfig"
          :tip="
            $gettext(
              'This modifies the Podman registry configuration file (/etc/containers/registries.conf)',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="storageConfig" :tab="$gettext('Storage Configuration')">
        <config-editor-pane
          :fetch="podman.storageConfig"
          :save="podman.saveStorageConfig"
          :tip="
            $gettext(
              'This modifies the Podman storage configuration file (/etc/containers/storage.conf)',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="podman" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

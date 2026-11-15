<script setup lang="ts">
defineOptions({
  name: 'apps-phpmyadmin-index',
})

import { useGettext } from 'vue3-gettext'

import phpmyadmin from '@/api/apps/phpmyadmin'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveLoading = ref(false)
const hostname = ref(window.location.hostname)
const port = ref(0)
const path = ref('')
const newPort = ref(0)
const url = computed(() => {
  return `http://${hostname.value}:${port.value}/${path.value}`
})

const getInfo = async () => {
  const data = await phpmyadmin.info()
  path.value = data.path
  port.value = data.port
  newPort.value = data.port
}

const handleSave = () => {
  saveLoading.value = true
  useRequest(phpmyadmin.port(newPort.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
      getInfo()
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}

onMounted(() => {
  getInfo()
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Status')">
        <n-flex vertical>
          <n-card :title="$gettext('Access Information')">
            <n-alert type="info">
              {{ $gettext('Access URL:') }} <a :href="url" target="_blank">{{ url }}</a>
            </n-alert>
          </n-card>
          <n-card :title="$gettext('Modify Port')">
            <n-flex>
              <n-input-number v-model:value="newPort" :min="1" :max="65535" />
              <n-button
                type="primary"
                :loading="saveLoading"
                :disabled="saveLoading"
                @click="handleSave"
              >
                {{ $gettext('Save') }}
              </n-button>
            </n-flex>
            {{ $gettext('Modify phpMyAdmin access port') }}
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <config-editor-pane
          :fetch="phpmyadmin.config"
          :save="phpmyadmin.updateConfig"
          :tip="
            $gettext(
              'This modifies the web server configuration file for phpMyAdmin. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

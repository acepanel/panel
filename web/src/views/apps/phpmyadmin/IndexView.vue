<script setup lang="ts">
defineOptions({
  name: 'apps-phpmyadmin-index'
})

import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import phpmyadmin from '@/api/apps/phpmyadmin'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveLoading = ref(false)
const saveConfigLoading = ref(false)
const hostname = ref(window.location.hostname)
const port = ref(0)
const path = ref('')
const newPort = ref(0)
const url = computed(() => {
  return `http://${hostname.value}:${port.value}/${path.value}`
})

const { data: config } = useRequest(phpmyadmin.config, {
  initialData: {
    config: ''
  }
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

const handleSaveConfig = () => {
  saveConfigLoading.value = true
  useRequest(phpmyadmin.updateConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

onMounted(() => {
  getInfo()
})
</script>

<template>
  <common-page show-footer>
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
              <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
                {{ $gettext('Save') }}
              </n-button>
            </n-flex>
            {{ $gettext('Modify phpMyAdmin access port') }}
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the OpenResty configuration file for phpMyAdmin. If you do not understand the meaning of each parameter, please do not modify it randomly!'
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button type="primary" :loading="saveConfigLoading" :disabled="saveConfigLoading" @click="handleSaveConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>

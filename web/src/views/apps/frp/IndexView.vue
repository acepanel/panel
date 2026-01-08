<script setup lang="ts">
defineOptions({
  name: 'apps-frp-index'
})

import { NButton, NFormItem, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

const { $gettext } = useGettext()
const currentTab = ref('frps')
const config = ref({
  frpc: '',
  frps: ''
})
const userInfo = ref({
  frpc: { user: '', group: '' },
  frps: { user: '', group: '' }
})

const getConfig = async () => {
  config.value.frps = await frp.config('frps')
  config.value.frpc = await frp.config('frpc')
}

const getUser = async () => {
  userInfo.value.frps = await frp.user('frps')
  userInfo.value.frpc = await frp.user('frpc')
}

const handleSaveConfig = (service: string) => {
  useRequest(frp.saveConfig(service, config.value[service as keyof typeof config.value])).onSuccess(
    () => {
      window.$message.success($gettext('Saved successfully'))
    }
  )
}

const handleSaveUser = (service: string) => {
  const info = userInfo.value[service as keyof typeof userInfo.value]
  useRequest(frp.saveUser(service, info.user, info.group)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

onMounted(() => {
  getConfig()
  getUser()
})
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="frps" tab="Frps">
        <n-flex vertical>
          <service-status service="frps" />
          <n-card :title="$gettext('Run User')">
            <template #header-extra>
              <n-button type="primary" @click="handleSaveUser('frps')">
                {{ $gettext('Save') }}
              </n-button>
            </template>
            <n-flex>
              <n-form-item :label="$gettext('User')">
                <n-input v-model:value="userInfo.frps.user" :placeholder="$gettext('User')" />
              </n-form-item>
              <n-form-item :label="$gettext('Group')">
                <n-input v-model:value="userInfo.frps.group" :placeholder="$gettext('Group')" />
              </n-form-item>
            </n-flex>
          </n-card>
          <n-card :title="$gettext('Modify Configuration')">
            <template #header-extra>
              <n-button type="primary" @click="handleSaveConfig('frps')">
                {{ $gettext('Save') }}
              </n-button>
            </template>
            <common-editor v-model:value="config.frps" height="60vh" />
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="frpc" tab="Frpc">
        <n-flex vertical>
          <service-status service="frpc" />
          <n-card :title="$gettext('Run User')">
            <template #header-extra>
              <n-button type="primary" @click="handleSaveUser('frpc')">
                {{ $gettext('Save') }}
              </n-button>
            </template>
            <n-flex>
              <n-form-item :label="$gettext('User')">
                <n-input v-model:value="userInfo.frpc.user" :placeholder="$gettext('User')" />
              </n-form-item>
              <n-form-item :label="$gettext('Group')">
                <n-input v-model:value="userInfo.frpc.group" :placeholder="$gettext('Group')" />
              </n-form-item>
            </n-flex>
          </n-card>
          <n-card :title="$gettext('Modify Configuration')">
            <template #header-extra>
              <n-button type="primary" @click="handleSaveConfig('frpc')">
                {{ $gettext('Save') }}
              </n-button>
            </template>
            <common-editor v-model:value="config.frpc" height="60vh" />
          </n-card>
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>

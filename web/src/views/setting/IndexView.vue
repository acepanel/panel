<script setup lang="ts">
defineOptions({
  name: 'setting-index'
})

import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import setting from '@/api/panel/setting'
import { usePermissionStore, useThemeStore } from '@/store'
import CreateModal from '@/views/setting/CreateModal.vue'
import SettingBase from '@/views/setting/SettingBase.vue'
import SettingSafe from '@/views/setting/SettingSafe.vue'
import SettingUser from '@/views/setting/SettingUser.vue'

const { $gettext } = useGettext()
const themeStore = useThemeStore()
const permissionStore = usePermissionStore()
const currentTab = ref('base')
const createModal = ref(false)

const { data: model } = useRequest(setting.list, {
  initialData: {
    name: '',
    channel: 'stable',
    locale: 'en',
    port: 8888,
    entrance: '',
    entrance_error: '418',
    login_captcha: false,
    offline_mode: false,
    two_fa: false,
    lifetime: 0,
    ip_header: '',
    bind_domain: [],
    bind_ip: [],
    bind_ua: [],
    website_path: '',
    backup_path: '',
    hidden_menu: [],
    custom_logo: '',
    https: false,
    acme: false,
    public_ip: [],
    cert: '',
    key: ''
  }
})

const handleSave = () => {
  if (model.value.entrance.trim() === '') {
    model.value.entrance = '/'
  }
  useRequest(setting.update(model.value)).onSuccess(({ data }) => {
    window.$message.success($gettext('Saved successfully'))

    // 更新语言设置
    if (model.value.locale !== themeStore.locale) {
      themeStore.setLocale(model.value.locale)
    }

    // 更新隐藏菜单和自定义 Logo
    themeStore.setLogo(model.value.custom_logo || '')
    permissionStore.setHiddenRoutes(model.value.hidden_menu || [])

    // 如果需要重启，则自动刷新页面
    if (data.restart) {
      window.$message.info($gettext('Panel is restarting, page will refresh in 5 seconds'))
      setTimeout(() => {
        const protocol = model.value.https ? 'https:' : 'http:'
        const hostname = window.location.hostname
        const port = model.value.port
        const entrance = model.value.entrance || '/'
        // 构建新的 URL
        window.location.href = `${protocol}//${hostname}:${port}${entrance}`
      }, 5000)
    }
  })
}

const handleCreate = () => {
  createModal.value = true
}
</script>

<template>
  <common-page show-header show-footer>
    <template #tabbar>
      <n-tabs v-model:value="currentTab" animated>
        <n-tab name="base" :tab="$gettext('Basic')" />
        <n-tab name="safe" :tab="$gettext('Safe')" />
        <n-tab name="user" :tab="$gettext('User')" />
      </n-tabs>
    </template>
    <n-flex vertical>
      <n-flex>
        <n-button v-if="currentTab == 'user'" type="primary" @click="handleCreate">
          {{ $gettext('Create User') }}
        </n-button>
      </n-flex>
      <setting-base v-if="currentTab === 'base'" v-model:model="model" />
      <setting-safe v-if="currentTab === 'safe'" v-model:model="model" />
      <setting-user v-if="currentTab === 'user'" />
      <n-flex>
        <n-button v-if="currentTab != 'user'" type="primary" @click="handleSave">
          {{ $gettext('Save') }}
        </n-button>
      </n-flex>
    </n-flex>
  </common-page>
  <create-modal v-model:show="createModal" />
</template>

<style scoped lang="scss"></style>

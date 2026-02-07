<script setup lang="ts">
defineOptions({
  name: 'setting-index'
})

import type { MessageReactive } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import setting from '@/api/panel/setting'
import { usePermissionStore, useThemeStore } from '@/store'
import CreateModal from '@/views/setting/CreateModal.vue'
import SettingBase from '@/views/setting/SettingBase.vue'
import SettingSafe from '@/views/setting/SettingSafe.vue'
import SettingUser from '@/views/setting/SettingUser.vue'

let messageReactive: MessageReactive | null = null
const { $gettext } = useGettext()
const themeStore = useThemeStore()
const permissionStore = usePermissionStore()
const currentTab = ref('base')
const createModal = ref(false)
const isObtainCert = ref(false)
const saveLoading = ref(false)

// 记录已保存的 HTTPS 相关设置，用于判断是否有未保存的修改
const savedHttpsState = ref({ https: false, acme: false, public_ip: '[]' })
const httpsSettingsDirty = computed(() => {
  return (
    model.value.https !== savedHttpsState.value.https ||
    model.value.acme !== savedHttpsState.value.acme ||
    JSON.stringify(model.value.public_ip) !== savedHttpsState.value.public_ip
  )
})
const snapshotHttpsState = () => {
  savedHttpsState.value = {
    https: model.value.https,
    acme: model.value.acme,
    public_ip: JSON.stringify(model.value.public_ip)
  }
}

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
    project_path: '',
    hidden_menu: [],
    custom_logo: '',
    https: false,
    acme: false,
    public_ip: [],
    cert: '',
    key: ''
  }
})

// 数据加载完成后快照 HTTPS 状态
watch(model, () => snapshotHttpsState(), { once: true, deep: true })

const handleSave = () => {
  if (model.value.entrance.trim() === '') {
    model.value.entrance = '/'
  }
  saveLoading.value = true
  useRequest(setting.update(model.value))
    .onSuccess(({ data }) => {
    window.$message.success($gettext('Saved successfully'))

    // 更新 HTTPS 快照
    snapshotHttpsState()

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
    .onComplete(() => {
      saveLoading.value = false
    })
}

const handleObtainCert = () => {
  isObtainCert.value = true
  messageReactive = window.$message.loading($gettext('Please wait...'), {
    duration: 0
  })
  useRequest(setting.obtainCert())
    .onSuccess(() => {
      window.$message.success($gettext('Certificate refreshed successfully'))
      window.$message.info($gettext('Panel is restarting, page will refresh in 5 seconds'))
      setTimeout(() => {
        const hostname = window.location.hostname
        const port = model.value.port
        const entrance = model.value.entrance || '/'
        window.location.href = `https://${hostname}:${port}${entrance}`
      }, 5000)
    })
    .onComplete(() => {
      isObtainCert.value = false
      messageReactive?.destroy()
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
        <n-button v-if="currentTab != 'user'" type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
          {{ $gettext('Save') }}
        </n-button>
        <n-button
          v-if="currentTab === 'safe' && model.https && model.acme"
          type="info"
          :disabled="httpsSettingsDirty || isObtainCert"
          @click="handleObtainCert"
        >
          {{ $gettext('Refresh Certificate') }}
        </n-button>
      </n-flex>
    </n-flex>
  </common-page>
  <create-modal v-model:show="createModal" />
</template>

<style scoped lang="scss"></style>

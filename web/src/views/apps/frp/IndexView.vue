<script setup lang="ts">
defineOptions({
  name: 'apps-frp-index',
})

import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'

import FrpcConfigTuneView from './FrpcConfigTuneView.vue'
import FrpProxyView from './FrpProxyView.vue'
import FrpsConfigTuneView from './FrpsConfigTuneView.vue'
import FrpVisitorView from './FrpVisitorView.vue'

const { $gettext } = useGettext()
const currentTab = ref('frps')
const frpsTab = ref('status')
const frpcTab = ref('status')
const saveUserFrpsLoading = ref(false)
const saveUserFrpcLoading = ref(false)
const userInfo = ref({
  frpc: { user: '', group: '' },
  frps: { user: '', group: '' },
})

const fetchFrpsConfig = () => frp.config('frps')
const saveFrpsConfig = (content: string) => frp.saveConfig('frps', content)
const fetchFrpcConfig = () => frp.config('frpc')
const saveFrpcConfig = (content: string) => frp.saveConfig('frpc', content)

const getUser = async () => {
  ;[userInfo.value.frps, userInfo.value.frpc] = await Promise.all([
    frp.user('frps'),
    frp.user('frpc'),
  ])
}

const handleSaveUser = (service: string) => {
  const info = userInfo.value[service as keyof typeof userInfo.value]
  const loadingRef = service === 'frps' ? saveUserFrpsLoading : saveUserFrpcLoading
  loadingRef.value = true
  useRequest(frp.saveUser(service, info.user, info.group))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      loadingRef.value = false
    })
}

onMounted(() => {
  getUser()
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="frps" tab="Frps">
        <n-tabs v-model:value="frpsTab" type="line" animated>
          <n-tab-pane name="status" :tab="$gettext('Running Status')">
            <service-status service="frps" />
          </n-tab-pane>
          <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
            <frps-config-tune-view />
          </n-tab-pane>
          <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
            <config-editor-pane
              :fetch="fetchFrpsConfig"
              :save="saveFrpsConfig"
              :tip="
                $gettext(
                  'This modifies the Frps main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
                )
              "
            />
          </n-tab-pane>
          <n-tab-pane name="user" :tab="$gettext('Run User')">
            <n-flex vertical>
              <n-form inline>
                <n-form-item :label="$gettext('User')">
                  <n-input v-model:value="userInfo.frps.user" :placeholder="$gettext('User')" />
                </n-form-item>
                <n-form-item :label="$gettext('Group')">
                  <n-input v-model:value="userInfo.frps.group" :placeholder="$gettext('Group')" />
                </n-form-item>
              </n-form>
              <n-flex>
                <n-button
                  type="primary"
                  :loading="saveUserFrpsLoading"
                  :disabled="saveUserFrpsLoading"
                  @click="handleSaveUser('frps')"
                >
                  {{ $gettext('Save') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>
          <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
            <realtime-log service="frps" />
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="frpc" tab="Frpc">
        <n-tabs v-model:value="frpcTab" type="line" animated>
          <n-tab-pane name="status" :tab="$gettext('Running Status')">
            <service-status service="frpc" />
          </n-tab-pane>
          <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
            <frpc-config-tune-view />
          </n-tab-pane>
          <n-tab-pane name="proxies" :tab="$gettext('Proxy Management')">
            <frp-proxy-view />
          </n-tab-pane>
          <n-tab-pane name="visitors" :tab="$gettext('Visitor Management')">
            <frp-visitor-view />
          </n-tab-pane>
          <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
            <config-editor-pane
              :fetch="fetchFrpcConfig"
              :save="saveFrpcConfig"
              :tip="
                $gettext(
                  'This modifies the Frpc main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
                )
              "
            />
          </n-tab-pane>
          <n-tab-pane name="user" :tab="$gettext('Run User')">
            <n-flex vertical>
              <n-form inline>
                <n-form-item :label="$gettext('User')">
                  <n-input v-model:value="userInfo.frpc.user" :placeholder="$gettext('User')" />
                </n-form-item>
                <n-form-item :label="$gettext('Group')">
                  <n-input v-model:value="userInfo.frpc.group" :placeholder="$gettext('Group')" />
                </n-form-item>
              </n-form>
              <n-flex>
                <n-button
                  type="primary"
                  :loading="saveUserFrpcLoading"
                  :disabled="saveUserFrpcLoading"
                  @click="handleSaveUser('frpc')"
                >
                  {{ $gettext('Save') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-tab-pane>
          <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
            <realtime-log service="frpc" />
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>

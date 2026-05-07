<script setup lang="ts">
defineOptions({
  name: 'mongodb-config-tune',
})

import { useGettext } from 'vue3-gettext'

import mongodb from '@/api/apps/mongodb'

const { $gettext } = useGettext()
const currentTab = ref('storage')

const dbPath = ref('')
const cacheSizeGB = ref('')
const port = ref('')
const bindIp = ref('')
const systemLogPath = ref('')
const authorization = ref('')

const saveLoading = ref(false)

const authorizationOptions = [
  { label: 'enabled', value: 'enabled' },
  { label: 'disabled', value: 'disabled' },
]

useRequest(mongodb.configTune()).onSuccess(({ data }: any) => {
  dbPath.value = data.db_path ?? ''
  cacheSizeGB.value = data.cache_size_gb ?? ''
  port.value = data.port ?? ''
  bindIp.value = data.bind_ip ?? ''
  systemLogPath.value = data.system_log_path ?? ''
  authorization.value = data.authorization ?? ''
})

const getConfigData = () => ({
  db_path: dbPath.value,
  cache_size_gb: cacheSizeGB.value,
  port: port.value,
  bind_ip: bindIp.value,
  system_log_path: systemLogPath.value,
  authorization: authorization.value,
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(mongodb.saveConfigTune(getConfigData()))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <n-tab-pane name="storage" :tab="$gettext('Storage')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('MongoDB storage engine and data path settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Data Path (storage.dbPath)')">
            <n-input v-model:value="dbPath" placeholder="/data/db" />
          </n-form-item>
          <n-form-item
            :label="
              $gettext('WiredTiger Cache Size GB (storage.wiredTiger.engineConfig.cacheSizeGB)')
            "
          >
            <n-input v-model:value="cacheSizeGB" placeholder="0.5" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="network" :tab="$gettext('Network')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('MongoDB network and security settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Port (net.port)')">
            <n-input v-model:value="port" placeholder="27017" />
          </n-form-item>
          <n-form-item :label="$gettext('Bind IP (net.bindIp)')">
            <n-input v-model:value="bindIp" placeholder="127.0.0.1" />
          </n-form-item>
          <n-form-item :label="$gettext('Authorization (security.authorization)')">
            <n-select v-model:value="authorization" :options="authorizationOptions" clearable />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="logging" :tab="$gettext('Logging')">
      <n-flex vertical>
        <n-form>
          <n-form-item :label="$gettext('System Log Path (systemLog.path)')">
            <n-input v-model:value="systemLogPath" placeholder="/var/log/mongodb/mongod.log" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

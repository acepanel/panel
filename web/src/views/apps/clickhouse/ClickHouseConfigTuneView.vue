<script setup lang="ts">
defineOptions({
  name: 'clickhouse-config-tune',
})

import { useGettext } from 'vue3-gettext'

import clickhouse from '@/api/apps/clickhouse'

const { $gettext } = useGettext()
const currentTab = ref('network')

const listenHost = ref('')
const httpPort = ref('')
const tcpPort = ref('')
const maxMemoryUsage = ref('')
const maxThreads = ref('')
const path = ref('')
const tmpPath = ref('')
const logLevel = ref('')

const saveLoading = ref(false)

const logLevelOptions = [
  { label: 'trace', value: 'trace' },
  { label: 'debug', value: 'debug' },
  { label: 'information', value: 'information' },
  { label: 'warning', value: 'warning' },
  { label: 'error', value: 'error' },
]

useRequest(clickhouse.configTune()).onSuccess(({ data }: any) => {
  listenHost.value = data.listen_host ?? ''
  httpPort.value = data.http_port ?? ''
  tcpPort.value = data.tcp_port ?? ''
  maxMemoryUsage.value = data.max_memory_usage ?? ''
  maxThreads.value = data.max_threads ?? ''
  path.value = data.path ?? ''
  tmpPath.value = data.tmp_path ?? ''
  logLevel.value = data.log_level ?? ''
})

const getConfigData = () => ({
  listen_host: listenHost.value,
  http_port: httpPort.value,
  tcp_port: tcpPort.value,
  max_memory_usage: maxMemoryUsage.value,
  max_threads: maxThreads.value,
  path: path.value,
  tmp_path: tmpPath.value,
  log_level: logLevel.value,
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(clickhouse.saveConfigTune(getConfigData()))
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
    <n-tab-pane name="network" :tab="$gettext('Network')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('ClickHouse network settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Listen Host')">
            <n-input v-model:value="listenHost" placeholder="127.0.0.1" />
          </n-form-item>
          <n-form-item :label="$gettext('HTTP Port')">
            <n-input v-model:value="httpPort" placeholder="8123" />
          </n-form-item>
          <n-form-item :label="$gettext('TCP Port')">
            <n-input v-model:value="tcpPort" placeholder="9000" />
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
    <n-tab-pane name="performance" :tab="$gettext('Performance')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('ClickHouse performance settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Max Memory Usage')">
            <n-input v-model:value="maxMemoryUsage" placeholder="10000000000" />
          </n-form-item>
          <n-form-item :label="$gettext('Max Threads')">
            <n-input v-model:value="maxThreads" placeholder="0" />
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
    <n-tab-pane name="paths" :tab="$gettext('Paths')">
      <n-flex vertical>
        <n-form>
          <n-form-item :label="$gettext('Data Path')">
            <n-input v-model:value="path" placeholder="/var/lib/clickhouse/" />
          </n-form-item>
          <n-form-item :label="$gettext('Temp Path')">
            <n-input v-model:value="tmpPath" placeholder="/var/lib/clickhouse/tmp/" />
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
          <n-form-item :label="$gettext('Log Level (logger.level)')">
            <n-select v-model:value="logLevel" :options="logLevelOptions" clearable />
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

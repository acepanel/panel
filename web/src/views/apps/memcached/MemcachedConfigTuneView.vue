<script setup lang="ts">
defineOptions({
  name: 'memcached-config-tune'
})

import { useGettext } from 'vue3-gettext'

import memcached from '@/api/apps/memcached'

const { $gettext } = useGettext()

const port = ref('')
const udpPort = ref('')
const listenAddress = ref('')
const memory = ref('')
const maxConnections = ref('')
const threads = ref('')

const saveLoading = ref(false)

useRequest(memcached.configTune()).onSuccess(({ data }: any) => {
  port.value = data.port ?? ''
  udpPort.value = data.udp_port ?? ''
  listenAddress.value = data.listen_address ?? ''
  memory.value = data.memory ?? ''
  maxConnections.value = data.max_connections ?? ''
  threads.value = data.threads ?? ''
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(
    memcached.saveConfigTune({
      port: port.value,
      udp_port: udpPort.value,
      listen_address: listenAddress.value,
      memory: memory.value,
      max_connections: maxConnections.value,
      threads: threads.value
    })
  )
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">
      {{ $gettext('Common Memcached settings.') }}
    </n-alert>
    <n-form>
      <n-form-item :label="$gettext('Port (-p)')">
        <n-input v-model:value="port" :placeholder="$gettext('e.g. 11211')" />
      </n-form-item>
      <n-form-item :label="$gettext('UDP Port (-U, 0 to disable)')">
        <n-input v-model:value="udpPort" :placeholder="$gettext('e.g. 0')" />
      </n-form-item>
      <n-form-item :label="$gettext('Listen Address (-l)')">
        <n-input v-model:value="listenAddress" :placeholder="$gettext('e.g. 127.0.0.1')" />
      </n-form-item>
      <n-form-item :label="$gettext('Memory (-m, MB)')">
        <n-input v-model:value="memory" :placeholder="$gettext('e.g. 64')" />
      </n-form-item>
      <n-form-item :label="$gettext('Max Connections (-c)')">
        <n-input v-model:value="maxConnections" :placeholder="$gettext('e.g. 1024')" />
      </n-form-item>
      <n-form-item :label="$gettext('Threads (-t)')">
        <n-input v-model:value="threads" :placeholder="$gettext('e.g. 4')" />
      </n-form-item>
    </n-form>
    <n-flex>
      <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>

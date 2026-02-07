<script setup lang="ts">
defineOptions({
  name: 'memcached-config-tune'
})

import { useGettext } from 'vue3-gettext'

import memcached from '@/api/apps/memcached'

const { $gettext } = useGettext()

const port = ref<number | null>(null)
const udpPort = ref<number | null>(null)
const listenAddress = ref('')
const memory = ref<number | null>(null)
const maxConnections = ref<number | null>(null)
const threads = ref<number | null>(null)

const saveLoading = ref(false)

useRequest(memcached.configTune()).onSuccess(({ data }: any) => {
  port.value = Number(data.port) || null
  udpPort.value = Number(data.udp_port) ?? null
  listenAddress.value = data.listen_address ?? ''
  memory.value = Number(data.memory) || null
  maxConnections.value = Number(data.max_connections) || null
  threads.value = Number(data.threads) || null
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(
    memcached.saveConfigTune({
      port: String(port.value ?? ''),
      udp_port: String(udpPort.value ?? ''),
      listen_address: listenAddress.value,
      memory: String(memory.value ?? ''),
      max_connections: String(maxConnections.value ?? ''),
      threads: String(threads.value ?? '')
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
        <n-input-number class="w-full" v-model:value="port" :placeholder="$gettext('e.g. 11211')" :min="1" :max="65535" />
      </n-form-item>
      <n-form-item :label="$gettext('UDP Port (-U, 0 to disable)')">
        <n-input-number class="w-full" v-model:value="udpPort" :placeholder="$gettext('e.g. 0')" :min="0" :max="65535" />
      </n-form-item>
      <n-form-item :label="$gettext('Listen Address (-l)')">
        <n-input v-model:value="listenAddress" :placeholder="$gettext('e.g. 127.0.0.1')" />
      </n-form-item>
      <n-form-item :label="$gettext('Memory (-m, MB)')">
        <n-input-number class="w-full" v-model:value="memory" :placeholder="$gettext('e.g. 64')" :min="1" />
      </n-form-item>
      <n-form-item :label="$gettext('Max Connections (-c)')">
        <n-input-number class="w-full" v-model:value="maxConnections" :placeholder="$gettext('e.g. 1024')" :min="1" />
      </n-form-item>
      <n-form-item :label="$gettext('Threads (-t)')">
        <n-input-number class="w-full" v-model:value="threads" :placeholder="$gettext('e.g. 4')" :min="1" />
      </n-form-item>
    </n-form>
    <n-flex>
      <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>

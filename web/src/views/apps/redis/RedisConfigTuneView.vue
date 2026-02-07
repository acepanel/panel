<script setup lang="ts">
defineOptions({
  name: 'redis-config-tune'
})

import { useGettext } from 'vue3-gettext'

import redis from '@/api/apps/redis'

const { $gettext } = useGettext()
const currentTab = ref('general')

// 常规设置
const bind = ref('')
const port = ref('')
const databases = ref('')
const requirepass = ref('')
const timeout = ref('')
const tcpKeepalive = ref('')

// 内存
const maxmemory = ref('')
const maxmemoryPolicy = ref('')

// 持久化
const appendonly = ref('')
const appendfsync = ref('')

const saveLoading = ref(false)

const maxmemoryPolicyOptions = [
  { label: 'noeviction', value: 'noeviction' },
  { label: 'allkeys-lru', value: 'allkeys-lru' },
  { label: 'allkeys-lfu', value: 'allkeys-lfu' },
  { label: 'allkeys-random', value: 'allkeys-random' },
  { label: 'volatile-lru', value: 'volatile-lru' },
  { label: 'volatile-lfu', value: 'volatile-lfu' },
  { label: 'volatile-random', value: 'volatile-random' },
  { label: 'volatile-ttl', value: 'volatile-ttl' }
]

const appendfsyncOptions = [
  { label: 'always', value: 'always' },
  { label: 'everysec', value: 'everysec' },
  { label: 'no', value: 'no' }
]

const yesNoOptions = [
  { label: 'yes', value: 'yes' },
  { label: 'no', value: 'no' }
]

useRequest(redis.configTune()).onSuccess(({ data }: any) => {
  bind.value = data.bind ?? ''
  port.value = data.port ?? ''
  databases.value = data.databases ?? ''
  requirepass.value = data.requirepass ?? ''
  timeout.value = data.timeout ?? ''
  tcpKeepalive.value = data.tcp_keepalive ?? ''
  maxmemory.value = data.maxmemory ?? ''
  maxmemoryPolicy.value = data.maxmemory_policy ?? ''
  appendonly.value = data.appendonly ?? ''
  appendfsync.value = data.appendfsync ?? ''
})

const getConfigData = () => ({
  bind: bind.value,
  port: port.value,
  databases: databases.value,
  requirepass: requirepass.value,
  timeout: timeout.value,
  tcp_keepalive: tcpKeepalive.value,
  maxmemory: maxmemory.value,
  maxmemory_policy: maxmemoryPolicy.value,
  appendonly: appendonly.value,
  appendfsync: appendfsync.value
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(redis.saveConfigTune(getConfigData()))
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
    <n-tab-pane name="general" :tab="$gettext('General')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Common Redis general settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Bind (bind)">
            <n-input v-model:value="bind" :placeholder="$gettext('e.g. 127.0.0.1')" />
          </n-form-item>
          <n-form-item label="Port (port)">
            <n-input v-model:value="port" :placeholder="$gettext('e.g. 6379')" />
          </n-form-item>
          <n-form-item label="Databases (databases)">
            <n-input v-model:value="databases" :placeholder="$gettext('e.g. 16')" />
          </n-form-item>
          <n-form-item label="Password (requirepass)">
            <n-input
              v-model:value="requirepass"
              type="password"
              show-password-on="click"
              :placeholder="$gettext('Leave empty for no password')"
            />
          </n-form-item>
          <n-form-item label="Timeout (timeout)">
            <n-input
              v-model:value="timeout"
              :placeholder="$gettext('e.g. 0 (disabled) or seconds')"
            />
          </n-form-item>
          <n-form-item label="TCP Keepalive (tcp-keepalive)">
            <n-input v-model:value="tcpKeepalive" :placeholder="$gettext('e.g. 300')" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="memory" :tab="$gettext('Memory')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Redis memory management settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Max Memory (maxmemory)">
            <n-input
              v-model:value="maxmemory"
              :placeholder="$gettext('e.g. 256mb or 0 (no limit)')"
            />
          </n-form-item>
          <n-form-item label="Maxmemory Policy (maxmemory-policy)">
            <n-select v-model:value="maxmemoryPolicy" :options="maxmemoryPolicyOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="persistence" :tab="$gettext('Persistence')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Redis AOF persistence settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Append Only (appendonly)">
            <n-select v-model:value="appendonly" :options="yesNoOptions" />
          </n-form-item>
          <n-form-item label="Append Fsync (appendfsync)">
            <n-select v-model:value="appendfsync" :options="appendfsyncOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

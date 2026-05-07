<script setup lang="ts">
defineOptions({
  name: 'valkey-config-tune',
})

import { useGettext } from 'vue3-gettext'

import valkey from '@/api/apps/valkey'

const { $gettext } = useGettext()
const currentTab = ref('general')

const bind = ref('')
const port = ref<number | null>(null)
const databases = ref<number | null>(null)
const requirepass = ref('')
const timeout = ref<number | null>(null)
const tcpKeepalive = ref<number | null>(null)

const maxmemoryNum = ref<number | null>(null)
const maxmemoryUnit = ref('mb')
const maxmemoryPolicy = ref('')

const appendonly = ref('')
const appendfsync = ref('')

const saveLoading = ref(false)

const sizeUnitOptions = [
  { label: 'kb', value: 'kb' },
  { label: 'mb', value: 'mb' },
  { label: 'gb', value: 'gb' },
]

const parseSizeValue = (val: string): { num: number | null; unit: string } => {
  if (!val) return { num: null, unit: 'mb' }
  const match = val.match(/^(\d+)\s*(kb|mb|gb)$/i)
  if (match) {
    return { num: Number(match[1]), unit: match[2]!.toLowerCase() }
  }
  return { num: Number(val) || null, unit: 'mb' }
}

const composeSizeValue = (num: number | null, unit: string): string => {
  if (num == null) return ''
  return `${num}${unit}`
}

const maxmemoryPolicyOptions = [
  { label: 'noeviction', value: 'noeviction' },
  { label: 'allkeys-lru', value: 'allkeys-lru' },
  { label: 'allkeys-lfu', value: 'allkeys-lfu' },
  { label: 'allkeys-random', value: 'allkeys-random' },
  { label: 'volatile-lru', value: 'volatile-lru' },
  { label: 'volatile-lfu', value: 'volatile-lfu' },
  { label: 'volatile-random', value: 'volatile-random' },
  { label: 'volatile-ttl', value: 'volatile-ttl' },
]

const appendfsyncOptions = [
  { label: 'always', value: 'always' },
  { label: 'everysec', value: 'everysec' },
  { label: 'no', value: 'no' },
]

const yesNoOptions = [
  { label: 'yes', value: 'yes' },
  { label: 'no', value: 'no' },
]

useRequest(valkey.configTune()).onSuccess(({ data }: any) => {
  bind.value = data.bind ?? ''
  port.value = Number(data.port) || null
  databases.value = Number(data.databases) || null
  requirepass.value = data.requirepass ?? ''
  timeout.value = Number(data.timeout) || null
  tcpKeepalive.value = Number(data.tcp_keepalive) || null
  const mm = parseSizeValue(data.maxmemory ?? '')
  maxmemoryNum.value = mm.num
  maxmemoryUnit.value = mm.unit
  maxmemoryPolicy.value = data.maxmemory_policy || null
  appendonly.value = data.appendonly || null
  appendfsync.value = data.appendfsync || null
})

const getConfigData = () => ({
  bind: bind.value,
  port: String(port.value ?? ''),
  databases: String(databases.value ?? ''),
  requirepass: requirepass.value,
  timeout: String(timeout.value ?? ''),
  tcp_keepalive: String(tcpKeepalive.value ?? ''),
  maxmemory: composeSizeValue(maxmemoryNum.value, maxmemoryUnit.value),
  maxmemory_policy: maxmemoryPolicy.value ?? '',
  appendonly: appendonly.value ?? '',
  appendfsync: appendfsync.value ?? '',
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(valkey.saveConfigTune(getConfigData()))
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
          {{ $gettext('Common Valkey general settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Bind (bind)')">
            <n-input v-model:value="bind" :placeholder="$gettext('e.g. 127.0.0.1')" />
          </n-form-item>
          <n-form-item :label="$gettext('Port (port)')">
            <n-input-number
              class="w-full"
              v-model:value="port"
              :placeholder="$gettext('e.g. 6379')"
              :min="1"
              :max="65535"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Databases (databases)')">
            <n-input-number
              class="w-full"
              v-model:value="databases"
              :placeholder="$gettext('e.g. 16')"
              :min="1"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Password (requirepass)')">
            <n-input
              v-model:value="requirepass"
              type="password"
              show-password-on="click"
              :placeholder="$gettext('Leave empty for no password')"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Timeout (timeout)')">
            <n-input-number
              class="w-full"
              v-model:value="timeout"
              :placeholder="$gettext('e.g. 0 (disabled) or seconds')"
              :min="0"
            />
          </n-form-item>
          <n-form-item :label="$gettext('TCP Keepalive (tcp-keepalive)')">
            <n-input-number
              class="w-full"
              v-model:value="tcpKeepalive"
              :placeholder="$gettext('e.g. 300')"
              :min="0"
            />
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
    <n-tab-pane name="memory" :tab="$gettext('Memory')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Valkey memory management settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Max Memory (maxmemory)')">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="maxmemoryNum"
                :placeholder="$gettext('e.g. 256')"
                :min="0"
                style="flex: 1"
              />
              <n-select v-model:value="maxmemoryUnit" :options="sizeUnitOptions" class="w-20" />
            </n-input-group>
          </n-form-item>
          <n-form-item :label="$gettext('Maxmemory Policy (maxmemory-policy)')">
            <n-select v-model:value="maxmemoryPolicy" :options="maxmemoryPolicyOptions" clearable />
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
    <n-tab-pane name="persistence" :tab="$gettext('Persistence')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Valkey AOF persistence settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Append Only (appendonly)')">
            <n-select v-model:value="appendonly" :options="yesNoOptions" clearable />
          </n-form-item>
          <n-form-item :label="$gettext('Append Fsync (appendfsync)')">
            <n-select v-model:value="appendfsync" :options="appendfsyncOptions" clearable />
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

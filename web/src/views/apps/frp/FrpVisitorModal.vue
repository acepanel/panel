<script setup lang="ts">
defineOptions({
  name: 'frp-visitor-modal',
})

import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { required: true })
const props = defineProps<{
  // 传入访问者表示编辑，为空表示新增
  visitor?: any
}>()
const emit = defineEmits<{ submitted: [] }>()

const defaultModel = () => ({
  name: '',
  type: 'stcp',
  enabled: true,
  server_user: '',
  server_name: '',
  secret_key: '',
  bind_addr: '127.0.0.1',
  bind_port: null as number | null,
  protocol: '',
  keep_tunnel_open: false,
  max_retries_an_hour: null as number | null,
  min_retry_interval: null as number | null,
  fallback_to: '',
  fallback_timeout_ms: null as number | null,
  transport: { use_encryption: false, use_compression: false },
})

const model = ref(defaultModel())

const isEdit = computed(() => !!props.visitor)
const isXTCP = computed(() => model.value.type === 'xtcp')

const typeOptions = ['stcp', 'sudp', 'xtcp'].map((item) => ({ label: item, value: item }))

const protocolOptions = ['quic', 'kcp'].map((item) => ({ label: item, value: item }))

const fill = (visitor: any) => {
  const base = defaultModel()
  return {
    ...base,
    ...visitor,
    enabled: visitor.enabled !== false,
    transport: { ...base.transport, ...visitor.transport },
  }
}

watch(show, (val) => {
  if (val) {
    model.value = props.visitor ? fill(props.visitor) : defaultModel()
  }
})

// 只提交当前类型用得上的字段，避免写出 frp 不认识的配置
const buildPayload = () => {
  const m = model.value
  const payload: any = {
    name: m.name,
    type: m.type,
    enabled: m.enabled,
    server_user: m.server_user,
    server_name: m.server_name,
    secret_key: m.secret_key,
    bind_addr: m.bind_addr,
    bind_port: m.bind_port ?? 0,
  }

  if (m.transport.use_encryption || m.transport.use_compression) {
    payload.transport = m.transport
  }

  if (isXTCP.value) {
    payload.protocol = m.protocol
    payload.keep_tunnel_open = m.keep_tunnel_open
    payload.max_retries_an_hour = m.max_retries_an_hour ?? 0
    payload.min_retry_interval = m.min_retry_interval ?? 0
    payload.fallback_to = m.fallback_to
    payload.fallback_timeout_ms = m.fallback_timeout_ms ?? 0
  }

  return payload
}

const handleSubmit = async () => {
  const payload = buildPayload()
  await (isEdit.value ? frp.updateVisitor(payload.name, payload) : frp.addVisitor(payload))
  show.value = false
  emit('submitted')
  window.$message.success($gettext('Saved successfully'))
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="isEdit ? $gettext('Edit Visitor') : $gettext('Add Visitor')"
  >
    <n-form :model="model">
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="model.name"
          :disabled="isEdit"
          @keydown.enter.prevent
          :placeholder="$gettext('Unique name, letters, digits, dot, underscore and hyphen only')"
        />
      </n-form-item>
      <n-form-item path="type" :label="$gettext('Type')">
        <n-select v-model:value="model.type" :options="typeOptions" />
      </n-form-item>
      <n-form-item path="enabled" :label="$gettext('Enabled')">
        <n-switch v-model:value="model.enabled" />
      </n-form-item>
      <n-form-item path="server_name" :label="$gettext('Target Proxy (serverName)')">
        <n-input
          v-model:value="model.server_name"
          @keydown.enter.prevent
          :placeholder="$gettext('Name of the proxy to visit')"
        />
      </n-form-item>
      <n-form-item path="server_user" :label="$gettext('Target User (serverUser)')">
        <n-input
          v-model:value="model.server_user"
          @keydown.enter.prevent
          :placeholder="$gettext('Defaults to the current user')"
        />
      </n-form-item>
      <n-form-item path="secret_key" :label="$gettext('Secret Key (secretKey)')">
        <n-input
          v-model:value="model.secret_key"
          type="password"
          show-password-on="click"
          :placeholder="$gettext('Must match the target proxy')"
        />
      </n-form-item>
      <n-form-item path="bind_addr" :label="$gettext('Bind Address (bindAddr)')">
        <n-input v-model:value="model.bind_addr" placeholder="127.0.0.1" @keydown.enter.prevent />
      </n-form-item>
      <n-form-item path="bind_port" :label="$gettext('Bind Port (bindPort)')">
        <n-input-number
          class="w-full"
          v-model:value="model.bind_port"
          :min="-1"
          :max="65535"
          :placeholder="$gettext('-1 means do not bind, only accept redirected connections')"
        />
      </n-form-item>

      <template v-if="isXTCP">
        <n-form-item path="protocol" :label="$gettext('Protocol (protocol)')">
          <n-select v-model:value="model.protocol" :options="protocolOptions" clearable />
        </n-form-item>
        <n-form-item path="keep_tunnel_open" :label="$gettext('Keep Tunnel Open (keepTunnelOpen)')">
          <n-switch v-model:value="model.keep_tunnel_open" />
        </n-form-item>
        <template v-if="model.keep_tunnel_open">
          <n-form-item
            path="max_retries_an_hour"
            :label="$gettext('Max Retries Per Hour (maxRetriesAnHour)')"
          >
            <n-input-number
              class="w-full"
              v-model:value="model.max_retries_an_hour"
              :min="1"
              :placeholder="$gettext('e.g. 8')"
            />
          </n-form-item>
          <n-form-item
            path="min_retry_interval"
            :label="$gettext('Min Retry Interval (minRetryInterval)')"
          >
            <n-input-number
              class="w-full"
              v-model:value="model.min_retry_interval"
              :min="1"
              :placeholder="$gettext('e.g. 90')"
            />
          </n-form-item>
        </template>
        <n-form-item path="fallback_to" :label="$gettext('Fallback To (fallbackTo)')">
          <n-input
            v-model:value="model.fallback_to"
            @keydown.enter.prevent
            :placeholder="$gettext('Name of the visitor to fall back to')"
          />
        </n-form-item>
        <n-form-item
          path="fallback_timeout_ms"
          :label="$gettext('Fallback Timeout (fallbackTimeoutMs)')"
        >
          <n-input-number
            class="w-full"
            v-model:value="model.fallback_timeout_ms"
            :min="1"
            :placeholder="$gettext('e.g. 500')"
          />
        </n-form-item>
      </template>

      <n-collapse>
        <n-collapse-item :title="$gettext('Transport')" name="transport">
          <n-form-item :label="$gettext('Encryption (transport.useEncryption)')">
            <n-switch v-model:value="model.transport.use_encryption" />
          </n-form-item>
          <n-form-item :label="$gettext('Compression (transport.useCompression)')">
            <n-switch v-model:value="model.transport.use_compression" />
          </n-form-item>
        </n-collapse-item>
      </n-collapse>
    </n-form>
    <app-button type="info" mt-16 block @click="handleSubmit">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>

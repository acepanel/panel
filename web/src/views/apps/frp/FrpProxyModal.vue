<script setup lang="ts">
defineOptions({
  name: 'frp-proxy-modal',
})

import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { required: true })
const props = defineProps<{
  // 传入代理表示编辑，为空表示新增
  proxy?: any
}>()
const emit = defineEmits<{ submitted: [] }>()

const defaultModel = () => ({
  name: '',
  type: 'tcp',
  enabled: true,
  local_ip: '127.0.0.1',
  local_port: null as number | null,
  remote_port: null as number | null,
  custom_domains: [] as string[],
  subdomain: '',
  http_user: '',
  http_password: '',
  route_by_http_user: '',
  locations: [] as string[],
  host_header_rewrite: '',
  request_headers: {} as Record<string, string>,
  response_headers: {} as Record<string, string>,
  multiplexer: 'httpconnect',
  secret_key: '',
  allow_users: [] as string[],
  transport: {
    use_encryption: false,
    use_compression: false,
    bandwidth_limit: '',
    bandwidth_limit_mode: '',
    proxy_protocol_version: '',
  },
  load_balancer: { group: '', group_key: '' },
  health_check: {
    type: '',
    timeout_seconds: null as number | null,
    max_failed: null as number | null,
    interval_seconds: null as number | null,
    path: '',
  },
  plugin: {
    type: '',
    unix_path: '',
    http_user: '',
    http_password: '',
    username: '',
    password: '',
    local_path: '',
    strip_prefix: '',
    local_addr: '',
    host_header_rewrite: '',
    crt_path: '',
    key_path: '',
  },
  metadatas: {} as Record<string, string>,
  annotations: {} as Record<string, string>,
})

const model = ref(defaultModel())

const isEdit = computed(() => !!props.proxy)
// 使用插件时 localIP 与 localPort 会被忽略
const usePlugin = computed(() => !!model.value.plugin.type)
const isPortForward = computed(() => ['tcp', 'udp'].includes(model.value.type))
const isVhost = computed(() => ['http', 'https', 'tcpmux'].includes(model.value.type))
const isHTTPAuth = computed(() => ['http', 'tcpmux'].includes(model.value.type))
const isSecret = computed(() => ['stcp', 'sudp', 'xtcp'].includes(model.value.type))
const canLoadBalance = computed(() => ['tcp', 'http'].includes(model.value.type))

const typeOptions = ['tcp', 'udp', 'http', 'https', 'tcpmux', 'stcp', 'sudp', 'xtcp'].map(
  (item) => ({ label: item, value: item }),
)

const bandwidthModeOptions = [
  { label: $gettext('Client'), value: 'client' },
  { label: $gettext('Server'), value: 'server' },
]

const proxyProtocolOptions = [
  { label: 'v1', value: 'v1' },
  { label: 'v2', value: 'v2' },
]

const healthCheckOptions = [
  { label: 'tcp', value: 'tcp' },
  { label: 'http', value: 'http' },
]

const pluginOptions = [
  'unix_domain_socket',
  'http_proxy',
  'socks5',
  'static_file',
  'https2http',
  'https2https',
  'http2https',
  'http2http',
  'tls2raw',
].map((item) => ({ label: item, value: item }))

// 插件类型到所需字段的映射
const pluginFields: Record<string, string[]> = {
  unix_domain_socket: ['unix_path'],
  http_proxy: ['http_user', 'http_password'],
  socks5: ['username', 'password'],
  static_file: ['local_path', 'strip_prefix', 'http_user', 'http_password'],
  https2http: ['local_addr', 'host_header_rewrite', 'crt_path', 'key_path'],
  https2https: ['local_addr', 'host_header_rewrite', 'crt_path', 'key_path'],
  http2https: ['local_addr', 'host_header_rewrite'],
  http2http: ['local_addr', 'host_header_rewrite'],
  tls2raw: ['local_addr', 'crt_path', 'key_path'],
}

const hasPluginField = (field: string) => pluginFields[model.value.plugin.type]?.includes(field)

// 后端不返回的空值统一补成默认值，避免表单出现 undefined
const fill = (proxy: any) => {
  const base = defaultModel()
  return {
    ...base,
    ...proxy,
    enabled: proxy.enabled !== false,
    custom_domains: proxy.custom_domains ?? [],
    locations: proxy.locations ?? [],
    allow_users: proxy.allow_users ?? [],
    request_headers: proxy.request_headers?.set ?? {},
    response_headers: proxy.response_headers?.set ?? {},
    transport: { ...base.transport, ...proxy.transport },
    load_balancer: { ...base.load_balancer, ...proxy.load_balancer },
    health_check: { ...base.health_check, ...proxy.health_check },
    plugin: { ...base.plugin, ...proxy.plugin },
    metadatas: proxy.metadatas ?? {},
    annotations: proxy.annotations ?? {},
  }
}

watch(show, (val) => {
  if (val) {
    model.value = props.proxy ? fill(props.proxy) : defaultModel()
  }
})

// 判断嵌套对象是否有值，全为空时不提交，避免生成空的 TOML 子表
const filled = (obj: Record<string, any>) =>
  Object.values(obj).some((value) => value !== '' && value !== false && value != null)

// 只提交当前类型用得上的字段，避免写出 frp 不认识的配置
const buildPayload = () => {
  const m = model.value
  const payload: any = {
    name: m.name,
    type: m.type,
    enabled: m.enabled,
  }

  if (filled(m.transport)) {
    payload.transport = m.transport
  }
  if (Object.keys(m.metadatas).length) {
    payload.metadatas = m.metadatas
  }
  if (Object.keys(m.annotations).length) {
    payload.annotations = m.annotations
  }

  if (!usePlugin.value) {
    payload.local_ip = m.local_ip
    payload.local_port = m.local_port ?? 0
  } else {
    payload.plugin = m.plugin
  }

  if (isPortForward.value) {
    payload.remote_port = m.remote_port ?? 0
  }
  if (isVhost.value) {
    payload.custom_domains = m.custom_domains
    payload.subdomain = m.subdomain
  }
  if (isHTTPAuth.value) {
    payload.http_user = m.http_user
    payload.http_password = m.http_password
    payload.route_by_http_user = m.route_by_http_user
  }
  if (m.type === 'http') {
    payload.locations = m.locations
    payload.host_header_rewrite = m.host_header_rewrite
    if (Object.keys(m.request_headers).length) {
      payload.request_headers = { set: m.request_headers }
    }
    if (Object.keys(m.response_headers).length) {
      payload.response_headers = { set: m.response_headers }
    }
  }
  if (m.type === 'tcpmux') {
    payload.multiplexer = m.multiplexer
  }
  if (isSecret.value) {
    payload.secret_key = m.secret_key
    payload.allow_users = m.allow_users
  }
  if (canLoadBalance.value && m.load_balancer.group) {
    payload.load_balancer = m.load_balancer
  }
  if (m.health_check.type) {
    payload.health_check = m.health_check
  }

  return payload
}

const handleSubmit = async () => {
  const payload = buildPayload()
  await (isEdit.value ? frp.updateProxy(payload.name, payload) : frp.addProxy(payload))
  show.value = false
  emit('submitted')
  window.$message.success($gettext('Saved successfully'))
}
</script>

<template>
  <app-modal
    v-model:show="show"
    size="lg"
    :title="isEdit ? $gettext('Edit Proxy') : $gettext('Add Proxy')"
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

      <template v-if="!usePlugin">
        <n-form-item path="local_ip" :label="$gettext('Local Address (localIP)')">
          <n-input v-model:value="model.local_ip" placeholder="127.0.0.1" @keydown.enter.prevent />
        </n-form-item>
        <n-form-item path="local_port" :label="$gettext('Local Port (localPort)')">
          <n-input-number class="w-full" v-model:value="model.local_port" :min="1" :max="65535" />
        </n-form-item>
      </template>

      <n-form-item
        v-if="isPortForward"
        path="remote_port"
        :label="$gettext('Remote Port (remotePort)')"
      >
        <n-input-number
          class="w-full"
          v-model:value="model.remote_port"
          :min="0"
          :max="65535"
          :placeholder="$gettext('0 means frps assigns a random port')"
        />
      </n-form-item>

      <template v-if="isVhost">
        <n-form-item path="custom_domains" :label="$gettext('Custom Domains (customDomains)')">
          <n-dynamic-input
            v-model:value="model.custom_domains"
            placeholder="web01.yourdomain.com"
            show-sort-button
          />
        </n-form-item>
        <n-form-item path="subdomain" :label="$gettext('Subdomain (subdomain)')">
          <n-input
            v-model:value="model.subdomain"
            @keydown.enter.prevent
            :placeholder="$gettext('Requires subDomainHost on frps')"
          />
        </n-form-item>
      </template>

      <template v-if="isHTTPAuth">
        <n-form-item path="http_user" :label="$gettext('HTTP Username (httpUser)')">
          <n-input v-model:value="model.http_user" @keydown.enter.prevent />
        </n-form-item>
        <n-form-item path="http_password" :label="$gettext('HTTP Password (httpPassword)')">
          <n-input v-model:value="model.http_password" type="password" show-password-on="click" />
        </n-form-item>
        <n-form-item path="route_by_http_user" :label="$gettext('Route By User (routeByHTTPUser)')">
          <n-input v-model:value="model.route_by_http_user" @keydown.enter.prevent />
        </n-form-item>
      </template>

      <template v-if="model.type === 'http'">
        <n-form-item path="locations" :label="$gettext('Locations (locations)')">
          <n-dynamic-input v-model:value="model.locations" placeholder="/" show-sort-button />
        </n-form-item>
        <n-form-item
          path="host_header_rewrite"
          :label="$gettext('Host Header Rewrite (hostHeaderRewrite)')"
        >
          <n-input
            v-model:value="model.host_header_rewrite"
            placeholder="example.com"
            @keydown.enter.prevent
          />
        </n-form-item>
        <n-form-item path="request_headers" :label="$gettext('Request Headers (requestHeaders)')">
          <key-value-editor
            v-model="model.request_headers"
            :key-placeholder="$gettext('Header')"
            :value-placeholder="$gettext('Value')"
            :add-button-text="$gettext('Add Header')"
            default-key-prefix="x-from-where"
            separator=":"
          />
        </n-form-item>
        <n-form-item
          path="response_headers"
          :label="$gettext('Response Headers (responseHeaders)')"
        >
          <key-value-editor
            v-model="model.response_headers"
            :key-placeholder="$gettext('Header')"
            :value-placeholder="$gettext('Value')"
            :add-button-text="$gettext('Add Header')"
            default-key-prefix="x-from-where"
            separator=":"
          />
        </n-form-item>
      </template>

      <n-form-item
        v-if="model.type === 'tcpmux'"
        path="multiplexer"
        :label="$gettext('Multiplexer (multiplexer)')"
      >
        <n-input v-model:value="model.multiplexer" placeholder="httpconnect" />
      </n-form-item>

      <template v-if="isSecret">
        <n-form-item path="secret_key" :label="$gettext('Secret Key (secretKey)')">
          <n-input
            v-model:value="model.secret_key"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Visitors must use the same secret key')"
          />
        </n-form-item>
        <n-form-item path="allow_users" :label="$gettext('Allowed Users (allowUsers)')">
          <n-dynamic-input
            v-model:value="model.allow_users"
            :placeholder="$gettext('* means all users')"
            show-sort-button
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
          <n-form-item :label="$gettext('Bandwidth Limit (transport.bandwidthLimit)')">
            <n-input
              v-model:value="model.transport.bandwidth_limit"
              placeholder="1MB"
              @keydown.enter.prevent
            />
          </n-form-item>
          <n-form-item :label="$gettext('Limit Side (transport.bandwidthLimitMode)')">
            <n-select
              v-model:value="model.transport.bandwidth_limit_mode"
              :options="bandwidthModeOptions"
              clearable
            />
          </n-form-item>
          <n-form-item
            v-if="['tcp', 'https'].includes(model.type)"
            :label="$gettext('Proxy Protocol (transport.proxyProtocolVersion)')"
          >
            <n-select
              v-model:value="model.transport.proxy_protocol_version"
              :options="proxyProtocolOptions"
              clearable
            />
          </n-form-item>
        </n-collapse-item>
        <n-collapse-item
          v-if="canLoadBalance"
          :title="$gettext('Load Balancer')"
          name="load-balancer"
        >
          <n-form-item :label="$gettext('Group (loadBalancer.group)')">
            <n-input v-model:value="model.load_balancer.group" @keydown.enter.prevent />
          </n-form-item>
          <n-form-item :label="$gettext('Group Key (loadBalancer.groupKey)')">
            <n-input v-model:value="model.load_balancer.group_key" @keydown.enter.prevent />
          </n-form-item>
        </n-collapse-item>
        <n-collapse-item :title="$gettext('Health Check')" name="health-check">
          <n-form-item :label="$gettext('Type (healthCheck.type)')">
            <n-select
              v-model:value="model.health_check.type"
              :options="healthCheckOptions"
              clearable
            />
          </n-form-item>
          <template v-if="model.health_check.type">
            <n-form-item :label="$gettext('Timeout Seconds (healthCheck.timeoutSeconds)')">
              <n-input-number
                class="w-full"
                v-model:value="model.health_check.timeout_seconds"
                :min="1"
                :placeholder="$gettext('e.g. 3')"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Max Failed (healthCheck.maxFailed)')">
              <n-input-number
                class="w-full"
                v-model:value="model.health_check.max_failed"
                :min="1"
                :placeholder="$gettext('e.g. 3')"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Interval Seconds (healthCheck.intervalSeconds)')">
              <n-input-number
                class="w-full"
                v-model:value="model.health_check.interval_seconds"
                :min="1"
                :placeholder="$gettext('e.g. 10')"
              />
            </n-form-item>
            <n-form-item
              v-if="model.health_check.type === 'http'"
              :label="$gettext('Path (healthCheck.path)')"
            >
              <n-input
                v-model:value="model.health_check.path"
                placeholder="/status"
                @keydown.enter.prevent
              />
            </n-form-item>
          </template>
        </n-collapse-item>
        <n-collapse-item :title="$gettext('Plugin')" name="plugin">
          <n-alert type="info" mb-16>
            {{ $gettext('When a plugin is used, the local address and port are ignored.') }}
          </n-alert>
          <n-form-item :label="$gettext('Type (plugin.type)')">
            <n-select v-model:value="model.plugin.type" :options="pluginOptions" clearable />
          </n-form-item>
          <n-form-item
            v-if="hasPluginField('unix_path')"
            :label="$gettext('Socket Path (unixPath)')"
          >
            <n-input
              v-model:value="model.plugin.unix_path"
              placeholder="/var/run/docker.sock"
              @keydown.enter.prevent
            />
          </n-form-item>
          <n-form-item
            v-if="hasPluginField('local_addr')"
            :label="$gettext('Local Address (localAddr)')"
          >
            <n-input
              v-model:value="model.plugin.local_addr"
              placeholder="127.0.0.1:80"
              @keydown.enter.prevent
            />
          </n-form-item>
          <n-form-item
            v-if="hasPluginField('local_path')"
            :label="$gettext('Local Path (localPath)')"
          >
            <n-input
              v-model:value="model.plugin.local_path"
              placeholder="/var/www/blog"
              @keydown.enter.prevent
            />
          </n-form-item>
          <n-form-item
            v-if="hasPluginField('strip_prefix')"
            :label="$gettext('Strip Prefix (stripPrefix)')"
          >
            <n-input v-model:value="model.plugin.strip_prefix" @keydown.enter.prevent />
          </n-form-item>
          <n-form-item
            v-if="hasPluginField('host_header_rewrite')"
            :label="$gettext('Host Header Rewrite (hostHeaderRewrite)')"
          >
            <n-input v-model:value="model.plugin.host_header_rewrite" @keydown.enter.prevent />
          </n-form-item>
          <n-form-item v-if="hasPluginField('crt_path')" :label="$gettext('Certificate (crtPath)')">
            <n-input v-model:value="model.plugin.crt_path" @keydown.enter.prevent />
          </n-form-item>
          <n-form-item v-if="hasPluginField('key_path')" :label="$gettext('Private Key (keyPath)')">
            <n-input v-model:value="model.plugin.key_path" @keydown.enter.prevent />
          </n-form-item>
          <n-form-item v-if="hasPluginField('http_user')" :label="$gettext('Username (httpUser)')">
            <n-input v-model:value="model.plugin.http_user" @keydown.enter.prevent />
          </n-form-item>
          <n-form-item
            v-if="hasPluginField('http_password')"
            :label="$gettext('Password (httpPassword)')"
          >
            <n-input
              v-model:value="model.plugin.http_password"
              type="password"
              show-password-on="click"
            />
          </n-form-item>
          <n-form-item v-if="hasPluginField('username')" :label="$gettext('Username (username)')">
            <n-input v-model:value="model.plugin.username" @keydown.enter.prevent />
          </n-form-item>
          <n-form-item v-if="hasPluginField('password')" :label="$gettext('Password (password)')">
            <n-input
              v-model:value="model.plugin.password"
              type="password"
              show-password-on="click"
            />
          </n-form-item>
        </n-collapse-item>
        <n-collapse-item :title="$gettext('Metadata')" name="metadata">
          <n-form-item :label="$gettext('Metadatas (metadatas)')">
            <key-value-editor
              v-model="model.metadatas"
              :add-button-text="$gettext('Add Metadata')"
              default-key-prefix="var"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Annotations (annotations)')">
            <key-value-editor
              v-model="model.annotations"
              :add-button-text="$gettext('Add Annotation')"
              default-key-prefix="key"
            />
          </n-form-item>
        </n-collapse-item>
      </n-collapse>
    </n-form>
    <app-button type="info" mt-16 block @click="handleSubmit">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>

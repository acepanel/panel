<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import { useFormLayout } from '@/components/system/composables/useFormLayout'
import type { WebServerFeatures } from '@/utils'

import DurationInput from './DurationInput.vue'

type SizeUnit = 'k' | 'm' | 'g'

// Go time.Duration 在 JSON 中以纳秒表示
const SECOND = 1000000000

const sizeFactors: Record<SizeUnit, number> = { k: 1024, m: 1024 ** 2, g: 1024 ** 3 }

const proxy = defineModel<any>('proxy', { required: true })
const props = defineProps<{
  index: number
  features: WebServerFeatures
  upstreamNames: string[]
}>()
const emit = defineEmits<{ remove: [] }>()

const { $gettext } = useGettext()
const formLayout = useFormLayout(140)

// Location 匹配类型选项
const locationMatchTypes = [
  { label: $gettext('Exact Match (=)'), value: '=' },
  { label: $gettext('Priority Prefix Match (^~)'), value: '^~' },
  { label: $gettext('Prefix Match'), value: '' },
  { label: $gettext('Case-sensitive Regex (~)'), value: '~' },
  { label: $gettext('Case-insensitive Regex (~*)'), value: '~*' },
]

// 解析 location 字符串，返回匹配类型和表达式
const parseLocation = (location: string): { type: string; expression: string } => {
  if (!location) return { type: '', expression: '/' }
  // 精确匹配 =
  if (location.startsWith('= ')) {
    return { type: '=', expression: location.slice(2) }
  }
  // 优先前缀匹配 ^~
  if (location.startsWith('^~ ')) {
    return { type: '^~', expression: location.slice(3) }
  }
  // 不区分大小写正则 ~*
  if (location.startsWith('~* ')) {
    return { type: '~*', expression: location.slice(3) }
  }
  // 区分大小写正则 ~
  if (location.startsWith('~ ')) {
    return { type: '~', expression: location.slice(2) }
  }
  // 普通前缀匹配
  return { type: '', expression: location }
}

// 组合 location 字符串
const buildLocation = (type: string, expression: string): string => {
  if (!expression) expression = '/'
  if (type === '') return expression
  return `${type} ${expression}`
}

const parsedLocation = computed(() => parseLocation(proxy.value.location))

const updateLocationType = (type: string) => {
  proxy.value.location = buildLocation(type, parsedLocation.value.expression)
}

const updateLocationExpression = (expression: string) => {
  proxy.value.location = buildLocation(parsedLocation.value.type, expression)
}

// 代理目标的默认 Host，与后端 DefaultProxyHost 同一规则：本机、IP、unix 套接字与上游发访客域名，外部域名发目标域名
const defaultProxyHost = (pass: string): string => {
  let hostname: string
  try {
    hostname = new URL(pass).hostname
  } catch {
    return '$host'
  }
  const local =
    !hostname.includes('.') ||
    hostname.startsWith('[') ||
    /^\d+(\.\d+){3}$/.test(hostname) ||
    props.upstreamNames.some((name) => name.toLowerCase() === hostname)
  return local ? '$host' : '$proxy_host'
}

const toUpstream = computed(() => {
  try {
    const hostname = new URL(proxy.value.pass).hostname
    return props.upstreamNames.some((name) => name.toLowerCase() === hostname)
  } catch {
    return false
  }
})

// 目标是上游时，有的服务器的负载均衡只会原样转发，Host 与请求头都改不了
const requestRewritable = computed(() => props.features.upstreamRequestRewrite || !toUpstream.value)

// 有的服务器的超时与缓冲是跟着上游成员走的，目标是上游时没法按规则设
const tunable = computed(() => props.features.upstreamTuning || !toUpstream.value)

// Proxy Pass 变化时自动更新 Host
const handlePassChange = (value: string) => {
  proxy.value.pass = value
  proxy.value.host = defaultProxyHost(value)
}

const httpVersionOptions = computed(() =>
  props.features.proxyHTTPVersions.map((value) => ({ label: `HTTP/${value}`, value })),
)

// 高级配置里当前服务器一项都不支持时整块不显示
const showAdvanced = computed(
  () =>
    props.features.proxyHTTPVersions.length > 0 ||
    props.features.proxyBodySize ||
    props.features.proxyTimeouts.length > 0 ||
    props.features.proxyRetry.length > 0 ||
    props.features.proxySSLVerify ||
    props.features.proxyResponseHeaders ||
    props.features.proxyAccessControl,
)

const timeoutDefault = (key: string) =>
  props.features.proxyTimeouts.includes(key) ? 60 * SECOND : 0

// 可选配置块启用时的默认值，关闭时置为 null；当前服务器不支持的项留空
const blockDefaults = {
  cache: () => ({
    valid: ttlOnly.value ? { [ttlCodes]: '10m' } : { '200 302': '10m', '404': '10s' },
    no_cache_conditions: [],
    use_stale: [],
    background_update: false,
    lock: false,
    min_uses: 0,
    methods: [],
    key: '',
  }),
  timeout: () => ({
    connect: timeoutDefault('connect'),
    read: timeoutDefault('read'),
    send: timeoutDefault('send'),
  }),
  retry: () => ({
    conditions: props.features.proxyRetry.includes('conditions') ? ['error', 'timeout'] : [],
    tries: 0,
    timeout: 0,
  }),
  ssl_backend: () => ({
    verify: false,
    trusted_certificate: '',
    verify_depth: props.features.proxySSLVerifyDepth ? 1 : 0,
  }),
  response_headers: () => ({ hide: [], add: {} }),
  access_control: () => ({ allow: [], deny: [] }),
}

const toggleBlock = (key: keyof typeof blockDefaults, enabled: boolean) => {
  proxy.value[key] = enabled ? blockDefaults[key]() : null
}

const cacheEnabled = computed(() => proxy.value.cache !== null && proxy.value.cache !== undefined)

// 只有一个有效期的服务器，后端把它记在默认可缓存的状态码名下
const ttlCodes = '200 301'
const ttlOnly = computed(() => props.features.proxyCache === 'ttl')
const cacheTTL = computed({
  get: () => proxy.value.cache.valid[ttlCodes] ?? '',
  set: (v: string) => (proxy.value.cache.valid = v ? { [ttlCodes]: v } : {}),
})

// 请求体大小以字节为权威值，单位只影响显示，初始取能整除的最大单位
const inferSizeUnit = (bytes: number): SizeUnit => {
  if (bytes > 0) {
    if (bytes % sizeFactors.g === 0) return 'g'
    if (bytes % sizeFactors.m === 0) return 'm'
    if (bytes % sizeFactors.k === 0) return 'k'
  }
  return 'm'
}
const sizeUnit = ref(inferSizeUnit(proxy.value.client_max_body_size))

const bodySize = computed(() =>
  proxy.value.client_max_body_size
    ? proxy.value.client_max_body_size / sizeFactors[sizeUnit.value]
    : null,
)

const updateBodySize = (v: number | null) => {
  proxy.value.client_max_body_size = v && v > 0 ? v * sizeFactors[sizeUnit.value] : 0
}
</script>

<template>
  <n-card closable @close="emit('remove')" mb-4>
    <template #header>
      <n-flex align="center" :size="8">
        <!-- 拖拽手柄 -->
        <div class="drag-handle" cursor-grab>
          <the-icon icon="mdi:drag" :size="20" />
        </div>
        <span>{{ $gettext('Rule') }} #{{ index + 1 }}</span>
        <n-tag size="small">{{ proxy.location }}</n-tag>
        <the-icon icon="mdi:arrow-right-bold" :size="20" />
        <n-tag size="small" type="success">{{ proxy.pass }}</n-tag>
      </n-flex>
    </template>
    <n-form v-bind="formLayout">
      <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
        <n-form-item-gi v-if="features.matchType" :span="12" :label="$gettext('Match Type')">
          <n-select
            :value="parsedLocation.type"
            :options="locationMatchTypes"
            @update:value="updateLocationType"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="features.matchType ? 12 : 24" :label="$gettext('Match Expression')">
          <n-input
            :value="parsedLocation.expression"
            :placeholder="$gettext('e.g., /, /api, ^/api/v[0-9]+/')"
            @update:value="updateLocationExpression"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="12" :label="$gettext('Proxy Pass')">
          <n-input
            :value="proxy.pass"
            :placeholder="
              $gettext('Backend address, e.g., http://127.0.0.1:8080 or http://upstream_name')
            "
            @update:value="handlePassChange"
          />
        </n-form-item-gi>
        <n-form-item-gi v-if="requestRewritable" :span="12" :label="$gettext('Proxy Host')">
          <n-input
            v-model:value="proxy.host"
            :placeholder="
              $gettext(
                '$host keeps the visitor Host, $proxy_host sends the upstream hostname; empty means $proxy_host',
              )
            "
          />
        </n-form-item-gi>
        <n-form-item-gi v-if="features.proxySNI" :span="12" :label="$gettext('Proxy SNI')">
          <n-input
            v-model:value="proxy.sni"
            :placeholder="$gettext('Optional, for HTTPS backends')"
          />
        </n-form-item-gi>
        <n-form-item-gi v-if="features.proxyCache" :span="6" :label="$gettext('Enable Cache')">
          <n-switch :value="cacheEnabled" @update:value="(v: boolean) => toggleBlock('cache', v)" />
        </n-form-item-gi>
        <n-form-item-gi v-if="tunable" :span="6" :label="$gettext('Enable Buffering')">
          <n-switch v-model:value="proxy.buffering" />
        </n-form-item-gi>
        <n-form-item-gi v-if="features.resolver" :span="12" :label="$gettext('DNS Resolver')">
          <n-dynamic-tags v-model:value="proxy.resolver" :placeholder="$gettext('e.g., 8.8.8.8')" />
        </n-form-item-gi>
        <n-form-item-gi
          v-if="features.resolver && proxy.resolver.length"
          :span="12"
          :label="$gettext('Resolver Timeout')"
        >
          <duration-input v-model:value="proxy.resolver_timeout" :min="1" :max="3600" />
        </n-form-item-gi>
      </n-grid>
      <!-- 可折叠配置区域 -->
      <n-collapse :default-expanded-names="[]" mt-4>
        <!-- 缓存配置详情 -->
        <n-collapse-item
          v-if="features.proxyCache && cacheEnabled"
          :title="$gettext('Cache Settings')"
          name="cache"
        >
          <n-grid v-if="ttlOnly" cols="1 m:24" :x-gap="16" responsive="screen">
            <n-form-item-gi :span="24" :label="$gettext('Cache Valid')">
              <n-input
                v-model:value="cacheTTL"
                :placeholder="$gettext('Duration, e.g., 10m, 1h, 1d')"
              />
              <template #feedback>
                {{
                  $gettext(
                    'Applies when the backend sets no expiration; URLs with a query string are cached only if the backend sets one',
                  )
                }}
              </template>
            </n-form-item-gi>
          </n-grid>
          <n-grid v-else cols="1 m:24" :x-gap="16" responsive="screen">
            <!-- 缓存有效期 -->
            <n-form-item-gi :span="24" :label="$gettext('Cache Valid')">
              <key-value-editor
                v-model="proxy.cache.valid"
                :key-placeholder="$gettext('Status codes, e.g., 200 302 or any')"
                :value-placeholder="$gettext('Duration, e.g., 10m, 1h, 1d')"
                :add-button-text="$gettext('Add Cache Valid Rule')"
                default-key-prefix="20"
                default-value="10m"
              />
            </n-form-item-gi>
            <!-- 不缓存条件 -->
            <n-form-item-gi :span="12" :label="$gettext('No Cache Conditions')">
              <n-select
                v-model:value="proxy.cache.no_cache_conditions"
                :options="[
                  { label: '$cookie_nocache', value: '$cookie_nocache' },
                  { label: '$arg_nocache', value: '$arg_nocache' },
                  { label: '$http_pragma', value: '$http_pragma' },
                  { label: '$http_authorization', value: '$http_authorization' },
                  { label: '$http_cache_control', value: '$http_cache_control' },
                ]"
                multiple
                filterable
                tag
                :placeholder="$gettext('Select or enter conditions')"
              />
            </n-form-item-gi>
            <!-- 过期缓存使用策略 -->
            <n-form-item-gi :span="12" :label="$gettext('Use Stale')">
              <n-select
                v-model:value="proxy.cache.use_stale"
                :options="[
                  { label: 'error', value: 'error' },
                  { label: 'timeout', value: 'timeout' },
                  { label: 'updating', value: 'updating' },
                  { label: 'http_500', value: 'http_500' },
                  { label: 'http_502', value: 'http_502' },
                  { label: 'http_503', value: 'http_503' },
                  { label: 'http_504', value: 'http_504' },
                ]"
                multiple
                :placeholder="$gettext('When to use stale cache')"
              />
            </n-form-item-gi>
            <!-- 后台更新 -->
            <n-form-item-gi :span="6" :label="$gettext('Background Update')">
              <n-switch v-model:value="proxy.cache.background_update" />
            </n-form-item-gi>
            <!-- 缓存锁 -->
            <n-form-item-gi :span="6" :label="$gettext('Cache Lock')">
              <n-switch v-model:value="proxy.cache.lock" />
            </n-form-item-gi>
            <!-- 最小请求次数 -->
            <n-form-item-gi :span="6" :label="$gettext('Min Uses')">
              <n-input-number
                :value="proxy.cache.min_uses || null"
                :min="0"
                :max="100"
                w-full
                :placeholder="$gettext('Default')"
                @update:value="(v: number | null) => (proxy.cache.min_uses = v ?? 0)"
              />
            </n-form-item-gi>
            <!-- 缓存方法 -->
            <n-form-item-gi :span="6" :label="$gettext('Cache Methods')">
              <n-select
                v-model:value="proxy.cache.methods"
                :options="[
                  { label: 'GET', value: 'GET' },
                  { label: 'HEAD', value: 'HEAD' },
                  { label: 'POST', value: 'POST' },
                ]"
                multiple
                :placeholder="$gettext('Default: GET HEAD')"
              />
            </n-form-item-gi>
            <!-- 自定义缓存键 -->
            <n-form-item-gi :span="24" :label="$gettext('Cache Key')">
              <n-input
                v-model:value="proxy.cache.key"
                :placeholder="$gettext('Custom cache key, e.g., $scheme$host$request_uri')"
              />
            </n-form-item-gi>
          </n-grid>
        </n-collapse-item>

        <!-- 自定义请求头 -->
        <n-collapse-item
          v-if="requestRewritable"
          :title="$gettext('Custom Request Headers')"
          name="headers"
        >
          <key-value-editor
            v-model="proxy.headers"
            :key-placeholder="$gettext('Header name')"
            :value-placeholder="
              features.proxyHeaderVars
                ? $gettext('Value or variable like $host, $remote_addr')
                : $gettext('Header value')
            "
            :add-button-text="$gettext('Add Request Header')"
            default-key-prefix="X-Custom-Header"
          />
        </n-collapse-item>

        <!-- 响应内容替换 -->
        <n-collapse-item
          v-if="features.proxyReplaces"
          :title="$gettext('Response Content Replacement')"
          name="replaces"
        >
          <key-value-editor
            v-model="proxy.replaces"
            :key-placeholder="$gettext('Original content')"
            :value-placeholder="$gettext('Replacement content')"
            :add-button-text="$gettext('Add Replacement Rule')"
            default-key-prefix="/old_"
            default-value="/new"
            separator="=>"
          />
        </n-collapse-item>

        <!-- 高级配置 -->
        <n-collapse-item v-if="showAdvanced" :title="$gettext('Advanced Settings')" name="advanced">
          <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
            <!-- HTTP 协议版本 -->
            <n-form-item-gi
              v-if="httpVersionOptions.length"
              :span="12"
              :label="$gettext('HTTP Version')"
            >
              <n-select
                v-model:value="proxy.http_version"
                :options="httpVersionOptions"
                :placeholder="$gettext('Select HTTP version')"
              />
            </n-form-item-gi>

            <!-- 请求体大小限制 -->
            <n-form-item-gi
              v-if="features.proxyBodySize"
              :span="12"
              :label="$gettext('Max Body Size')"
            >
              <n-input-group>
                <n-input-number
                  :value="bodySize"
                  :min="0"
                  flex-1
                  :placeholder="$gettext('Use global')"
                  @update:value="updateBodySize"
                />
                <n-select
                  v-model:value="sizeUnit"
                  :options="[
                    { label: 'KB', value: 'k' },
                    { label: 'MB', value: 'm' },
                    { label: 'GB', value: 'g' },
                  ]"
                  class="w-20"
                />
              </n-input-group>
            </n-form-item-gi>

            <!-- 超时设置开关 -->
            <n-form-item-gi
              v-if="features.proxyTimeouts.length && tunable"
              :span="12"
              :label="$gettext('Timeout Settings')"
            >
              <n-switch
                :value="proxy.timeout !== null"
                @update:value="(v: boolean) => toggleBlock('timeout', v)"
              />
            </n-form-item-gi>
          </n-grid>

          <!-- 超时配置详情 -->
          <template v-if="features.proxyTimeouts.length && tunable && proxy.timeout">
            <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
              <n-form-item-gi
                v-if="features.proxyTimeouts.includes('connect')"
                :span="12"
                :label="$gettext('Connect Timeout')"
              >
                <duration-input v-model:value="proxy.timeout.connect" :min="1" />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="features.proxyTimeouts.includes('read')"
                :span="12"
                :label="$gettext('Read Timeout')"
              >
                <duration-input v-model:value="proxy.timeout.read" :min="1" />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="features.proxyTimeouts.includes('send')"
                :span="12"
                :label="$gettext('Send Timeout')"
              >
                <duration-input v-model:value="proxy.timeout.send" :min="1" />
              </n-form-item-gi>
            </n-grid>
          </template>

          <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
            <!-- 重试配置开关 -->
            <n-form-item-gi
              v-if="features.proxyRetry.length"
              :span="8"
              :label="$gettext('Retry Settings')"
            >
              <n-switch
                :value="proxy.retry !== null"
                @update:value="(v: boolean) => toggleBlock('retry', v)"
              />
            </n-form-item-gi>

            <!-- SSL 后端验证开关（仅 https） -->
            <n-form-item-gi
              v-if="features.proxySSLVerify && proxy.pass?.startsWith('https')"
              :span="8"
              :label="$gettext('SSL Backend Verify')"
            >
              <n-switch
                :value="proxy.ssl_backend !== null"
                @update:value="(v: boolean) => toggleBlock('ssl_backend', v)"
              />
            </n-form-item-gi>

            <!-- 响应头修改开关 -->
            <n-form-item-gi
              v-if="features.proxyResponseHeaders"
              :span="8"
              :label="$gettext('Response Headers')"
            >
              <n-switch
                :value="proxy.response_headers !== null"
                @update:value="(v: boolean) => toggleBlock('response_headers', v)"
              />
            </n-form-item-gi>
          </n-grid>

          <!-- 重试配置详情 -->
          <template v-if="features.proxyRetry.length && proxy.retry">
            <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
              <n-form-item-gi
                v-if="features.proxyRetry.includes('conditions')"
                :span="12"
                :label="$gettext('Retry Conditions')"
              >
                <n-select
                  v-model:value="proxy.retry.conditions"
                  :options="[
                    { label: 'error', value: 'error' },
                    { label: 'timeout', value: 'timeout' },
                    { label: 'invalid_header', value: 'invalid_header' },
                    { label: 'http_500', value: 'http_500' },
                    { label: 'http_502', value: 'http_502' },
                    { label: 'http_503', value: 'http_503' },
                    { label: 'http_504', value: 'http_504' },
                    { label: 'http_429', value: 'http_429' },
                    { label: 'non_idempotent', value: 'non_idempotent' },
                    { label: 'off', value: 'off' },
                  ]"
                  multiple
                  :placeholder="$gettext('Select retry conditions')"
                />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="features.proxyRetry.includes('tries')"
                :span="12"
                :label="$gettext('Max Tries')"
              >
                <n-input-number
                  :value="proxy.retry.tries || null"
                  :min="0"
                  :placeholder="$gettext('Unlimited')"
                  @update:value="(v: number | null) => (proxy.retry.tries = v ?? 0)"
                />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="features.proxyRetry.includes('timeout')"
                :span="12"
                :label="$gettext('Retry Timeout')"
              >
                <duration-input
                  v-model:value="proxy.retry.timeout"
                  :min="0"
                  :placeholder="$gettext('Unlimited')"
                />
              </n-form-item-gi>
            </n-grid>
          </template>

          <!-- SSL 后端验证详情 -->
          <template
            v-if="features.proxySSLVerify && proxy.ssl_backend && proxy.pass?.startsWith('https')"
          >
            <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
              <n-form-item-gi :span="6" :label="$gettext('Enable Verify')">
                <n-switch v-model:value="proxy.ssl_backend.verify" />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="features.proxySSLVerifyDepth"
                :span="6"
                :label="$gettext('Verify Depth')"
              >
                <n-input-number v-model:value="proxy.ssl_backend.verify_depth" :min="1" :max="10" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Trusted Certificate')">
                <n-input
                  v-model:value="proxy.ssl_backend.trusted_certificate"
                  :placeholder="
                    $gettext('CA certificate path, e.g. /etc/ssl/certs/ca-certificates.crt')
                  "
                />
              </n-form-item-gi>
            </n-grid>
          </template>

          <!-- 响应头修改详情 -->
          <template v-if="features.proxyResponseHeaders && proxy.response_headers">
            <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
              <n-form-item-gi :span="12" :label="$gettext('Hide Headers')">
                <n-select
                  v-model:value="proxy.response_headers.hide"
                  :options="[
                    { label: 'X-Powered-By', value: 'X-Powered-By' },
                    { label: 'Server', value: 'Server' },
                    { label: 'X-AspNet-Version', value: 'X-AspNet-Version' },
                    { label: 'X-AspNetMvc-Version', value: 'X-AspNetMvc-Version' },
                    { label: 'X-Runtime', value: 'X-Runtime' },
                    { label: 'X-Version', value: 'X-Version' },
                  ]"
                  multiple
                  filterable
                  tag
                  :placeholder="$gettext('Select or input headers to hide')"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Add Headers')">
                <key-value-editor
                  v-model="proxy.response_headers.add"
                  :key-placeholder="$gettext('Header name')"
                  :value-placeholder="$gettext('Header value')"
                  :add-button-text="$gettext('Add Response Header')"
                  default-key-prefix="X-Custom-Header"
                />
                <template #feedback>
                  {{
                    $gettext(
                      'Custom headers replace the site-level HSTS and Alt-Svc headers for this location',
                    )
                  }}
                </template>
              </n-form-item-gi>
            </n-grid>
          </template>

          <n-grid v-if="features.proxyAccessControl" cols="1 m:24" :x-gap="16" responsive="screen">
            <!-- IP 访问控制开关 -->
            <n-form-item-gi :span="8" :label="$gettext('IP Access Control')">
              <n-switch
                :value="proxy.access_control !== null"
                @update:value="(v: boolean) => toggleBlock('access_control', v)"
              />
            </n-form-item-gi>
          </n-grid>

          <!-- IP 访问控制详情 -->
          <template v-if="features.proxyAccessControl && proxy.access_control">
            <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
              <n-form-item-gi :span="12" :label="$gettext('Allow IPs')">
                <n-dynamic-tags
                  v-model:value="proxy.access_control.allow"
                  :placeholder="$gettext('IP or CIDR, e.g. 192.168.1.0/24')"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Deny IPs')">
                <n-dynamic-tags
                  v-model:value="proxy.access_control.deny"
                  :placeholder="$gettext('IP or CIDR, e.g. all')"
                />
              </n-form-item-gi>
            </n-grid>
          </template>
        </n-collapse-item>

        <!-- 自定义指令 -->
        <n-collapse-item :title="$gettext('Custom Directives')" name="custom">
          <n-form-item :show-label="false">
            <common-editor v-model:value="proxy.custom" height="20vh" :lang="features.lang" />
            <template #feedback>
              {{
                $gettext(
                  'Written in the native syntax of the current web server and appended to the end of this rule; not migrated when switching web servers',
                )
              }}
            </template>
          </n-form-item>
        </n-collapse-item>
      </n-collapse>
    </n-form>
  </n-card>
</template>

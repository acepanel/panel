<script setup lang="ts">
import type { MessageReactive } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'
import draggable from 'vuedraggable'

import cert from '@/api/panel/cert'
import file from '@/api/panel/file'
import home from '@/api/panel/home'
import website from '@/api/panel/website'
import KeyValueEditor from '@/components/common/KeyValueEditor.vue'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const editId = defineModel<number>('editId', { type: Number, required: true })

const { $gettext } = useGettext()
let messageReactive: MessageReactive | null = null

const current = ref('listen')
const loading = ref(false)
const saveLoading = ref(false)
const resetLoading = ref(false)
const clearLogLoading = ref(false)
const id = ref(0)
const initialSetting = {
  id: 0,
  name: '',
  type: 'proxy',
  listens: [],
  domains: [],
  path: '',
  root: '',
  index: [],
  ssl: false,
  ssl_cert: '',
  ssl_key: '',
  hsts: false,
  ocsp: false,
  http_redirect: false,
  ssl_protocols: [],
  ssl_not_before: '',
  ssl_not_after: '',
  ssl_dns_names: [],
  ssl_issuer: '',
  ssl_ocsp_server: [],
  access_log: '',
  error_log: '',
  php: 0,
  rewrite: '',
  open_basedir: false,
  upstreams: [],
  proxies: [],
  redirects: [],
  rate_limit: null,
  real_ip: null,
  basic_auth: {},
  custom_configs: [],
}
const setting = ref<any>({ ...initialSetting })
const fetchSetting = () => {
  loading.value = true
  useRequest(website.config(id.value))
    .onSuccess(({ data }: any) => {
      setting.value = data
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch(show, (v) => {
  if (v) {
    id.value = editId.value
    current.value = 'listen'
    fetchSetting()
  }
})
const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    webserver: 'nginx',
    php: [
      {
        label: $gettext('Not used'),
        value: 0,
      },
    ],
    db: [
      {
        label: '',
        value: '',
      },
    ],
  },
})

// 是否为 Nginx
const isNginx = computed(() => installedEnvironment.value.webserver === 'nginx')
const certs = ref<any>([])
useRequest(cert.certs(1, 10000)).onSuccess(({ data }) => {
  certs.value = data.items
})
const { data: rewrites } = useRequest(website.rewrites, {
  initialData: {},
})
const rewriteOptions = computed(() => {
  return Object.keys(rewrites.value).map((key) => ({
    label: key,
    value: key,
  }))
})
const rewriteValue = ref(null)
const title = computed(() => {
  if (setting.value) {
    return $gettext('Edit Website - %{ name }', { name: setting.value.name })
  }
  return $gettext('Edit Website')
})
const certOptions = computed(() => {
  return certs.value.map((item: any) => ({
    label: item.domains.join(', '),
    value: item.id,
  }))
})
const selectedCert = ref(null)

const handleSave = () => {
  // 如果开启了ssl但没有任何监听地址设置了ssl，则自动添加443
  if (setting.value.ssl && !setting.value.listens.some((item: any) => item.args?.includes('ssl'))) {
    const args = ['ssl']
    if (isNginx.value) {
      args.push('quic')
    }
    setting.value.listens.push({
      address: '443',
      args,
    })
  }
  // 如果关闭了ssl，自动禁用所有ssl和quic
  if (!setting.value.ssl) {
    setting.value.listens = setting.value.listens.filter((item: any) => item.address !== '443') // 443直接删掉
    setting.value.listens.forEach((item: any) => {
      item.args = []
    })
  }

  // 将真实 IP 文本转回数组
  if (setting.value.real_ip) {
    setting.value.real_ip.from = realIPFrom.value
      .split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s.length > 0)
  }

  saveLoading.value = true
  useRequest(website.saveConfig(id.value, setting.value))
    .onSuccess(() => {
      fetchSetting()
      window.$message.success($gettext('Saved successfully'))
      window.$bus.emit('website:refresh')
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}

const handleReset = () => {
  resetLoading.value = true
  useRequest(website.resetConfig(id.value))
    .onSuccess(() => {
      fetchSetting()
      window.$message.success($gettext('Reset successfully'))
    })
    .onComplete(() => {
      resetLoading.value = false
    })
}

const handleRewrite = (value: string) => {
  setting.value.rewrite = rewrites.value[value] || ''
}

const isObtainCert = ref(false)
const dnsModal = ref(false)
const dnsList = ref<any>([])
const selectedDnsId = ref<number | null>(null)

const handleObtainCert = () => {
  // 检测泛域名
  const hasWildcard = setting.value.domains?.some((domain: string) => domain.includes('*'))
  if (hasWildcard) {
    // 加载 DNS 列表并弹窗选择
    useRequest(cert.dns(1, 10000)).onSuccess(({ data }) => {
      dnsList.value = data.items.map((item: any) => ({
        label: item.name,
        value: item.id,
      }))
      if (dnsList.value.length === 0) {
        window.$message.error(
          $gettext(
            'Your website contains wildcard domains, which require DNS verification. Please add a DNS provider in Certificate Management first.',
          ),
        )
        return
      }
      selectedDnsId.value = null
      dnsModal.value = true
    })
    return
  }

  doObtainCert()
}

const handleDnsObtainCert = () => {
  if (!selectedDnsId.value) {
    window.$message.error($gettext('Please select a DNS provider'))
    return
  }
  dnsModal.value = false
  doObtainCert(selectedDnsId.value)
}

const doObtainCert = (dnsId?: number) => {
  isObtainCert.value = true
  messageReactive = window.$message.loading($gettext('Please wait...'), {
    duration: 0,
  })
  useRequest(website.obtainCert(id.value, dnsId))
    .onSuccess(() => {
      fetchSetting()
      window.$message.success($gettext('Issued successfully'))
    })
    .onComplete(() => {
      isObtainCert.value = false
      messageReactive?.destroy()
    })
}

const handleSelectCert = (value: number) => {
  const cert = certs.value.find((item: any) => item.id === value)
  if (cert && cert.cert !== '' && cert.key !== '') {
    setting.value.ssl_cert = cert.cert
    setting.value.ssl_key = cert.key
  } else {
    window.$message.error($gettext('The selected certificate is invalid'))
  }
}

const accessLogRef = ref<{ clear: () => void } | null>(null)
const errorLogRef = ref<{ clear: () => void } | null>(null)

const clearLog = async () => {
  const isError = current.value === 'error_log'
  const path = isError ? setting.value.error_log : setting.value.access_log
  if (!path || path === 'off') return
  clearLogLoading.value = true
  useRequest(file.truncate(path))
    .onSuccess(() => {
      if (isError) errorLogRef.value?.clear()
      else accessLogRef.value?.clear()
      window.$message.success($gettext('Cleared successfully'))
    })
    .onComplete(() => {
      clearLogLoading.value = false
    })
}

const onCreateListen = () => {
  return {
    address: '',
    args: [],
  }
}

const toggleArg = (args: string[], arg: string, checked: boolean) => {
  const index = args.indexOf(arg)
  if (checked && index === -1) {
    args.push(arg)
  } else if (!checked && index !== -1) {
    args.splice(index, 1)
  }
}

const hasArg = (args: string[], arg: string) => {
  return args.includes(arg)
}

// ========== 唯一 ID 生成 ==========
let idCounter = 0
const generateId = () => `_${Date.now()}_${++idCounter}`

// 确保列表项有唯一 ID
const ensureItemIds = () => {
  setting.value.upstreams?.forEach((item: any) => {
    if (!item._id) item._id = generateId()
  })
  setting.value.proxies?.forEach((item: any) => {
    if (!item._id) item._id = generateId()
  })
  setting.value.redirects?.forEach((item: any) => {
    if (!item._id) item._id = generateId()
  })
  setting.value.custom_configs?.forEach((item: any) => {
    if (!item._id) item._id = generateId()
  })
}

// 监听 setting 变化，确保所有项都有 ID
watch(
  () => setting.value,
  () => {
    ensureItemIds()
  },
  { immediate: true, deep: false },
)

// ========== Upstreams 相关 ==========
// 添加新的上游
const addUpstream = () => {
  const name = `${setting.value.name.replace(/-/g, '_')}_upstream_${(setting.value.upstreams?.length || 0) + 1}`
  if (!setting.value.upstreams) {
    setting.value.upstreams = []
  }
  setting.value.upstreams.push({
    _id: generateId(),
    name,
    servers: {},
    algo: '',
    keepalive: 32,
    resolver: [],
    resolver_timeout: 5 * 1000000000, // 5秒，以纳秒为单位
  })
}

// 删除上游
const removeUpstream = (index: number) => {
  if (setting.value.upstreams) {
    setting.value.upstreams.splice(index, 1)
  }
}

// 更新上游超时时间值
const updateUpstreamTimeoutValue = (upstream: any, value: number) => {
  const parsed = parseDuration(upstream.resolver_timeout)
  upstream.resolver_timeout = buildDuration(value, parsed.unit)
}

// 更新上游超时时间单位
const updateUpstreamTimeoutUnit = (upstream: any, unit: string) => {
  const parsed = parseDuration(upstream.resolver_timeout)
  upstream.resolver_timeout = buildDuration(parsed.value, unit)
}

// ========== Proxies 相关 ==========
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

// 从 URL 提取主机名
const extractHostFromUrl = (url: string): string => {
  try {
    const urlObj = new URL(url)
    return urlObj.hostname
  } catch {
    return ''
  }
}

// 添加新的代理
const addProxy = () => {
  if (!setting.value.proxies) {
    setting.value.proxies = []
  }
  setting.value.proxies.push({
    _id: generateId(),
    location: '/',
    pass: 'http://127.0.0.1:8080',
    host: '$host',
    sni: '',
    cache: null, // null 表示禁用缓存
    buffering: true,
    resolver: [],
    resolver_timeout: 5 * 1000000000, // 5秒，以纳秒为单位
    headers: {},
    replaces: {},
    http_version: '1.1',
    timeout: null,
    retry: null,
    client_max_body_size: 0,
    ssl_backend: null,
    response_headers: null,
    access_control: null,
  })
}

// ========== 缓存配置相关 ==========
// 创建默认缓存配置
const createDefaultCacheConfig = () => ({
  valid: { '200 302': '10m', '404': '10s' },
  no_cache_conditions: [],
  use_stale: [],
  background_update: false,
  lock: false,
  min_uses: 0,
  methods: [],
  key: '',
})

// 切换缓存启用状态
const toggleProxyCache = (proxy: any, enabled: boolean) => {
  if (enabled) {
    proxy.cache = createDefaultCacheConfig()
  } else {
    proxy.cache = null
  }
}

// 判断缓存是否启用
const isCacheEnabled = (proxy: any) => {
  return proxy.cache !== null && proxy.cache !== undefined
}

// 从字节解析为 {value, unit} 格式
const parseSize = (bytes: number): { value: number; unit: string } => {
  if (!bytes || bytes <= 0) return { value: 0, unit: 'm' }

  if (bytes >= 1024 * 1024 * 1024 && bytes % (1024 * 1024 * 1024) === 0) {
    return { value: bytes / (1024 * 1024 * 1024), unit: 'g' }
  }
  if (bytes >= 1024 * 1024 && bytes % (1024 * 1024) === 0) {
    return { value: bytes / (1024 * 1024), unit: 'm' }
  }
  if (bytes >= 1024 && bytes % 1024 === 0) {
    return { value: bytes / 1024, unit: 'k' }
  }
  return { value: bytes, unit: '' }
}

// 将 {value, unit} 转换为字节
const buildSize = (value: number, unit: string): number => {
  if (!value || value <= 0) return 0
  switch (unit) {
    case 'g':
      return value * 1024 * 1024 * 1024
    case 'm':
      return value * 1024 * 1024
    case 'k':
      return value * 1024
    default:
      return value
  }
}

// 创建默认超时配置
const createDefaultTimeoutConfig = () => ({
  connect: 60 * SECOND,
  read: 60 * SECOND,
  send: 60 * SECOND,
})

// 创建默认重试配置
const createDefaultRetryConfig = () => ({
  conditions: ['error', 'timeout'],
  tries: 0,
  timeout: 0,
})

// 创建默认 SSL 后端配置
const createDefaultSSLBackendConfig = () => ({
  verify: false,
  trusted_certificate: '',
  verify_depth: 1,
})

// 创建默认响应头配置
const createDefaultResponseHeadersConfig = () => ({
  hide: [],
  add: {},
})

// 创建默认访问控制配置
const createDefaultAccessControlConfig = () => ({
  allow: [],
  deny: [],
})

// 切换超时配置启用状态
const toggleProxyTimeout = (proxy: any, enabled: boolean) => {
  if (enabled) {
    proxy.timeout = createDefaultTimeoutConfig()
  } else {
    proxy.timeout = null
  }
}

// 切换重试配置启用状态
const toggleProxyRetry = (proxy: any, enabled: boolean) => {
  if (enabled) {
    proxy.retry = createDefaultRetryConfig()
  } else {
    proxy.retry = null
  }
}

// 切换 SSL 后端验证启用状态
const toggleProxySSLBackend = (proxy: any, enabled: boolean) => {
  if (enabled) {
    proxy.ssl_backend = createDefaultSSLBackendConfig()
  } else {
    proxy.ssl_backend = null
  }
}

// 切换响应头配置启用状态
const toggleProxyResponseHeaders = (proxy: any, enabled: boolean) => {
  if (enabled) {
    proxy.response_headers = createDefaultResponseHeadersConfig()
  } else {
    proxy.response_headers = null
  }
}

// 切换访问控制启用状态
const toggleProxyAccessControl = (proxy: any, enabled: boolean) => {
  if (enabled) {
    proxy.access_control = createDefaultAccessControlConfig()
  } else {
    proxy.access_control = null
  }
}

// 更新超时时间值
const updateProxyTimeoutValue = (proxy: any, field: string, value: number) => {
  if (!proxy.timeout) return
  const parsed = parseDuration(proxy.timeout[field])
  proxy.timeout[field] = buildDuration(value, parsed.unit)
}

// 更新超时时间单位
const updateProxyTimeoutUnit = (proxy: any, field: string, unit: string) => {
  if (!proxy.timeout) return
  const parsed = parseDuration(proxy.timeout[field])
  proxy.timeout[field] = buildDuration(parsed.value, unit)
}

// 更新请求体大小值
const updateClientMaxBodySizeValue = (proxy: any, value: number) => {
  const parsed = parseSize(proxy.client_max_body_size)
  proxy.client_max_body_size = buildSize(value, parsed.unit || 'm')
}

// 更新请求体大小单位
const updateClientMaxBodySizeUnit = (proxy: any, unit: string) => {
  const parsed = parseSize(proxy.client_max_body_size)
  proxy.client_max_body_size = buildSize(parsed.value || 0, unit)
}

// 更新重试超时值
const updateRetryTimeoutValue = (proxy: any, value: number) => {
  if (!proxy.retry) return
  const parsed = parseDuration(proxy.retry.timeout)
  proxy.retry.timeout = buildDuration(value, parsed.unit)
}

// 更新重试超时单位
const updateRetryTimeoutUnit = (proxy: any, unit: string) => {
  if (!proxy.retry) return
  const parsed = parseDuration(proxy.retry.timeout)
  proxy.retry.timeout = buildDuration(parsed.value, unit)
}

// 删除代理
const removeProxy = (index: number) => {
  if (setting.value.proxies) {
    setting.value.proxies.splice(index, 1)
  }
}

// 处理 Proxy Pass 变化，自动更新 Host
const handleProxyPassChange = (proxy: any, value: string) => {
  proxy.pass = value
  const extracted = extractHostFromUrl(value)
  if (extracted !== '') {
    proxy.host = extracted
  } else {
    proxy.host = '$host'
  }
}

// 更新 Location 匹配类型
const updateLocationType = (proxy: any, type: string) => {
  const parsed = parseLocation(proxy.location)
  proxy.location = buildLocation(type, parsed.expression)
}

// 更新 Location 表达式
const updateLocationExpression = (proxy: any, expression: string) => {
  const parsed = parseLocation(proxy.location)
  proxy.location = buildLocation(parsed.type, expression)
}

// ========== 时间单位相关 ==========
// Go time.Duration 在 JSON 中以纳秒表示
const NANOSECOND = 1
const SECOND = 1000000000 * NANOSECOND
const MINUTE = 60 * SECOND
const HOUR = 60 * MINUTE

// 时间单位选项
const timeUnitOptions = [
  { label: $gettext('Seconds'), value: 's' },
  { label: $gettext('Minutes'), value: 'm' },
  { label: $gettext('Hours'), value: 'h' },
]

// 从纳秒解析为 {value, unit} 格式
const parseDuration = (ns: number): { value: number; unit: string } => {
  if (!ns || ns <= 0) return { value: 5, unit: 's' }

  if (ns >= HOUR && ns % HOUR === 0) {
    return { value: ns / HOUR, unit: 'h' }
  }
  if (ns >= MINUTE && ns % MINUTE === 0) {
    return { value: ns / MINUTE, unit: 'm' }
  }
  return { value: Math.floor(ns / SECOND), unit: 's' }
}

// 将 {value, unit} 转换为纳秒
const buildDuration = (value: number, unit: string): number => {
  if (!value || value <= 0) value = 5
  switch (unit) {
    case 'h':
      return value * HOUR
    case 'm':
      return value * MINUTE
    default:
      return value * SECOND
  }
}

// 更新超时时间值
const updateTimeoutValue = (proxy: any, value: number) => {
  const parsed = parseDuration(proxy.resolver_timeout)
  proxy.resolver_timeout = buildDuration(value, parsed.unit)
}

// 更新超时时间单位
const updateTimeoutUnit = (proxy: any, unit: string) => {
  const parsed = parseDuration(proxy.resolver_timeout)
  proxy.resolver_timeout = buildDuration(parsed.value, unit)
}

// ========== 重定向相关 ==========
// 重定向类型选项
const redirectTypeOptions = [
  { label: $gettext('URL Redirect'), value: 'url' },
  { label: $gettext('Host Redirect'), value: 'host' },
  { label: $gettext('404 Redirect'), value: '404' },
]

// 状态码选项
const redirectStatusCodeOptions = [
  { label: '301 - ' + $gettext('Moved Permanently'), value: 301 },
  { label: '302 - ' + $gettext('Found'), value: 302 },
  { label: '307 - ' + $gettext('Temporary Redirect'), value: 307 },
  { label: '308 - ' + $gettext('Permanent Redirect'), value: 308 },
]

// 添加重定向规则
const addRedirect = () => {
  if (!setting.value.redirects) {
    setting.value.redirects = []
  }
  setting.value.redirects.push({
    _id: generateId(),
    type: 'url',
    from: '/',
    to: '/new',
    keep_uri: true,
    status_code: 308,
  })
}

// 删除重定向规则
const removeRedirect = (index: number) => {
  if (setting.value.redirects) {
    setting.value.redirects.splice(index, 1)
  }
}

// 获取重定向类型的标签
const getRedirectTypeLabel = (type: string) => {
  const option = redirectTypeOptions.find((opt) => opt.value === type)
  return option ? option.label : type
}

// ========== 高级设置相关（日志设置、限流限速、真实 IP、基本认证）==========
// 默认日志路径
const defaultAccessLog = computed(() => `/opt/ace/sites/${setting.value.name}/log/access.log`)
const defaultErrorLog = computed(() => `/opt/ace/sites/${setting.value.name}/log/error.log`)

// 日志路径选项
const accessLogOptions = computed(() => [
  { label: $gettext('Disabled'), value: 'off' },
  { label: $gettext('Default Path'), value: defaultAccessLog.value },
])
const errorLogOptions = computed(() => [
  { label: $gettext('Disabled'), value: 'off' },
  { label: $gettext('Default Path'), value: defaultErrorLog.value },
])

// 限流限速是否启用
const rateLimitEnabled = computed({
  get: () => setting.value.rate_limit !== null,
  set: (value: boolean) => {
    if (value) {
      setting.value.rate_limit = {
        per_server: 0,
        per_ip: 0,
        rate: 0,
      }
    } else {
      setting.value.rate_limit = null
    }
  },
})

// 真实 IP 是否启用
const realIPEnabled = computed({
  get: () => setting.value.real_ip !== null,
  set: (value: boolean) => {
    if (value) {
      setting.value.real_ip = {
        from: [],
        header: 'X-Real-IP',
        recursive: false,
      }
    } else {
      setting.value.real_ip = null
    }
  },
})

// 真实 IP 来源列表，多行文本与数组双向转换
const realIPFrom = computed({
  get: () => setting.value.real_ip?.from?.join('\n') ?? '',
  set: (value: string) => {
    if (setting.value.real_ip) {
      setting.value.real_ip.from = value.split('\n')
    }
  },
})

// ========== 自定义配置相关 ==========
// 添加自定义配置
const addCustomConfig = () => {
  if (!setting.value.custom_configs) {
    setting.value.custom_configs = []
  }
  const index = setting.value.custom_configs.length + 1
  setting.value.custom_configs.push({
    _id: generateId(),
    name: `custom_${index}`,
    scope: 'site',
    content: '',
  })
}

// 删除自定义配置
const removeCustomConfig = (index: number) => {
  if (setting.value.custom_configs) {
    setting.value.custom_configs.splice(index, 1)
  }
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="title"
    :style="{ width: '70vw', maxWidth: '1080px' }"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-spin :show="loading">
      <n-tabs v-model:value="current" type="line" animated>
        <n-tab-pane name="listen" :tab="$gettext('Domain & Listening')">
          <n-form v-if="setting">
            <n-form-item :label="$gettext('Domain')">
              <n-dynamic-input
                v-model:value="setting.domains"
                placeholder="example.com"
                :min="1"
                show-sort-button
              />
            </n-form-item>
            <n-form-item :label="$gettext('Listening Address')">
              <n-dynamic-input
                v-model:value="setting.listens"
                show-sort-button
                :on-create="onCreateListen"
              >
                <template #default="{ value }">
                  <div flex w-full items-center>
                    <n-input v-model:value="value.address" clearable />
                    <n-checkbox
                      :checked="hasArg(value.args, 'ssl')"
                      @update:checked="(checked: boolean) => toggleArg(value.args, 'ssl', checked)"
                      ml-5
                      mr-5
                      w-30
                    >
                      HTTPS
                    </n-checkbox>
                    <n-checkbox
                      v-if="isNginx"
                      :checked="hasArg(value.args, 'quic')"
                      @update:checked="(checked: boolean) => toggleArg(value.args, 'quic', checked)"
                      w-50
                    >
                      QUIC(HTTP3)
                    </n-checkbox>
                  </div>
                </template>
              </n-dynamic-input>
            </n-form-item>
          </n-form>
          <n-skeleton v-else text :repeat="10" />
        </n-tab-pane>
        <n-tab-pane name="basic" :tab="$gettext('Basic Settings')">
          <n-form v-if="setting">
            <n-form-item :label="$gettext('Website Directory')">
              <n-input
                v-model:value="setting.path"
                :placeholder="$gettext('Enter website directory (absolute path)')"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Running Directory')">
              <n-input
                v-model:value="setting.root"
                :placeholder="
                  $gettext('Enter running directory (needed for Laravel etc.) (absolute path)')
                "
              />
            </n-form-item>
            <n-form-item :label="$gettext('Default Document')">
              <n-dynamic-tags v-model:value="setting.index" />
            </n-form-item>
            <n-form-item v-if="setting.type == 'php'" :label="$gettext('PHP Version')">
              <n-select
                v-model:value="setting.php"
                :default-value="0"
                :options="installedEnvironment.php"
                :placeholder="$gettext('Select PHP Version')"
                @keydown.enter.prevent
              >
              </n-select>
            </n-form-item>
            <n-form-item v-if="setting.type == 'php'" :label="$gettext('Anti-cross-site Attack')">
              <n-switch v-model:value="setting.open_basedir" />
            </n-form-item>
          </n-form>
          <n-skeleton v-else text :repeat="10" />
        </n-tab-pane>
        <n-tab-pane v-if="setting.type === 'proxy'" name="upstreams" :tab="$gettext('Upstreams')">
          <n-flex vertical>
            <!-- 上游卡片列表 -->
            <draggable
              v-model="setting.upstreams"
              item-key="_id"
              handle=".drag-handle"
              :animation="200"
              ghost-class="ghost-card"
            >
              <template #item="{ element: upstream, index }">
                <n-card closable @close="removeUpstream(index)" mb-4>
                  <template #header>
                    <n-flex align="center" :size="8">
                      <!-- 拖拽手柄 -->
                      <div class="drag-handle" cursor-grab>
                        <the-icon icon="mdi:drag" :size="20" />
                      </div>
                      <span>{{ $gettext('Upstream') }}</span>
                      <n-input
                        v-model:value="upstream.name"
                        :placeholder="$gettext('Upstream name')"
                        size="small"
                        class="w-50"
                      />
                    </n-flex>
                  </template>
                  <n-form label-placement="left" label-width="140px">
                    <n-grid :cols="24" :x-gap="16">
                      <n-form-item-gi :span="12" :label="$gettext('Load Balancing Algorithm')">
                        <n-select
                          v-model:value="upstream.algo"
                          :options="
                            isNginx
                              ? [
                                  { label: $gettext('Round Robin (default)'), value: '' },
                                  { label: 'least_conn', value: 'least_conn' },
                                  { label: 'ip_hash', value: 'ip_hash' },
                                  { label: 'hash', value: 'hash' },
                                  { label: 'random', value: 'random' },
                                ]
                              : [
                                  { label: $gettext('Round Robin (default)'), value: '' },
                                  { label: $gettext('Least Busy'), value: 'bybusyness' },
                                  { label: $gettext('By Traffic'), value: 'bytraffic' },
                                ]
                          "
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" :label="$gettext('Keepalive Connections')">
                        <n-input-number
                          :value="upstream.keepalive || null"
                          :min="0"
                          :max="1000"
                          w-full
                          :placeholder="$gettext('Disabled')"
                          @update:value="(v: number | null) => (upstream.keepalive = v ?? 0)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi v-if="isNginx" :span="12" :label="$gettext('DNS Resolver')">
                        <n-dynamic-tags
                          v-model:value="upstream.resolver"
                          :placeholder="$gettext('e.g., 8.8.8.8')"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        v-if="isNginx && upstream.resolver?.length"
                        :span="12"
                        :label="$gettext('Resolver Timeout')"
                      >
                        <n-input-group>
                          <n-input-number
                            :value="parseDuration(upstream.resolver_timeout).value"
                            :min="1"
                            :max="3600"
                            flex-1
                            @update:value="
                              (v: number | null) => updateUpstreamTimeoutValue(upstream, v ?? 5)
                            "
                          />
                          <n-select
                            :value="parseDuration(upstream.resolver_timeout).unit"
                            :options="timeUnitOptions"
                            class="w-25"
                            @update:value="(v: string) => updateUpstreamTimeoutUnit(upstream, v)"
                          />
                        </n-input-group>
                      </n-form-item-gi>
                    </n-grid>
                    <n-form-item :label="$gettext('Backend Servers')">
                      <key-value-editor
                        v-model="upstream.servers"
                        :key-placeholder="$gettext('Server address, e.g., 127.0.0.1:8080')"
                        :value-placeholder="$gettext('Options, e.g., weight=5 backup')"
                        :add-button-text="$gettext('Add Server')"
                        default-key-prefix="127.0.0.1:8080"
                      />
                    </n-form-item>
                  </n-form>
                </n-card>
              </template>
            </draggable>

            <!-- 空状态 -->
            <n-empty v-if="!setting.upstreams || setting.upstreams.length === 0">
              {{ $gettext('No upstreams configured') }}
            </n-empty>

            <!-- 添加按钮 -->
            <n-button type="primary" dashed @click="addUpstream" mb-5>
              {{ $gettext('Add Upstream') }}
            </n-button>
          </n-flex>
        </n-tab-pane>
        <n-tab-pane v-if="setting.type === 'proxy'" name="proxies" :tab="$gettext('Proxies')">
          <n-flex vertical>
            <!-- 代理卡片列表 -->
            <draggable
              v-model="setting.proxies"
              item-key="_id"
              handle=".drag-handle"
              :animation="200"
              ghost-class="ghost-card"
            >
              <template #item="{ element: proxy, index }">
                <n-card closable @close="removeProxy(index)" mb-4>
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
                  <n-form label-placement="left" label-width="140px">
                    <n-grid :cols="24" :x-gap="16">
                      <n-form-item-gi v-if="isNginx" :span="12" :label="$gettext('Match Type')">
                        <n-select
                          :value="parseLocation(proxy.location).type"
                          :options="locationMatchTypes"
                          @update:value="(v: string) => updateLocationType(proxy, v)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        :span="isNginx ? 12 : 24"
                        :label="$gettext('Match Expression')"
                      >
                        <n-input
                          :value="parseLocation(proxy.location).expression"
                          :placeholder="$gettext('e.g., /, /api, ^/api/v[0-9]+/')"
                          @update:value="(v: string) => updateLocationExpression(proxy, v)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" :label="$gettext('Proxy Pass')">
                        <n-input
                          :value="proxy.pass"
                          :placeholder="
                            $gettext(
                              'Backend address, e.g., http://127.0.0.1:8080 or http://upstream_name',
                            )
                          "
                          @update:value="(v: string) => handleProxyPassChange(proxy, v)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" :label="$gettext('Proxy Host')">
                        <n-input
                          v-model:value="proxy.host"
                          :placeholder="
                            $gettext('Default: $proxy_host, or extracted from Proxy Pass')
                          "
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" :label="$gettext('Proxy SNI')">
                        <n-input
                          v-model:value="proxy.sni"
                          :placeholder="$gettext('Optional, for HTTPS backends')"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="6" :label="$gettext('Enable Cache')">
                        <n-switch
                          :value="isCacheEnabled(proxy)"
                          @update:value="(v: boolean) => toggleProxyCache(proxy, v)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="6" :label="$gettext('Enable Buffering')">
                        <n-switch v-model:value="proxy.buffering" />
                      </n-form-item-gi>
                      <n-form-item-gi v-if="isNginx" :span="12" :label="$gettext('DNS Resolver')">
                        <n-dynamic-tags
                          v-model:value="proxy.resolver"
                          :placeholder="$gettext('e.g., 8.8.8.8')"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        v-if="isNginx && proxy.resolver.length"
                        :span="12"
                        :label="$gettext('Resolver Timeout')"
                      >
                        <n-input-group>
                          <n-input-number
                            :value="parseDuration(proxy.resolver_timeout).value"
                            :min="1"
                            :max="3600"
                            flex-1
                            @update:value="(v: number | null) => updateTimeoutValue(proxy, v ?? 5)"
                          />
                          <n-select
                            :value="parseDuration(proxy.resolver_timeout).unit"
                            :options="timeUnitOptions"
                            class="w-25"
                            @update:value="(v: string) => updateTimeoutUnit(proxy, v)"
                          />
                        </n-input-group>
                      </n-form-item-gi>
                    </n-grid>
                    <!-- 可折叠配置区域 -->
                    <n-collapse :default-expanded-names="[]" mt-4>
                      <!-- 缓存配置详情 -->
                      <n-collapse-item
                        v-if="isNginx && isCacheEnabled(proxy)"
                        :title="$gettext('Cache Settings')"
                        name="cache"
                      >
                        <n-grid :cols="24" :x-gap="16">
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
                              :placeholder="
                                $gettext('Custom cache key, e.g., $scheme$host$request_uri')
                              "
                            />
                          </n-form-item-gi>
                        </n-grid>
                      </n-collapse-item>

                      <!-- 自定义请求头 -->
                      <n-collapse-item :title="$gettext('Custom Request Headers')" name="headers">
                        <key-value-editor
                          v-model="proxy.headers"
                          :key-placeholder="$gettext('Header name')"
                          :value-placeholder="
                            $gettext('Value or variable like $host, $remote_addr')
                          "
                          :add-button-text="$gettext('Add Request Header')"
                          default-key-prefix="X-Custom-Header"
                        />
                      </n-collapse-item>

                      <!-- 响应内容替换 -->
                      <n-collapse-item
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

                      <!-- 高级配置（仅 Nginx） -->
                      <n-collapse-item
                        v-if="isNginx"
                        :title="$gettext('Advanced Settings')"
                        name="advanced"
                      >
                        <n-grid :cols="24" :x-gap="16">
                          <!-- HTTP 协议版本 -->
                          <n-form-item-gi :span="8" :label="$gettext('HTTP Version')">
                            <n-select
                              v-model:value="proxy.http_version"
                              :options="[
                                { label: 'HTTP/1.0', value: '1.0' },
                                { label: 'HTTP/1.1', value: '1.1' },
                                { label: 'HTTP/2', value: '2' },
                              ]"
                              :placeholder="$gettext('Select HTTP version')"
                            />
                          </n-form-item-gi>

                          <!-- 请求体大小限制 -->
                          <n-form-item-gi :span="8" :label="$gettext('Max Body Size')">
                            <n-input-group>
                              <n-input-number
                                :value="
                                  proxy.client_max_body_size
                                    ? parseSize(proxy.client_max_body_size).value
                                    : null
                                "
                                :min="0"
                                flex-1
                                :placeholder="$gettext('Use global')"
                                @update:value="
                                  (v: number | null) => updateClientMaxBodySizeValue(proxy, v ?? 0)
                                "
                              />
                              <n-select
                                :value="
                                  parseSize(proxy.client_max_body_size || 1024 * 1024).unit || 'm'
                                "
                                :options="[
                                  { label: 'KB', value: 'k' },
                                  { label: 'MB', value: 'm' },
                                  { label: 'GB', value: 'g' },
                                ]"
                                class="w-20"
                                @update:value="(v: string) => updateClientMaxBodySizeUnit(proxy, v)"
                              />
                            </n-input-group>
                          </n-form-item-gi>

                          <!-- 超时设置开关 -->
                          <n-form-item-gi :span="8" :label="$gettext('Timeout Settings')">
                            <n-switch
                              :value="proxy.timeout !== null"
                              @update:value="(v: boolean) => toggleProxyTimeout(proxy, v)"
                            />
                          </n-form-item-gi>
                        </n-grid>

                        <!-- 超时配置详情 -->
                        <template v-if="proxy.timeout">
                          <n-grid :cols="24" :x-gap="16">
                            <n-form-item-gi :span="8" :label="$gettext('Connect Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="parseDuration(proxy.timeout.connect).value"
                                  :min="1"
                                  flex-1
                                  @update:value="
                                    (v: number | null) =>
                                      updateProxyTimeoutValue(proxy, 'connect', v ?? 1)
                                  "
                                />
                                <n-select
                                  :value="parseDuration(proxy.timeout.connect).unit"
                                  :options="timeUnitOptions"
                                  class="w-25"
                                  @update:value="
                                    (v: string) => updateProxyTimeoutUnit(proxy, 'connect', v)
                                  "
                                />
                              </n-input-group>
                            </n-form-item-gi>
                            <n-form-item-gi :span="8" :label="$gettext('Read Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="parseDuration(proxy.timeout.read).value"
                                  :min="1"
                                  flex-1
                                  @update:value="
                                    (v: number | null) =>
                                      updateProxyTimeoutValue(proxy, 'read', v ?? 1)
                                  "
                                />
                                <n-select
                                  :value="parseDuration(proxy.timeout.read).unit"
                                  :options="timeUnitOptions"
                                  class="w-25"
                                  @update:value="
                                    (v: string) => updateProxyTimeoutUnit(proxy, 'read', v)
                                  "
                                />
                              </n-input-group>
                            </n-form-item-gi>
                            <n-form-item-gi :span="8" :label="$gettext('Send Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="parseDuration(proxy.timeout.send).value"
                                  :min="1"
                                  flex-1
                                  @update:value="
                                    (v: number | null) =>
                                      updateProxyTimeoutValue(proxy, 'send', v ?? 1)
                                  "
                                />
                                <n-select
                                  :value="parseDuration(proxy.timeout.send).unit"
                                  :options="timeUnitOptions"
                                  class="w-25"
                                  @update:value="
                                    (v: string) => updateProxyTimeoutUnit(proxy, 'send', v)
                                  "
                                />
                              </n-input-group>
                            </n-form-item-gi>
                          </n-grid>
                        </template>

                        <n-grid :cols="24" :x-gap="16">
                          <!-- 重试配置开关 -->
                          <n-form-item-gi :span="8" :label="$gettext('Retry Settings')">
                            <n-switch
                              :value="proxy.retry !== null"
                              @update:value="(v: boolean) => toggleProxyRetry(proxy, v)"
                            />
                          </n-form-item-gi>

                          <!-- SSL 后端验证开关（仅 https） -->
                          <n-form-item-gi
                            v-if="proxy.pass?.startsWith('https')"
                            :span="8"
                            :label="$gettext('SSL Backend Verify')"
                          >
                            <n-switch
                              :value="proxy.ssl_backend !== null"
                              @update:value="(v: boolean) => toggleProxySSLBackend(proxy, v)"
                            />
                          </n-form-item-gi>

                          <!-- 响应头修改开关 -->
                          <n-form-item-gi :span="8" :label="$gettext('Response Headers')">
                            <n-switch
                              :value="proxy.response_headers !== null"
                              @update:value="(v: boolean) => toggleProxyResponseHeaders(proxy, v)"
                            />
                          </n-form-item-gi>
                        </n-grid>

                        <!-- 重试配置详情 -->
                        <template v-if="proxy.retry">
                          <n-grid :cols="24" :x-gap="16">
                            <n-form-item-gi :span="12" :label="$gettext('Retry Conditions')">
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
                            <n-form-item-gi :span="6" :label="$gettext('Max Tries')">
                              <n-input-number
                                :value="proxy.retry.tries || null"
                                :min="0"
                                :placeholder="$gettext('Unlimited')"
                                @update:value="(v: number | null) => (proxy.retry.tries = v ?? 0)"
                              />
                            </n-form-item-gi>
                            <n-form-item-gi :span="6" :label="$gettext('Retry Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="
                                    proxy.retry.timeout
                                      ? parseDuration(proxy.retry.timeout).value
                                      : null
                                  "
                                  :min="0"
                                  flex-1
                                  :placeholder="$gettext('Unlimited')"
                                  @update:value="
                                    (v: number | null) => updateRetryTimeoutValue(proxy, v ?? 0)
                                  "
                                />
                                <n-select
                                  :value="parseDuration(proxy.retry.timeout).unit"
                                  :options="timeUnitOptions"
                                  class="w-25"
                                  @update:value="(v: string) => updateRetryTimeoutUnit(proxy, v)"
                                />
                              </n-input-group>
                            </n-form-item-gi>
                          </n-grid>
                        </template>

                        <!-- SSL 后端验证详情 -->
                        <template v-if="proxy.ssl_backend && proxy.pass?.startsWith('https')">
                          <n-grid :cols="24" :x-gap="16">
                            <n-form-item-gi :span="6" :label="$gettext('Enable Verify')">
                              <n-switch v-model:value="proxy.ssl_backend.verify" />
                            </n-form-item-gi>
                            <n-form-item-gi :span="6" :label="$gettext('Verify Depth')">
                              <n-input-number
                                v-model:value="proxy.ssl_backend.verify_depth"
                                :min="1"
                                :max="10"
                              />
                            </n-form-item-gi>
                            <n-form-item-gi :span="12" :label="$gettext('Trusted Certificate')">
                              <n-input
                                v-model:value="proxy.ssl_backend.trusted_certificate"
                                :placeholder="
                                  $gettext(
                                    'CA certificate path, e.g. /etc/ssl/certs/ca-certificates.crt',
                                  )
                                "
                              />
                            </n-form-item-gi>
                          </n-grid>
                        </template>

                        <!-- 响应头修改详情 -->
                        <template v-if="proxy.response_headers">
                          <n-grid :cols="24" :x-gap="16">
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
                            </n-form-item-gi>
                          </n-grid>
                        </template>

                        <n-grid :cols="24" :x-gap="16">
                          <!-- IP 访问控制开关 -->
                          <n-form-item-gi :span="8" :label="$gettext('IP Access Control')">
                            <n-switch
                              :value="proxy.access_control !== null"
                              @update:value="(v: boolean) => toggleProxyAccessControl(proxy, v)"
                            />
                          </n-form-item-gi>
                        </n-grid>

                        <!-- IP 访问控制详情 -->
                        <template v-if="proxy.access_control">
                          <n-grid :cols="24" :x-gap="16">
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
                    </n-collapse>
                  </n-form>
                </n-card>
              </template>
            </draggable>

            <!-- 空状态 -->
            <n-empty v-if="!setting.proxies || setting.proxies.length === 0">
              {{ $gettext('No proxy rules configured') }}
            </n-empty>

            <!-- 添加按钮 -->
            <n-button type="primary" dashed @click="addProxy" mb-5>
              {{ $gettext('Add Proxy Rule') }}
            </n-button>
          </n-flex>
        </n-tab-pane>
        <n-tab-pane name="https" tab="HTTPS">
          <n-flex vertical v-if="setting">
            <n-card v-if="setting.ssl && setting.ssl_issuer != ''">
              <n-descriptions :column="3">
                <n-descriptions-item>
                  <template #label>{{ $gettext('Certificate Validity') }}</template>
                  <n-flex>
                    <n-tag>{{ setting.ssl_not_before }}</n-tag>
                    -
                    <n-tag>{{ setting.ssl_not_after }}</n-tag>
                  </n-flex>
                </n-descriptions-item>
                <n-descriptions-item>
                  <template #label>{{ $gettext('Issuer') }}</template>
                  <n-flex>
                    <n-tag>{{ setting.ssl_issuer }}</n-tag>
                  </n-flex>
                </n-descriptions-item>
                <n-descriptions-item>
                  <template #label>{{ $gettext('Domains') }}</template>
                  <n-flex>
                    <n-tag v-for="item in setting.ssl_dns_names" :key="item">{{ item }}</n-tag>
                  </n-flex>
                </n-descriptions-item>
              </n-descriptions>
            </n-card>
            <n-form>
              <n-grid :cols="24" :x-gap="24">
                <n-form-item-gi :span="12" :label="$gettext('Main Switch')">
                  <n-switch v-model:value="setting.ssl" />
                </n-form-item-gi>
                <n-form-item-gi
                  v-if="setting.ssl"
                  :span="12"
                  :label="$gettext('Use Existing Certificate')"
                >
                  <n-select
                    v-model:value="selectedCert"
                    :options="certOptions"
                    @update-value="handleSelectCert"
                  />
                </n-form-item-gi>
              </n-grid>
            </n-form>
            <n-form v-if="setting.ssl">
              <n-grid :cols="24" :x-gap="24">
                <n-gi :span="12">
                  <n-form inline>
                    <n-form-item label="HSTS">
                      <n-switch v-model:value="setting.hsts" />
                    </n-form-item>
                    <n-form-item :label="$gettext('HTTP Redirect')">
                      <n-switch v-model:value="setting.http_redirect" />
                    </n-form-item>
                    <n-form-item :label="$gettext('OCSP Stapling')">
                      <n-switch v-model:value="setting.ocsp" />
                    </n-form-item>
                  </n-form>
                </n-gi>
                <n-form-item-gi :span="12" :label="$gettext('TLS Version')">
                  <n-select
                    v-model:value="setting.ssl_protocols"
                    :options="[
                      { label: 'TLS 1.0', value: 'TLSv1' },
                      { label: 'TLS 1.1', value: 'TLSv1.1' },
                      { label: 'TLS 1.2', value: 'TLSv1.2' },
                      { label: 'TLS 1.3', value: 'TLSv1.3' },
                    ]"
                    multiple
                  />
                </n-form-item-gi>
              </n-grid>
            </n-form>
            <n-form v-if="setting.ssl">
              <n-grid :cols="2" :x-gap="24">
                <n-gi>
                  <n-form-item :label="$gettext('Certificate')">
                    <n-input
                      v-model:value="setting.ssl_cert"
                      type="textarea"
                      :placeholder="$gettext('Enter the content of the PEM certificate file')"
                      rows="10"
                    />
                  </n-form-item>
                </n-gi>
                <n-gi>
                  <n-form-item :label="$gettext('Private Key')">
                    <n-input
                      v-model:value="setting.ssl_key"
                      type="textarea"
                      :placeholder="$gettext('Enter the content of the KEY private key file')"
                      rows="10"
                    />
                  </n-form-item>
                </n-gi>
              </n-grid>
            </n-form>
          </n-flex>
          <n-skeleton v-else text :repeat="10" />
        </n-tab-pane>
        <n-tab-pane v-if="setting.type == 'php'" name="rewrite" :tab="$gettext('Rewrite')">
          <n-flex vertical>
            <n-form v-if="isNginx" label-placement="left" label-width="auto">
              <n-form-item :label="$gettext('Presets')">
                <n-select
                  v-model:value="rewriteValue"
                  clearable
                  :options="rewriteOptions"
                  @update-value="handleRewrite"
                />
              </n-form-item>
            </n-form>
            <common-editor v-if="setting" v-model:value="setting.rewrite" height="60vh" />
          </n-flex>
        </n-tab-pane>
        <n-tab-pane name="redirects" :tab="$gettext('Redirects')">
          <n-flex vertical>
            <!-- 重定向卡片列表 -->
            <draggable
              v-model="setting.redirects"
              item-key="_id"
              handle=".drag-handle"
              :animation="200"
              ghost-class="ghost-card"
            >
              <template #item="{ element: redirect, index }">
                <n-card closable @close="removeRedirect(index)" mb-4>
                  <template #header>
                    <n-flex align="center" :size="8">
                      <!-- 拖拽手柄 -->
                      <div class="drag-handle" cursor-grab>
                        <the-icon icon="mdi:drag" :size="20" />
                      </div>
                      <span>{{ $gettext('Rule') }} #{{ index + 1 }}</span>
                      <n-tag size="small" :type="redirect.type === '404' ? 'warning' : 'default'">
                        {{ getRedirectTypeLabel(redirect.type) }}
                      </n-tag>
                      <template v-if="redirect.type !== '404'">
                        <n-tag size="small">{{ redirect.from }}</n-tag>
                        <the-icon icon="mdi:arrow-right-bold" :size="20" />
                      </template>
                      <n-tag size="small" type="success">{{ redirect.to }}</n-tag>
                    </n-flex>
                  </template>
                  <n-form label-placement="left" label-width="140px">
                    <n-grid :cols="24" :x-gap="16">
                      <n-form-item-gi :span="12" :label="$gettext('Redirect Type')">
                        <n-select v-model:value="redirect.type" :options="redirectTypeOptions" />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" :label="$gettext('Status Code')">
                        <n-select
                          v-model:value="redirect.status_code"
                          :options="redirectStatusCodeOptions"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        v-if="redirect.type !== '404'"
                        :span="12"
                        :label="$gettext('Source')"
                      >
                        <n-input
                          v-model:value="redirect.from"
                          :placeholder="
                            redirect.type === 'url'
                              ? $gettext('Source path, e.g., /old')
                              : $gettext('Source host, e.g., example.com')
                          "
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        :span="redirect.type === '404' ? 24 : 12"
                        :label="$gettext('Target')"
                      >
                        <n-input
                          v-model:value="redirect.to"
                          :placeholder="
                            redirect.type === 'url'
                              ? $gettext('Target path, e.g., /new')
                              : $gettext('Target URL, e.g., https://example.com')
                          "
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" :label="$gettext('Keep URI')">
                        <n-switch v-model:value="redirect.keep_uri" />
                        <n-text depth="3" class="ml-2">
                          {{ $gettext('Keep the original request path and query parameters') }}
                        </n-text>
                      </n-form-item-gi>
                    </n-grid>
                  </n-form>
                </n-card>
              </template>
            </draggable>

            <!-- 空状态 -->
            <n-empty v-if="!setting.redirects || setting.redirects.length === 0">
              {{ $gettext('No redirect rules configured') }}
            </n-empty>

            <!-- 添加按钮 -->
            <n-button type="primary" dashed @click="addRedirect" mb-5>
              {{ $gettext('Add Redirect Rule') }}
            </n-button>
          </n-flex>
        </n-tab-pane>
        <n-tab-pane name="advanced" :tab="$gettext('Advanced Settings')">
          <n-collapse accordion>
            <!-- 访问统计（仅 nginx） -->
            <n-collapse-item
              v-if="isNginx"
              :title="$gettext('Access Statistics')"
              name="stat_settings"
            >
              <n-form label-placement="left" label-width="140px">
                <n-form-item :label="$gettext('Enable Statistics')">
                  <n-switch v-model:value="setting.stat_enabled" />
                </n-form-item>
              </n-form>
            </n-collapse-item>

            <!-- 日志设置 -->
            <n-collapse-item :title="$gettext('Log Settings')" name="log_settings">
              <n-form label-placement="left" label-width="140px">
                <n-form-item :label="$gettext('Access Log')">
                  <n-select
                    v-model:value="setting.access_log"
                    :options="accessLogOptions"
                    :placeholder="defaultAccessLog"
                    filterable
                    tag
                  />
                </n-form-item>
                <n-form-item :label="$gettext('Error Log')">
                  <n-select
                    v-model:value="setting.error_log"
                    :options="errorLogOptions"
                    :placeholder="defaultErrorLog"
                    filterable
                    tag
                  />
                </n-form-item>
              </n-form>
            </n-collapse-item>

            <!-- 限流限速设置 -->
            <n-collapse-item :title="$gettext('Rate Limiting')" name="rate_limit">
              <n-form label-placement="left" label-width="140px">
                <n-form-item :label="$gettext('Enable Rate Limiting')">
                  <n-switch v-model:value="rateLimitEnabled" />
                </n-form-item>
                <template v-if="rateLimitEnabled && setting.rate_limit">
                  <n-form-item :label="$gettext('Concurrent Limit')">
                    <n-input-number
                      :value="setting.rate_limit.per_server || null"
                      :min="0"
                      :max="100000"
                      w-full
                      :placeholder="$gettext('Unlimited')"
                      @update:value="(v: number | null) => (setting.rate_limit.per_server = v ?? 0)"
                    />
                    <template #feedback>
                      {{ $gettext('Limit the maximum concurrent connections for this site') }}
                    </template>
                  </n-form-item>
                  <n-form-item :label="$gettext('Per IP Limit')">
                    <n-input-number
                      :value="setting.rate_limit.per_ip || null"
                      :min="0"
                      :max="10000"
                      w-full
                      :placeholder="$gettext('Unlimited')"
                      @update:value="(v: number | null) => (setting.rate_limit.per_ip = v ?? 0)"
                    />
                    <template #feedback>
                      {{ $gettext('Limit the maximum concurrent connections per IP') }}
                    </template>
                  </n-form-item>
                  <n-form-item :label="$gettext('Rate Limit')">
                    <n-input-number
                      :value="setting.rate_limit.rate || null"
                      :min="0"
                      :max="1000000"
                      w-full
                      :placeholder="$gettext('Unlimited')"
                      @update:value="(v: number | null) => (setting.rate_limit.rate = v ?? 0)"
                    />
                    <template #feedback>
                      {{ $gettext('Limit the rate of each request (unit: KB)') }}
                    </template>
                  </n-form-item>
                </template>
              </n-form>
            </n-collapse-item>

            <!-- 真实 IP 设置 -->
            <n-collapse-item :title="$gettext('Real IP')" name="real_ip">
              <n-alert type="info" mb-4>
                {{
                  $gettext(
                    'Configure trusted proxy IPs (e.g., CDN or Frp) to identify real visitor IPs.',
                  )
                }}
              </n-alert>
              <n-alert type="warning" mb-4>
                {{
                  $gettext(
                    'If using Frp, fill in the Frp IP address (e.g., 127.0.0.1). If using CDN, fill in the CDN IP ranges. If unsure, you can fill in 0.0.0.0/0 (ipv4) or ::/0 (ipv6) [insecure].',
                  )
                }}
              </n-alert>
              <n-form label-placement="left" label-width="140px">
                <n-form-item :label="$gettext('Enable')">
                  <n-switch v-model:value="realIPEnabled" />
                </n-form-item>
                <template v-if="realIPEnabled && setting.real_ip">
                  <n-form-item :label="$gettext('IP Sources')">
                    <n-input
                      v-model:value="realIPFrom"
                      type="textarea"
                      :placeholder="$gettext('One per line, e.g., 127.0.0.1 or 10.0.0.0/8')"
                      :autosize="{ minRows: 3, maxRows: 10 }"
                    />
                  </n-form-item>
                  <n-form-item :label="$gettext('IP Header')">
                    <n-select
                      v-model:value="setting.real_ip.header"
                      :options="[
                        { label: 'X-Real-IP', value: 'X-Real-IP' },
                        { label: 'X-Forwarded-For', value: 'X-Forwarded-For' },
                        { label: 'CF-Connecting-IP', value: 'CF-Connecting-IP' },
                        { label: 'True-Client-IP', value: 'True-Client-IP' },
                        { label: 'Ali-Cdn-Real-Ip', value: 'Ali-Cdn-Real-Ip' },
                        { label: 'EO-Connecting-IP', value: 'EO-Connecting-IP' },
                      ]"
                      filterable
                      tag
                    />
                  </n-form-item>
                  <n-form-item :label="$gettext('Recursive')">
                    <n-switch v-model:value="setting.real_ip.recursive" />
                    <template #feedback>
                      {{ $gettext('Recursively search for real IP in X-Forwarded-For header') }}
                    </template>
                  </n-form-item>
                </template>
              </n-form>
            </n-collapse-item>

            <!-- 基本认证设置 -->
            <n-collapse-item :title="$gettext('Basic Authentication')" name="basic_auth">
              <n-form label-placement="left" label-width="140px">
                <n-form-item :label="$gettext('User Credentials')">
                  <key-value-editor
                    v-model="setting.basic_auth"
                    :key-placeholder="$gettext('Username')"
                    :value-placeholder="$gettext('Password')"
                    :add-button-text="$gettext('Add User')"
                    default-key-prefix="user"
                    value-type="password"
                    :show-password-toggle="true"
                  />
                </n-form-item>
              </n-form>
              <n-alert v-if="Object.keys(setting.basic_auth || {}).length > 0" type="info">
                {{
                  $gettext(
                    'Visitors will need to enter a username and password to access this website.',
                  )
                }}
              </n-alert>
            </n-collapse-item>
          </n-collapse>
        </n-tab-pane>
        <n-tab-pane name="custom_configs" :tab="$gettext('Custom Configs')">
          <n-flex vertical>
            <!-- 自定义配置列表 -->
            <draggable
              v-model="setting.custom_configs"
              item-key="_id"
              handle=".drag-handle"
              :animation="200"
              ghost-class="ghost-card"
            >
              <template #item="{ element: config, index }">
                <n-card closable @close="removeCustomConfig(index)" mb-4>
                  <template #header>
                    <n-flex align="center" :size="8">
                      <!-- 拖拽手柄 -->
                      <div class="drag-handle" cursor-grab>
                        <the-icon icon="mdi:drag" :size="20" />
                      </div>
                      <span>{{ $gettext('Config') }} #{{ index + 1 }}</span>
                    </n-flex>
                  </template>
                  <n-form label-placement="left" label-width="100px">
                    <n-grid :cols="24" :x-gap="16">
                      <n-form-item-gi :span="12" :label="$gettext('Name')">
                        <n-input
                          v-model:value="config.name"
                          :placeholder="
                            $gettext('Config name (letters, numbers, underscore, hyphen)')
                          "
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" :label="$gettext('Scope')">
                        <n-select
                          v-model:value="config.scope"
                          :options="[
                            { label: $gettext('This Website'), value: 'site' },
                            { label: $gettext('Global'), value: 'shared' },
                          ]"
                        />
                      </n-form-item-gi>
                    </n-grid>
                    <n-form-item :label="$gettext('Content')">
                      <common-editor
                        v-model:value="config.content"
                        height="30vh"
                        :lang="isNginx ? 'nginx' : 'apacheconf'"
                      />
                    </n-form-item>
                  </n-form>
                </n-card>
              </template>
            </draggable>

            <!-- 空状态 -->
            <n-empty v-if="!setting.custom_configs || setting.custom_configs.length === 0">
              {{ $gettext('No custom configs') }}
            </n-empty>

            <!-- 添加按钮 -->
            <n-button type="primary" dashed @click="addCustomConfig" mb-5>
              {{ $gettext('Add Custom Config') }}
            </n-button>
          </n-flex>
        </n-tab-pane>
        <n-tab-pane
          v-if="setting.access_log && setting.access_log !== 'off'"
          name="log"
          :tab="$gettext('Access Log')"
        >
          <n-flex vertical>
            <n-flex flex items-center>
              <n-alert type="warning" w-full>
                {{ $gettext('All logs can be viewed by downloading the file') }}
                <n-tag>{{ setting.access_log }}</n-tag>
                {{ $gettext('view') }}.
              </n-alert>
            </n-flex>
            <realtime-log ref="accessLogRef" :path="setting.access_log" pb-5 />
          </n-flex>
        </n-tab-pane>
        <n-tab-pane
          v-if="setting.error_log && setting.error_log !== 'off'"
          name="error_log"
          :tab="$gettext('Error Log')"
        >
          <n-flex vertical>
            <n-flex flex items-center>
              <n-alert type="warning" w-full>
                {{ $gettext('All logs can be viewed by downloading the file') }}
                <n-tag>{{ setting.error_log }}</n-tag>
                {{ $gettext('view') }}.
              </n-alert>
            </n-flex>
            <realtime-log ref="errorLogRef" :path="setting.error_log" />
          </n-flex>
        </n-tab-pane>
      </n-tabs>
    </n-spin>
    <template #footer>
      <n-flex justify="end">
        <ConfirmDialog
          type="danger"
          :content="$gettext('Are you sure you want to clear?')"
          @confirm="clearLog"
        >
          <template #trigger>
            <n-button type="primary" :loading="clearLogLoading" :disabled="clearLogLoading">
              {{ $gettext('Clear Logs') }}
            </n-button>
          </template>
        </ConfirmDialog>
        <n-button
          v-if="current === 'https' && setting && setting.domains.length > 0"
          :loading="isObtainCert"
          :disabled="isObtainCert"
          type="info"
          @click="handleObtainCert"
        >
          {{ $gettext('One-click Certificate Issuance') }}
        </n-button>
        <ConfirmDialog
          type="danger"
          :content="$gettext('Are you sure you want to reset the configuration?')"
          @confirm="handleReset"
        >
          <template #trigger>
            <n-button type="warning" :loading="resetLoading" :disabled="resetLoading">
              {{ $gettext('Reset Configuration') }}
            </n-button>
          </template>
        </ConfirmDialog>
        <n-button @click="show = false">
          {{ $gettext('Cancel') }}
        </n-button>
        <n-button
          v-if="current !== 'log' && current !== 'error_log'"
          type="primary"
          :loading="saveLoading"
          :disabled="saveLoading"
          @click="handleSave"
        >
          {{ $gettext('Save') }}
        </n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal
    v-model:show="dnsModal"
    preset="card"
    :title="$gettext('Select DNS Provider')"
    style="width: 60vw"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-alert type="warning">
        {{
          $gettext(
            'Your website contains wildcard domains (e.g. *.example.com), which require DNS verification to issue certificates.',
          )
        }}
      </n-alert>
      <n-form>
        <n-form-item :label="$gettext('DNS')">
          <n-select
            v-model:value="selectedDnsId"
            :placeholder="$gettext('Select DNS for certificate issuance')"
            :options="dnsList"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :disabled="!selectedDnsId" @click="handleDnsObtainCert">
        {{ $gettext('Issue') }}
      </n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped>
/* 拖拽时的占位卡片 */
:deep(.ghost-card) {
  opacity: 0.5;
}
</style>

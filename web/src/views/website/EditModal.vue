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
import ListInput from '@/components/common/ListInput.vue'
import PathSelector from '@/components/common/PathSelector.vue'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { webserverFeatures } from '@/utils'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const editId = defineModel<number>('editId', { type: Number, required: true })

const { $gettext } = useGettext()
const { isMobile } = useResponsive()
const formLayout = useFormLayout(140)
const presetFormLayout = useFormLayout('auto')
const configFormLayout = useFormLayout(100)
let messageReactive: MessageReactive | null = null

const current = ref('listen')
const loading = ref(false)
const saveLoading = ref(false)
const resetLoading = ref(false)
const switchTypeLoading = ref(false)
const clearLogLoading = ref(false)
const id = ref(0)
const targetType = ref('')
const targetPHP = ref<number | null>(null)
const targetProxy = ref('')
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
  basic_auth: [],
  custom_configs: [],
}
const setting = ref<any>({ ...initialSetting })
const fetchSetting = () => {
  loading.value = true
  useRequest(website.config(id.value))
    .onSuccess(({ data }: any) => {
      setting.value = data
      targetType.value = ''
      targetPHP.value = null
      targetProxy.value = ''
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

const showPathSelector = ref(false)
const pathSelectorTarget = ref<'path' | 'root'>('path')
const handleSelectPath = (target: 'path' | 'root') => {
  pathSelectorTarget.value = target
  showPathSelector.value = true
}

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    webserver: 'nginx',
    php: [],
    db: [
      {
        label: '',
        value: '',
      },
    ],
  },
})

// 当前 Web 服务器能力
const features = computed(() => webserverFeatures(installedEnvironment.value.webserver))
// 上游负载均衡算法选项，纯算法名不翻译
const upstreamAlgoLabels: Record<string, () => string> = {
  '': () => $gettext('Round Robin (default)'),
}
const upstreamAlgoOptions = computed(() =>
  features.value.upstreamAlgos.map((value) => ({
    label: upstreamAlgoLabels[value]?.() ?? value,
    value,
  })),
)
const websiteTypeOptions = computed(() => [
  { label: $gettext('Reverse Proxy'), value: 'proxy', disabled: setting.value.type === 'proxy' },
  { label: $gettext('PHP'), value: 'php', disabled: setting.value.type === 'php' },
  { label: $gettext('Pure Static'), value: 'static', disabled: setting.value.type === 'static' },
])
const websiteTypeLabel = (type: string) =>
  websiteTypeOptions.value.find((item) => item.value === type)?.label ?? type
const canSwitchType = computed(() => {
  if (!targetType.value || targetType.value === setting.value.type) {
    return false
  }
  if (targetType.value === 'php') {
    return Boolean(targetPHP.value)
  }
  if (targetType.value === 'proxy') {
    return targetProxy.value.trim() !== ''
  }
  return true
})
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
  // 目录级基本认证规则必须填写目录
  if (setting.value.basic_auth.some((a: any) => a._scope === 'dir' && !a.path?.trim())) {
    window.$message.error($gettext('Please enter the directory for basic authentication rules'))
    return
  }
  // 如果开启了ssl但没有任何监听地址设置了ssl，则自动添加443
  if (setting.value.ssl && !setting.value.listens.some((item: any) => item.args?.includes('ssl'))) {
    const args = ['ssl']
    if (features.value.quic) {
      args.push('quic')
    }
    // 存在 IPv6 监听时同步添加 IPv6 SSL 监听
    if (setting.value.listens.some((item: any) => item.address?.startsWith('[::]'))) {
      setting.value.listens.push({
        address: '[::]:443',
        args: [...args],
      })
    }
    setting.value.listens.push({
      address: '443',
      args,
    })
  }
  // 如果关闭了ssl，移除所有 SSL 监听（443、[::]:443 及带 ssl/quic 参数的地址）
  if (!setting.value.ssl) {
    setting.value.listens = setting.value.listens.filter(
      (item: any) =>
        item.address !== '443' &&
        !item.address?.endsWith(':443') &&
        !item.args?.includes('ssl') &&
        !item.args?.includes('quic'),
    )
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

const handleSwitchType = () => {
  const from = websiteTypeLabel(setting.value.type)
  const to = websiteTypeLabel(targetType.value)
  window.$dialog.warning({
    title: $gettext('Confirm Website Type Switch'),
    content: $gettext(
      'Switch the website type from %{ from } to %{ to }? Common settings and website files will be preserved. The original type-specific configuration will be deleted and rebuilt for the new type.',
      { from, to },
    ),
    positiveText: $gettext('Confirm Switch'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      switchTypeLoading.value = true
      useRequest(
        website.switchType(id.value, {
          type: targetType.value,
          php: targetType.value === 'php' ? targetPHP.value : 0,
          proxy: targetType.value === 'proxy' ? targetProxy.value.trim() : '',
        }),
      )
        .onSuccess(() => {
          fetchSetting()
          window.$bus.emit('website:refresh')
          window.$message.success($gettext('Website type switched successfully'))
        })
        .onComplete(() => {
          switchTypeLoading.value = false
        })
    },
  })
}

const handleReset = () => {
  resetLoading.value = true
  useRequest(website.resetConfig(id.value))
    .onSuccess(() => {
      fetchSetting()
      window.$bus.emit('website:refresh')
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
      window.$bus.emit('website:refresh')
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
    ensureTimeUnit(item, 'resolver_timeout', item.resolver_timeout)
  })
  setting.value.proxies?.forEach((item: any) => {
    if (!item._id) item._id = generateId()
    if (!item._size_unit) item._size_unit = parseSize(item.client_max_body_size).unit || 'm'
    ensureTimeUnit(item, 'resolver_timeout', item.resolver_timeout)
    if (item.timeout) {
      ensureTimeUnit(item, 'timeout_connect', item.timeout.connect)
      ensureTimeUnit(item, 'timeout_read', item.timeout.read)
      ensureTimeUnit(item, 'timeout_send', item.timeout.send)
    }
    if (item.retry) {
      ensureTimeUnit(item, 'retry_timeout', item.retry.timeout)
    }
  })
  setting.value.redirects?.forEach((item: any) => {
    if (!item._id) item._id = generateId()
  })
  setting.value.custom_configs?.forEach((item: any) => {
    if (!item._id) item._id = generateId()
  })
  if (!Array.isArray(setting.value.basic_auth)) setting.value.basic_auth = []
  setting.value.basic_auth.forEach((item: any) => {
    if (!item._id) item._id = generateId()
    if (!item._scope) item._scope = !item.path || item.path === '/' ? 'site' : 'dir'
  })
}

// 缺省时按字节值推断时间单位（仅初始化）
const ensureTimeUnit = (obj: any, key: string, ns: number | null | undefined) => {
  if (!obj._units) obj._units = {}
  if (!obj._units[key]) obj._units[key] = parseDurationUnit(ns)
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
    _units: { resolver_timeout: 's' },
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
  upstream.resolver_timeout =
    value > 0 ? value * durationUnitFactor(getUnitOf(upstream, 'resolver_timeout')) : 0
}

// 更新上游超时时间单位
const updateUpstreamTimeoutUnit = (upstream: any, unit: string) => {
  setUnitOf(upstream, 'resolver_timeout', unit)
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
    _size_unit: 'm',
    _units: { resolver_timeout: 's' },
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
    custom: '',
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
  return value * sizeUnitFactor(unit)
}

// 单位对应的字节换算因子
const sizeUnitFactor = (unit: string): number => {
  switch (unit) {
    case 'g':
      return 1024 * 1024 * 1024
    case 'm':
      return 1024 * 1024
    case 'k':
      return 1024
    default:
      return 1
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
    setUnitOf(proxy, 'timeout_connect', parseDurationUnit(proxy.timeout.connect))
    setUnitOf(proxy, 'timeout_read', parseDurationUnit(proxy.timeout.read))
    setUnitOf(proxy, 'timeout_send', parseDurationUnit(proxy.timeout.send))
  } else {
    proxy.timeout = null
  }
}

// 切换重试配置启用状态
const toggleProxyRetry = (proxy: any, enabled: boolean) => {
  if (enabled) {
    proxy.retry = createDefaultRetryConfig()
    setUnitOf(proxy, 'retry_timeout', 's')
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
  proxy.timeout[field] =
    value > 0 ? value * durationUnitFactor(getUnitOf(proxy, `timeout_${field}`)) : 0
}

// 更新超时时间单位
const updateProxyTimeoutUnit = (proxy: any, field: string, unit: string) => {
  if (!proxy.timeout) return
  setUnitOf(proxy, `timeout_${field}`, unit)
}

// 当前显示值（按 _size_unit 换算，字节为权威值）
const getClientMaxBodySizeValue = (proxy: any): number | null => {
  if (!proxy.client_max_body_size) return null
  return proxy.client_max_body_size / sizeUnitFactor(proxy._size_unit || 'm')
}

// 更新请求体大小值（按当前单位换算回字节）
const updateClientMaxBodySizeValue = (proxy: any, value: number) => {
  proxy.client_max_body_size = buildSize(value, proxy._size_unit || 'm')
}

// 更新请求体大小单位（仅改显示，字节保持不变）
const updateClientMaxBodySizeUnit = (proxy: any, unit: string) => {
  proxy._size_unit = unit
}

// 更新重试超时值
const updateRetryTimeoutValue = (proxy: any, value: number) => {
  if (!proxy.retry) return
  proxy.retry.timeout =
    value > 0 ? value * durationUnitFactor(getUnitOf(proxy, 'retry_timeout')) : 0
}

// 更新重试超时单位
const updateRetryTimeoutUnit = (proxy: any, unit: string) => {
  if (!proxy.retry) return
  setUnitOf(proxy, 'retry_timeout', unit)
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

// 时间单位换算因子（纳秒）
const durationUnitFactor = (unit: string): number => {
  switch (unit) {
    case 'h':
      return HOUR
    case 'm':
      return MINUTE
    default:
      return SECOND
  }
}

// 从纳秒推断显示单位（仅用于初始化 _units，加载已有数据时保持视觉一致）
const parseDurationUnit = (ns: number | null | undefined): string => {
  if (!ns || ns <= 0) return 's'
  if (ns >= HOUR && ns % HOUR === 0) return 'h'
  if (ns >= MINUTE && ns % MINUTE === 0) return 'm'
  return 's'
}

// 获取对象上某字段的当前单位
const getUnitOf = (obj: any, key: string): string => obj?._units?.[key] || 's'

// 设置对象上某字段的单位（仅改 UI 状态，不改字节）
const setUnitOf = (obj: any, key: string, unit: string): void => {
  if (!obj._units) obj._units = {}
  obj._units[key] = unit
}

// 把纳秒按 unit 换算成显示值
const getDurationDisplay = (ns: number | null | undefined, unit: string): number | null => {
  if (!ns || ns <= 0) return null
  return ns / durationUnitFactor(unit)
}

// 更新代理 resolver 超时时间值
const updateTimeoutValue = (proxy: any, value: number) => {
  proxy.resolver_timeout =
    value > 0 ? value * durationUnitFactor(getUnitOf(proxy, 'resolver_timeout')) : 0
}

// 更新代理 resolver 超时时间单位
const updateTimeoutUnit = (proxy: any, unit: string) => {
  setUnitOf(proxy, 'resolver_timeout', unit)
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

// ========== 基本认证相关 ==========
// 是否已存在其他整站规则（整站规则最多一条）
const hasSiteBasicAuth = (self?: any) =>
  setting.value.basic_auth.some((a: any) => a !== self && a._scope === 'site')

// 添加基本认证规则
const addBasicAuth = () => {
  const rule: any = { _id: generateId(), users: {} }
  setBasicAuthScope(rule, hasSiteBasicAuth() ? 'dir' : 'site')
  setting.value.basic_auth.push(rule)
}

// 删除基本认证规则
const removeBasicAuth = (index: number) => {
  setting.value.basic_auth.splice(index, 1)
}

// 切换基本认证规则的生效范围
const setBasicAuthScope = (auth: any, scope: string) => {
  auth._scope = scope
  auth.path = scope === 'site' ? '/' : ''
}

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
  <app-modal v-model:show="show" size="lg" :title="title">
    <n-spin :show="loading">
      <n-tabs v-model:value="current" type="line" animated>
        <n-tab-pane name="listen" :tab="$gettext('Domain & Listening')">
          <n-form v-if="setting">
            <n-form-item :label="$gettext('Domain')">
              <ListInput
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
                  <div class="flex flex-wrap gap-x-5 gap-y-2 w-full items-center">
                    <n-input v-model:value="value.address" clearable class="md:flex-1" />
                    <n-checkbox
                      :checked="hasArg(value.args, 'ssl')"
                      @update:checked="(checked: boolean) => toggleArg(value.args, 'ssl', checked)"
                      class="md:!w-30"
                    >
                      HTTPS
                    </n-checkbox>
                    <n-checkbox
                      v-if="features.quic"
                      :checked="hasArg(value.args, 'quic')"
                      @update:checked="(checked: boolean) => toggleArg(value.args, 'quic', checked)"
                      class="md:!w-50"
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
              <n-input-group>
                <n-input
                  v-model:value="setting.path"
                  :placeholder="$gettext('Enter website directory (absolute path)')"
                />
                <n-button @click="handleSelectPath('path')">
                  <template #icon>
                    <i-mdi-folder-open />
                  </template>
                </n-button>
              </n-input-group>
            </n-form-item>
            <n-form-item :label="$gettext('Running Directory')">
              <n-input-group>
                <n-input
                  v-model:value="setting.root"
                  :placeholder="
                    $gettext('Enter running directory (needed for Laravel etc.) (absolute path)')
                  "
                />
                <n-button @click="handleSelectPath('root')">
                  <template #icon>
                    <i-mdi-folder-open />
                  </template>
                </n-button>
              </n-input-group>
            </n-form-item>
            <n-form-item :label="$gettext('Default Document')">
              <n-dynamic-tags v-model:value="setting.index" />
            </n-form-item>
            <n-form-item v-if="setting.type == 'php'" :label="$gettext('PHP Version')">
              <n-select
                :value="setting.php || null"
                :options="installedEnvironment.php"
                :placeholder="$gettext('Select PHP Version')"
                clearable
                @update:value="(v: number | null) => (setting.php = v ?? 0)"
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
                        class="md:!w-50"
                      />
                    </n-flex>
                  </template>
                  <n-form v-bind="formLayout">
                    <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
                      <n-form-item-gi :span="12" :label="$gettext('Load Balancing Algorithm')">
                        <n-select v-model:value="upstream.algo" :options="upstreamAlgoOptions" />
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
                      <n-form-item-gi
                        v-if="features.resolver"
                        :span="12"
                        :label="$gettext('DNS Resolver')"
                      >
                        <n-dynamic-tags
                          v-model:value="upstream.resolver"
                          :placeholder="$gettext('e.g., 8.8.8.8')"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        v-if="features.resolver && upstream.resolver?.length"
                        :span="12"
                        :label="$gettext('Resolver Timeout')"
                      >
                        <n-input-group>
                          <n-input-number
                            :value="
                              getDurationDisplay(
                                upstream.resolver_timeout,
                                getUnitOf(upstream, 'resolver_timeout'),
                              )
                            "
                            :min="1"
                            :max="3600"
                            flex-1
                            @update:value="
                              (v: number | null) => updateUpstreamTimeoutValue(upstream, v ?? 0)
                            "
                          />
                          <n-select
                            :value="getUnitOf(upstream, 'resolver_timeout')"
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
                  <n-form v-bind="formLayout">
                    <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
                      <n-form-item-gi
                        v-if="features.matchType"
                        :span="12"
                        :label="$gettext('Match Type')"
                      >
                        <n-select
                          :value="parseLocation(proxy.location).type"
                          :options="locationMatchTypes"
                          @update:value="(v: string) => updateLocationType(proxy, v)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        :span="features.matchType ? 12 : 24"
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
                      <n-form-item-gi
                        v-if="features.resolver"
                        :span="12"
                        :label="$gettext('DNS Resolver')"
                      >
                        <n-dynamic-tags
                          v-model:value="proxy.resolver"
                          :placeholder="$gettext('e.g., 8.8.8.8')"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi
                        v-if="features.resolver && proxy.resolver.length"
                        :span="12"
                        :label="$gettext('Resolver Timeout')"
                      >
                        <n-input-group>
                          <n-input-number
                            :value="
                              getDurationDisplay(
                                proxy.resolver_timeout,
                                getUnitOf(proxy, 'resolver_timeout'),
                              )
                            "
                            :min="1"
                            :max="3600"
                            flex-1
                            @update:value="(v: number | null) => updateTimeoutValue(proxy, v ?? 0)"
                          />
                          <n-select
                            :value="getUnitOf(proxy, 'resolver_timeout')"
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
                        v-if="features.proxyCache && isCacheEnabled(proxy)"
                        :title="$gettext('Cache Settings')"
                        name="cache"
                      >
                        <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
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
                      <n-collapse-item
                        v-if="features.proxyAdvanced"
                        :title="$gettext('Advanced Settings')"
                        name="advanced"
                      >
                        <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
                          <!-- HTTP 协议版本 -->
                          <n-form-item-gi :span="12" :label="$gettext('HTTP Version')">
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
                          <n-form-item-gi :span="12" :label="$gettext('Max Body Size')">
                            <n-input-group>
                              <n-input-number
                                :value="getClientMaxBodySizeValue(proxy)"
                                :min="0"
                                flex-1
                                :placeholder="$gettext('Use global')"
                                @update:value="
                                  (v: number | null) => updateClientMaxBodySizeValue(proxy, v ?? 0)
                                "
                              />
                              <n-select
                                :value="proxy._size_unit || 'm'"
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
                          <n-form-item-gi :span="12" :label="$gettext('Timeout Settings')">
                            <n-switch
                              :value="proxy.timeout !== null"
                              @update:value="(v: boolean) => toggleProxyTimeout(proxy, v)"
                            />
                          </n-form-item-gi>
                        </n-grid>

                        <!-- 超时配置详情 -->
                        <template v-if="proxy.timeout">
                          <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
                            <n-form-item-gi :span="12" :label="$gettext('Connect Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="
                                    getDurationDisplay(
                                      proxy.timeout.connect,
                                      getUnitOf(proxy, 'timeout_connect'),
                                    )
                                  "
                                  :min="1"
                                  flex-1
                                  @update:value="
                                    (v: number | null) =>
                                      updateProxyTimeoutValue(proxy, 'connect', v ?? 0)
                                  "
                                />
                                <n-select
                                  :value="getUnitOf(proxy, 'timeout_connect')"
                                  :options="timeUnitOptions"
                                  class="w-25"
                                  @update:value="
                                    (v: string) => updateProxyTimeoutUnit(proxy, 'connect', v)
                                  "
                                />
                              </n-input-group>
                            </n-form-item-gi>
                            <n-form-item-gi :span="12" :label="$gettext('Read Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="
                                    getDurationDisplay(
                                      proxy.timeout.read,
                                      getUnitOf(proxy, 'timeout_read'),
                                    )
                                  "
                                  :min="1"
                                  flex-1
                                  @update:value="
                                    (v: number | null) =>
                                      updateProxyTimeoutValue(proxy, 'read', v ?? 0)
                                  "
                                />
                                <n-select
                                  :value="getUnitOf(proxy, 'timeout_read')"
                                  :options="timeUnitOptions"
                                  class="w-25"
                                  @update:value="
                                    (v: string) => updateProxyTimeoutUnit(proxy, 'read', v)
                                  "
                                />
                              </n-input-group>
                            </n-form-item-gi>
                            <n-form-item-gi :span="12" :label="$gettext('Send Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="
                                    getDurationDisplay(
                                      proxy.timeout.send,
                                      getUnitOf(proxy, 'timeout_send'),
                                    )
                                  "
                                  :min="1"
                                  flex-1
                                  @update:value="
                                    (v: number | null) =>
                                      updateProxyTimeoutValue(proxy, 'send', v ?? 0)
                                  "
                                />
                                <n-select
                                  :value="getUnitOf(proxy, 'timeout_send')"
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

                        <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
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
                          <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
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
                            <n-form-item-gi :span="12" :label="$gettext('Max Tries')">
                              <n-input-number
                                :value="proxy.retry.tries || null"
                                :min="0"
                                :placeholder="$gettext('Unlimited')"
                                @update:value="(v: number | null) => (proxy.retry.tries = v ?? 0)"
                              />
                            </n-form-item-gi>
                            <n-form-item-gi :span="12" :label="$gettext('Retry Timeout')">
                              <n-input-group>
                                <n-input-number
                                  :value="
                                    getDurationDisplay(
                                      proxy.retry.timeout,
                                      getUnitOf(proxy, 'retry_timeout'),
                                    )
                                  "
                                  :min="0"
                                  flex-1
                                  :placeholder="$gettext('Unlimited')"
                                  @update:value="
                                    (v: number | null) => updateRetryTimeoutValue(proxy, v ?? 0)
                                  "
                                />
                                <n-select
                                  :value="getUnitOf(proxy, 'retry_timeout')"
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
                          <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
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

                        <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
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
                          <common-editor
                            v-model:value="proxy.custom"
                            height="20vh"
                            :lang="features.lang"
                          />
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
              <n-descriptions :column="isMobile ? 1 : 3">
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
              <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
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
              <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
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
              <n-grid cols="1 m:2" :x-gap="24" responsive="screen">
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
            <n-form v-if="features.rewritePresets" v-bind="presetFormLayout">
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
                  <n-form v-bind="formLayout">
                    <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
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
            <n-collapse-item :title="$gettext('Website Type')" name="website_type">
              <n-form v-bind="formLayout">
                <n-form-item :label="$gettext('Current Website Type')">
                  <n-tag>{{ websiteTypeLabel(setting.type) }}</n-tag>
                </n-form-item>
                <n-form-item :label="$gettext('Target Website Type')">
                  <n-select
                    v-model:value="targetType"
                    :options="websiteTypeOptions"
                    :placeholder="$gettext('Select Website Type')"
                  />
                </n-form-item>
                <n-form-item v-if="targetType === 'php'" :label="$gettext('PHP Version')">
                  <n-select
                    v-model:value="targetPHP"
                    :options="installedEnvironment.php"
                    :placeholder="$gettext('Select PHP Version')"
                  />
                </n-form-item>
                <n-form-item v-if="targetType === 'proxy'" :label="$gettext('Proxy Target')">
                  <n-input
                    v-model:value="targetProxy"
                    :placeholder="$gettext('For example: http://127.0.0.1:3000')"
                  />
                </n-form-item>
                <n-alert type="warning" mb-4>
                  {{
                    $gettext(
                      'After switching, common settings and website files will be preserved. The original type-specific configuration will be deleted and rebuilt for the new type.',
                    )
                  }}
                </n-alert>
                <n-button
                  type="warning"
                  :loading="switchTypeLoading"
                  :disabled="!canSwitchType || switchTypeLoading || loading"
                  @click="handleSwitchType"
                >
                  {{ $gettext('Switch Website Type') }}
                </n-button>
              </n-form>
            </n-collapse-item>

            <!-- 访问统计 -->
            <n-collapse-item
              v-if="features.stat"
              :title="$gettext('Access Statistics')"
              name="stat_settings"
            >
              <n-form v-bind="formLayout">
                <n-form-item :label="$gettext('Enable Statistics')">
                  <n-switch v-model:value="setting.stat_enabled" />
                </n-form-item>
              </n-form>
            </n-collapse-item>

            <!-- LiteSpeed 页面缓存 -->
            <n-collapse-item v-if="features.lsCache" title="LSCache" name="lscache_settings">
              <n-alert type="info" mb-4>
                {{
                  $gettext(
                    'Enables the LiteSpeed cache module for this site. Pages are cached only when the application asks for it, e.g. WordPress with the LiteSpeed Cache plugin.',
                  )
                }}
              </n-alert>
              <n-form v-bind="formLayout">
                <n-form-item :label="$gettext('Enable LSCache')">
                  <n-switch v-model:value="setting.lscache" />
                </n-form-item>
              </n-form>
            </n-collapse-item>

            <!-- 日志设置 -->
            <n-collapse-item :title="$gettext('Log Settings')" name="log_settings">
              <n-form v-bind="formLayout">
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
            <n-collapse-item
              v-if="features.rateLimit"
              :title="$gettext('Rate Limiting')"
              name="rate_limit"
            >
              <n-form v-bind="formLayout">
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
            <n-collapse-item v-if="features.realIP" :title="$gettext('Real IP')" name="real_ip">
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
              <n-form v-bind="formLayout">
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
              <n-flex vertical>
                <n-card
                  v-for="(auth, index) in setting.basic_auth as any[]"
                  :key="auth._id"
                  closable
                  @close="removeBasicAuth(index)"
                >
                  <n-form v-bind="formLayout">
                    <n-form-item :label="$gettext('Scope')">
                      <n-radio-group
                        :value="auth._scope"
                        @update:value="(v: string) => setBasicAuthScope(auth, v)"
                      >
                        <n-radio value="site" :disabled="hasSiteBasicAuth(auth)">
                          {{ $gettext('Entire Website') }}
                        </n-radio>
                        <n-radio value="dir">{{ $gettext('Specific Directory') }}</n-radio>
                      </n-radio-group>
                    </n-form-item>
                    <n-form-item v-if="auth._scope === 'dir'" :label="$gettext('Directory')">
                      <n-input v-model:value="auth.path" placeholder="/admin" />
                    </n-form-item>
                    <n-form-item :label="$gettext('User Credentials')">
                      <key-value-editor
                        v-model="auth.users"
                        :key-placeholder="$gettext('Username')"
                        :value-placeholder="$gettext('Password')"
                        :add-button-text="$gettext('Add User')"
                        default-key-prefix="user"
                        value-type="password"
                        :show-password-toggle="true"
                      />
                    </n-form-item>
                  </n-form>
                </n-card>
                <n-button type="primary" dashed @click="addBasicAuth">
                  {{ $gettext('Add Rule') }}
                </n-button>
                <n-alert v-if="setting.basic_auth.length > 0" type="info">
                  {{
                    $gettext(
                      'Visitors will need to enter a username and password to access the protected directories.',
                    )
                  }}
                </n-alert>
              </n-flex>
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
                  <n-form v-bind="configFormLayout">
                    <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
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
                        :lang="features.lang"
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
  </app-modal>
  <app-modal v-model:show="dnsModal" :title="$gettext('Select DNS Provider')">
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
  </app-modal>
  <path-selector
    v-model:show="showPathSelector"
    v-model:path="setting[pathSelectorTarget]"
    :dir="true"
  />
</template>

<style scoped>
/* 拖拽时的占位卡片 */
:deep(.ghost-card) {
  opacity: 0.5;
}
</style>

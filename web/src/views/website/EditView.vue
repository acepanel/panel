<script setup lang="ts">
defineOptions({
  name: 'website-edit'
})

import type { MessageReactive } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'
import draggable from 'vuedraggable'

import cert from '@/api/panel/cert'
import home from '@/api/panel/home'
import website from '@/api/panel/website'

const { $gettext } = useGettext()
let messageReactive: MessageReactive | null = null

const current = ref('listen')
const route = useRoute()
const { id } = route.params
const { data: setting, send: fetchSetting } = useRequest(website.config(Number(id)), {
  initialData: {
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
    ssl_ciphers: '',
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
    basic_auth: {},
    custom_configs: []
  }
})
const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    webserver: 'nginx',
    php: [
      {
        label: $gettext('Not used'),
        value: 0
      }
    ],
    db: [
      {
        label: '',
        value: ''
      }
    ]
  }
})

// 是否为 Nginx
const isNginx = computed(() => installedEnvironment.value.webserver === 'nginx')
const certs = ref<any>([])
useRequest(cert.certs(1, 10000)).onSuccess(({ data }) => {
  certs.value = data.items
})
const { data: rewrites } = useRequest(website.rewrites, {
  initialData: {}
})
const rewriteOptions = computed(() => {
  return Object.keys(rewrites.value).map((key) => ({
    label: key,
    value: key
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
    value: item.id
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
      args
    })
  }
  // 如果关闭了ssl，自动禁用所有ssl和quic
  if (!setting.value.ssl) {
    setting.value.listens = setting.value.listens.filter((item: any) => item.address !== '443') // 443直接删掉
    setting.value.listens.forEach((item: any) => {
      item.args = []
    })
  }

  useRequest(website.saveConfig(Number(id), setting.value)).onSuccess(() => {
    fetchSetting()
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleReset = () => {
  useRequest(website.resetConfig(Number(id))).onSuccess(() => {
    fetchSetting()
    window.$message.success($gettext('Reset successfully'))
  })
}

const handleRewrite = (value: string) => {
  setting.value.rewrite = rewrites.value[value] || ''
}

const isObtainCert = ref(false)
const handleObtainCert = () => {
  isObtainCert.value = true
  messageReactive = window.$message.loading($gettext('Please wait...'), {
    duration: 0
  })
  useRequest(website.obtainCert(Number(id)))
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
    setting.value.ssl_certificate = cert.cert
    setting.value.ssl_certificate_key = cert.key
  } else {
    window.$message.error($gettext('The selected certificate is invalid'))
  }
}

const clearLog = async () => {
  useRequest(website.clearLog(Number(id))).onSuccess(() => {
    fetchSetting()
    window.$message.success($gettext('Cleared successfully'))
  })
}

const onCreateListen = () => {
  return {
    address: '',
    args: []
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

// ========== Upstreams 相关 ==========
// 添加新的上游
const addUpstream = () => {
  const name = `${setting.value.name.replace(/-/g, '_')}_upstream_${(setting.value.upstreams?.length || 0) + 1}`
  if (!setting.value.upstreams) {
    setting.value.upstreams = []
  }
  setting.value.upstreams.push({
    name,
    servers: {},
    algo: '',
    keepalive: 32,
    resolver: [],
    resolver_timeout: 5 * 1000000000 // 5秒，以纳秒为单位
  })
}

// 删除上游
const removeUpstream = (index: number) => {
  if (setting.value.upstreams) {
    setting.value.upstreams.splice(index, 1)
  }
}

// 为上游添加服务器
const addServerToUpstream = (index: number) => {
  const upstream = setting.value.upstreams[index]
  if (!upstream.servers) {
    upstream.servers = {}
  }
  upstream.servers[`127.0.0.1:${8080 + Object.keys(upstream.servers).length}`] = ''
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
  { label: $gettext('Case-insensitive Regex (~*)'), value: '~*' }
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
    location: '/',
    pass: 'http://127.0.0.1:8080',
    host: '$host',
    sni: '',
    cache: false,
    buffering: true,
    resolver: [],
    resolver_timeout: 5 * 1000000000, // 5秒，以纳秒为单位
    replaces: {}
  })
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
  { label: $gettext('Hours'), value: 'h' }
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
  { label: $gettext('404 Redirect'), value: '404' }
]

// 状态码选项
const redirectStatusCodeOptions = [
  { label: '301 - ' + $gettext('Moved Permanently'), value: 301 },
  { label: '302 - ' + $gettext('Found'), value: 302 },
  { label: '307 - ' + $gettext('Temporary Redirect'), value: 307 },
  { label: '308 - ' + $gettext('Permanent Redirect'), value: 308 }
]

// 添加重定向规则
const addRedirect = () => {
  if (!setting.value.redirects) {
    setting.value.redirects = []
  }
  setting.value.redirects.push({
    type: 'url',
    from: '/',
    to: '/new',
    keep_uri: true,
    status_code: 308
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

// ========== 高级设置相关（限流限速、基本认证）==========
// 限流限速是否启用
const rateLimitEnabled = computed({
  get: () => setting.value.rate_limit !== null,
  set: (value: boolean) => {
    if (value) {
      setting.value.rate_limit = {
        rate: '',
        concurrent: 0,
        zone: {}
      }
    } else {
      setting.value.rate_limit = null
    }
  }
})

// 添加基本认证用户
const addBasicAuthUser = () => {
  if (!setting.value.basic_auth) {
    setting.value.basic_auth = {}
  }
  // 使用时间戳确保用户名唯一
  let index = Date.now()
  let username = `user${index}`
  // 确保用户名不重复
  while (setting.value.basic_auth[username] !== undefined) {
    index++
    username = `user${index}`
  }
  setting.value.basic_auth[username] = ''
}

// 删除基本认证用户
const removeBasicAuthUser = (username: string) => {
  if (setting.value.basic_auth) {
    delete setting.value.basic_auth[username]
  }
}

// ========== 自定义配置相关 ==========
// 作用域选项
const scopeOptions = [
  { label: $gettext('This Website'), value: 'site' },
  { label: $gettext('Global'), value: 'shared' }
]

// 添加自定义配置
const addCustomConfig = () => {
  if (!setting.value.custom_configs) {
    setting.value.custom_configs = []
  }
  const index = setting.value.custom_configs.length + 1
  setting.value.custom_configs.push({
    name: `custom_${index}`,
    scope: 'site',
    content: ''
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
  <common-page show-footer :title="title">
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
                    @update:checked="(checked) => toggleArg(value.args, 'ssl', checked)"
                    ml-20
                    mr-20
                    w-120
                  >
                    HTTPS
                  </n-checkbox>
                  <n-checkbox
                    v-if="isNginx"
                    :checked="hasArg(value.args, 'quic')"
                    @update:checked="(checked) => toggleArg(value.args, 'quic', checked)"
                    w-200
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
            item-key="name"
            handle=".drag-handle"
            :animation="200"
            ghost-class="ghost-card"
          >
            <template #item="{ element: upstream, index }">
              <n-card closable @close="removeUpstream(index)" style="margin-bottom: 16px">
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
                      style="width: 200px"
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
                                { label: 'random', value: 'random' }
                              ]
                            : [
                                { label: $gettext('Round Robin (default)'), value: '' },
                                { label: $gettext('Least Busy'), value: 'bybusyness' },
                                { label: $gettext('By Traffic'), value: 'bytraffic' }
                              ]
                        "
                      />
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" :label="$gettext('Keepalive Connections')">
                      <n-input-number
                        v-model:value="upstream.keepalive"
                        :min="0"
                        :max="1000"
                        style="width: 100%"
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
                          style="flex: 1"
                          @update:value="
                            (v: number | null) => updateUpstreamTimeoutValue(upstream, v ?? 5)
                          "
                        />
                        <n-select
                          :value="parseDuration(upstream.resolver_timeout).unit"
                          :options="timeUnitOptions"
                          style="width: 100px"
                          @update:value="(v: string) => updateUpstreamTimeoutUnit(upstream, v)"
                        />
                      </n-input-group>
                    </n-form-item-gi>
                  </n-grid>
                  <n-form-item :label="$gettext('Backend Servers')">
                    <n-flex vertical :size="8" style="width: 100%">
                      <n-flex
                        v-for="(options, address) in upstream.servers"
                        :key="String(address)"
                        :size="8"
                        align="center"
                      >
                        <n-input
                          :default-value="String(address)"
                          :placeholder="$gettext('Server address, e.g., 127.0.0.1:8080')"
                          style="flex: 1"
                          @change="
                            (newAddr: string) => {
                              const oldAddr = String(address)
                              if (newAddr && newAddr !== oldAddr) {
                                upstream.servers[newAddr] = upstream.servers[oldAddr]
                                delete upstream.servers[oldAddr]
                              }
                            }
                          "
                        />
                        <n-input
                          :value="String(options)"
                          :placeholder="$gettext('Options, e.g., weight=5 backup')"
                          style="flex: 1"
                          @update:value="(v: string) => (upstream.servers[String(address)] = v)"
                        />
                        <n-button
                          type="error"
                          secondary
                          size="small"
                          style="flex-shrink: 0"
                          @click="delete upstream.servers[String(address)]"
                        >
                          {{ $gettext('Remove') }}
                        </n-button>
                      </n-flex>
                      <n-button dashed size="small" @click="addServerToUpstream(index)">
                        {{ $gettext('Add Server') }}
                      </n-button>
                    </n-flex>
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
          <n-button type="primary" dashed @click="addUpstream" mb-20>
            {{ $gettext('Add Upstream') }}
          </n-button>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane v-if="setting.type === 'proxy'" name="proxies" :tab="$gettext('Proxies')">
        <n-flex vertical>
          <!-- 代理卡片列表 -->
          <draggable
            v-model="setting.proxies"
            item-key="location"
            handle=".drag-handle"
            :animation="200"
            ghost-class="ghost-card"
          >
            <template #item="{ element: proxy, index }">
              <n-card closable @close="removeProxy(index)" style="margin-bottom: 16px">
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
                    <n-form-item-gi :span="isNginx ? 12 : 24" :label="$gettext('Match Expression')">
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
                            'Backend address, e.g., http://127.0.0.1:8080 or http://upstream_name'
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
                      <n-switch v-model:value="proxy.cache" />
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
                          style="flex: 1"
                          @update:value="(v: number | null) => updateTimeoutValue(proxy, v ?? 5)"
                        />
                        <n-select
                          :value="parseDuration(proxy.resolver_timeout).unit"
                          :options="timeUnitOptions"
                          style="width: 100px"
                          @update:value="(v: string) => updateTimeoutUnit(proxy, v)"
                        />
                      </n-input-group>
                    </n-form-item-gi>
                  </n-grid>
                  <n-divider>{{ $gettext('Response Content Replacement') }}</n-divider>
                  <n-flex vertical :size="8">
                    <n-flex
                      v-for="(toValue, fromValue) in proxy.replaces"
                      :key="String(fromValue)"
                      :size="8"
                      align="center"
                    >
                      <n-input
                        :value="String(fromValue)"
                        :placeholder="$gettext('Original content')"
                        style="flex: 1"
                        @blur="
                          (e: FocusEvent) => {
                            const newFrom = (e.target as HTMLInputElement).value
                            const oldFrom = String(fromValue)
                            if (newFrom && newFrom !== oldFrom) {
                              proxy.replaces[newFrom] = proxy.replaces[oldFrom]
                              delete proxy.replaces[oldFrom]
                            }
                          }
                        "
                      />
                      <span style="flex-shrink: 0">=></span>
                      <n-input
                        :value="String(toValue)"
                        :placeholder="$gettext('Replacement content')"
                        style="flex: 1"
                        @update:value="(v: string) => (proxy.replaces[String(fromValue)] = v)"
                      />
                      <n-button
                        type="error"
                        secondary
                        size="small"
                        style="flex-shrink: 0"
                        @click="delete proxy.replaces[String(fromValue)]"
                      >
                        {{ $gettext('Remove') }}
                      </n-button>
                    </n-flex>
                    <n-button
                      dashed
                      size="small"
                      @click="
                        () => {
                          if (!proxy.replaces) proxy.replaces = {}
                          proxy.replaces[`/old_${Object.keys(proxy.replaces).length}`] = '/new'
                        }
                      "
                    >
                      {{ $gettext('Add Replacement Rule') }}
                    </n-button>
                  </n-flex>
                </n-form>
              </n-card>
            </template>
          </draggable>

          <!-- 空状态 -->
          <n-empty v-if="!setting.proxies || setting.proxies.length === 0">
            {{ $gettext('No proxy rules configured') }}
          </n-empty>

          <!-- 添加按钮 -->
          <n-button type="primary" dashed @click="addProxy" mb-20>
            {{ $gettext('Add Proxy Rule') }}
          </n-button>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="https" tab="HTTPS">
        <n-flex vertical v-if="setting">
          <n-card v-if="setting.ssl && setting.ssl_issuer != ''">
            <n-descriptions :title="$gettext('Certificate Information')" :column="2">
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
              <n-descriptions-item>
                <template #label>OCSP</template>
                <n-flex>
                  <n-tag v-for="item in setting.ssl_ocsp_server" :key="item">{{ item }}</n-tag>
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
          <n-form inline v-if="setting.ssl">
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
          <n-form v-if="setting.ssl">
            <n-form-item :label="$gettext('TLS Version')">
              <n-select
                v-model:value="setting.ssl_protocols"
                :options="[
                  { label: 'TLS 1.0', value: 'TLSv1.0' },
                  { label: 'TLS 1.1', value: 'TLSv1.1' },
                  { label: 'TLS 1.2', value: 'TLSv1.2' },
                  { label: 'TLS 1.3', value: 'TLSv1.3' }
                ]"
                multiple
              />
            </n-form-item>
            <n-form-item :label="$gettext('Cipher Suites')">
              <n-input
                type="textarea"
                v-model:value="setting.ssl_ciphers"
                :placeholder="$gettext('Enter the cipher suite, leave blank to reset to default')"
                rows="4"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Certificate')">
              <n-input
                v-model:value="setting.ssl_cert"
                type="textarea"
                :placeholder="$gettext('Enter the content of the PEM certificate file')"
                :autosize="{ minRows: 10, maxRows: 15 }"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Private Key')">
              <n-input
                v-model:value="setting.ssl_key"
                type="textarea"
                :placeholder="$gettext('Enter the content of the KEY private key file')"
                :autosize="{ minRows: 10, maxRows: 15 }"
              />
            </n-form-item>
          </n-form>
        </n-flex>
        <n-skeleton v-else text :repeat="10" />
      </n-tab-pane>
      <n-tab-pane v-if="setting.type == 'php'" name="rewrite" :tab="$gettext('Rewrite')">
        <n-flex vertical>
          <n-form label-placement="left" label-width="auto">
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
            item-key="from"
            handle=".drag-handle"
            :animation="200"
            ghost-class="ghost-card"
          >
            <template #item="{ element: redirect, index }">
              <n-card closable @close="removeRedirect(index)" style="margin-bottom: 16px">
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
                      <n-text depth="3" class="ml-8">
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
          <n-button type="primary" dashed @click="addRedirect" mb-20>
            {{ $gettext('Add Redirect Rule') }}
          </n-button>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="advanced" :tab="$gettext('Advanced Settings')">
        <n-flex vertical>
          <!-- 限流限速设置 -->
          <n-card :title="$gettext('Rate Limiting')" style="margin-bottom: 16px">
            <n-form label-placement="left" label-width="140px">
              <n-form-item :label="$gettext('Enable Rate Limiting')">
                <n-switch v-model:value="rateLimitEnabled" />
              </n-form-item>
              <template v-if="rateLimitEnabled && setting.rate_limit">
                <n-grid :cols="24" :x-gap="16">
                  <n-form-item-gi :span="12" :label="$gettext('Rate Limit')">
                    <n-input
                      v-model:value="setting.rate_limit.rate"
                      :placeholder="$gettext('e.g., 10r/s (10 requests per second)')"
                    />
                  </n-form-item-gi>
                  <n-form-item-gi :span="12" :label="$gettext('Concurrent Connections')">
                    <n-input-number
                      v-model:value="setting.rate_limit.concurrent"
                      :min="0"
                      :max="10000"
                      :placeholder="$gettext('Max concurrent connections, 0 for unlimited')"
                      style="width: 100%"
                    />
                  </n-form-item-gi>
                </n-grid>
              </template>
            </n-form>
          </n-card>

          <!-- 基本认证设置 -->
          <n-card :title="$gettext('Basic Authentication')" style="margin-bottom: 16px">
            <n-form label-placement="left" label-width="140px">
              <n-form-item :label="$gettext('User Credentials')">
                <n-flex vertical :size="8" style="width: 100%">
                  <n-flex
                    v-for="(password, username) in setting.basic_auth"
                    :key="String(username)"
                    :size="8"
                    align="center"
                  >
                    <n-input
                      :default-value="String(username)"
                      :placeholder="$gettext('Username')"
                      style="flex: 1"
                      @change="
                        (newUsername: string) => {
                          const oldUsername = String(username)
                          if (newUsername && newUsername !== oldUsername) {
                            // 检查新用户名是否已存在
                            if (setting.basic_auth[newUsername] !== undefined) {
                              window.$message.error($gettext('Username already exists'))
                              return
                            }
                            setting.basic_auth[newUsername] = setting.basic_auth[oldUsername]
                            delete setting.basic_auth[oldUsername]
                          }
                        }
                      "
                    />
                    <n-input
                      :value="String(password)"
                      type="password"
                      show-password-on="click"
                      :placeholder="$gettext('Password')"
                      style="flex: 1"
                      @update:value="(v: string) => (setting.basic_auth[String(username)] = v)"
                    />
                    <n-button
                      type="error"
                      secondary
                      size="small"
                      style="flex-shrink: 0"
                      @click="removeBasicAuthUser(String(username))"
                    >
                      {{ $gettext('Remove') }}
                    </n-button>
                  </n-flex>
                  <n-button dashed size="small" @click="addBasicAuthUser">
                    {{ $gettext('Add User') }}
                  </n-button>
                </n-flex>
              </n-form-item>
            </n-form>
            <n-alert v-if="Object.keys(setting.basic_auth || {}).length > 0" type="info">
              {{ $gettext('Visitors will need to enter a username and password to access this website.') }}
            </n-alert>
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="custom_configs" :tab="$gettext('Custom Configs')">
        <n-flex vertical>
          <!-- 自定义配置列表 -->
          <draggable
            v-model="setting.custom_configs"
            item-key="name"
            handle=".drag-handle"
            :animation="200"
            ghost-class="ghost-card"
          >
            <template #item="{ element: config, index }">
              <n-card closable @close="removeCustomConfig(index)" style="margin-bottom: 16px">
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
                      <n-select v-model:value="config.scope" :options="scopeOptions" />
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
          <n-button type="primary" dashed @click="addCustomConfig" mb-20>
            {{ $gettext('Add Custom Config') }}
          </n-button>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="log" :tab="$gettext('Access Log')">
        <n-flex vertical>
          <n-flex flex items-center>
            <n-alert type="warning" w-full>
              {{ $gettext('All logs can be viewed by downloading the file') }}
              <n-tag>{{ setting.access_log }}</n-tag>
              {{ $gettext('view') }}.
            </n-alert>
          </n-flex>
          <realtime-log :path="setting.access_log" language="accesslog" pb-20 />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="error_log" :tab="$gettext('Error Log')">
        <n-flex vertical>
          <n-flex flex items-center>
            <n-alert type="warning" w-full>
              {{ $gettext('All logs can be viewed by downloading the file') }}
              <n-tag>{{ setting.error_log }}</n-tag>
              {{ $gettext('view') }}.
            </n-alert>
          </n-flex>
          <realtime-log :path="setting.error_log" language="accesslog" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>
    <n-button
      v-if="current !== 'log' && current !== 'error_log'"
      type="primary"
      @click="handleSave"
    >
      {{ $gettext('Save') }}
    </n-button>
    <n-popconfirm v-if="current == 'log'" @positive-click="clearLog">
      <template #trigger>
        <n-button type="primary">
          {{ $gettext('Clear Logs') }}
        </n-button>
      </template>
      {{ $gettext('Are you sure you want to clear?') }}
    </n-popconfirm>
    <n-button
      v-if="current === 'https' && setting && setting.domains.length > 0"
      :loading="isObtainCert"
      :disabled="isObtainCert"
      class="ml-16"
      type="info"
      @click="handleObtainCert"
    >
      {{ $gettext('One-click Certificate Issuance') }}
    </n-button>
    <n-popconfirm v-if="current === 'config'" @positive-click="handleReset">
      <template #trigger>
        <n-button type="warning" ml-16>
          {{ $gettext('Reset Configuration') }}
        </n-button>
      </template>
      {{ $gettext('Are you sure you want to reset the configuration?') }}
    </n-popconfirm>
  </common-page>
</template>

<style scoped>
/* 拖拽时的占位卡片 */
:deep(.ghost-card) {
  opacity: 0.5;
}
</style>

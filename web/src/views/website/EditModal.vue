<script setup lang="ts">
import type { MessageReactive } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'
import draggable from 'vuedraggable'

import cert from '@/api/panel/cert'
import file from '@/api/panel/file'
import home from '@/api/panel/home'
import website from '@/api/panel/website'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { webserverFeatures } from '@/utils'

import AdvancedTab from './edit/AdvancedTab.vue'
import CertSummary from './edit/CertSummary.vue'
import { generateId } from './edit/id'
import ProxyCard from './edit/ProxyCard.vue'
import UpstreamCard from './edit/UpstreamCard.vue'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const editId = defineModel<number>('editId', { type: Number, required: true })

const { $gettext } = useGettext()
const formLayout = useFormLayout(140)
const presetFormLayout = useFormLayout('auto')
const configFormLayout = useFormLayout(100)
let messageReactive: MessageReactive | null = null

const current = ref('listen')
const loading = ref(false)
const switchTypeLoading = ref(false)
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
  basic_auth: [],
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
    selectedCert.value = null
    rewriteValue.value = null
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
const tlsVersionLabels: Record<string, string> = {
  TLSv1: 'TLS 1.0',
  'TLSv1.1': 'TLS 1.1',
  'TLSv1.2': 'TLS 1.2',
  'TLSv1.3': 'TLS 1.3',
}
const tlsVersionOptions = computed(() =>
  features.value.tlsVersions.map((value) => ({ label: tlsVersionLabels[value], value })),
)
// 上游名称，代理据此判断目标是否为本站上游
const upstreamNames = computed<string[]>(
  () => setting.value.upstreams?.map((u: any) => u.name) ?? [],
)
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
const title = computed(() => $gettext('Edit Website - %{ name }', { name: setting.value.name }))
const certOptions = computed(() => {
  return certs.value.map((item: any) => ({
    label: item.domains.join(', '),
    value: item.id,
  }))
})
const selectedCert = ref(null)

const handleSave = async () => {
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

  // 清理真实 IP 来源里的空行与首尾空白
  if (setting.value.real_ip) {
    setting.value.real_ip.from = (setting.value.real_ip.from ?? [])
      .map((s: string) => s.trim())
      .filter((s: string) => s.length > 0)
  }

  await website.saveConfig(id.value, setting.value)
  fetchSetting()
  window.$message.success($gettext('Saved successfully'))
  window.$bus.emit('website:refresh')
}

const handleSwitchType = (target: any) => {
  switchTypeLoading.value = true
  useRequest(website.switchType(id.value, target))
    .onSuccess(() => {
      fetchSetting()
      window.$bus.emit('website:refresh')
      window.$message.success($gettext('Website type switched successfully'))
    })
    .onComplete(() => {
      switchTypeLoading.value = false
    })
}

const handleReset = async () => {
  await website.resetConfig(id.value)
  fetchSetting()
  window.$bus.emit('website:refresh')
  window.$message.success($gettext('Reset successfully'))
}

const handleRewrite = (value: string) => {
  setting.value.rewrite = rewrites.value[value] || ''
}

const isObtainCert = ref(false)
const dnsModal = ref(false)
const dnsList = ref<any>([])
const selectedDnsId = ref<number | null>(null)

const handleObtainCert = async () => {
  // 检测泛域名
  const hasWildcard = setting.value.domains?.some((domain: string) => domain.includes('*'))
  if (hasWildcard) {
    // 加载 DNS 列表并弹窗选择
    const data = await cert.dns(1, 10000)
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
  await file.truncate(path)
  if (isError) errorLogRef.value?.clear()
  else accessLogRef.value?.clear()
  window.$message.success($gettext('Cleared successfully'))
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
  if (!Array.isArray(setting.value.basic_auth)) setting.value.basic_auth = []
  setting.value.basic_auth.forEach((item: any) => {
    if (!item._id) item._id = generateId()
    if (!item._scope) item._scope = !item.path || item.path === '/' ? 'site' : 'dir'
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
    keepalive: features.value.upstreamKeepalive ? 32 : 0,
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

// ========== Proxies 相关 ==========
// 添加新的代理
const addProxy = () => {
  if (!setting.value.proxies) {
    setting.value.proxies = []
  }
  setting.value.proxies.push({
    _id: generateId(),
    location: '^~ /',
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

// 删除代理
const removeProxy = (index: number) => {
  if (setting.value.proxies) {
    setting.value.proxies.splice(index, 1)
  }
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
          <n-form>
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
        </n-tab-pane>
        <n-tab-pane name="basic" :tab="$gettext('Basic Settings')">
          <n-form>
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
        </n-tab-pane>
        <n-tab-pane name="upstreams" :tab="$gettext('Upstreams')">
          <n-flex vertical>
            <!-- 上游卡片列表 -->
            <draggable
              v-model="setting.upstreams"
              item-key="_id"
              handle=".drag-handle"
              :animation="200"
              ghost-class="ghost-card"
            >
              <template #item="{ index }">
                <upstream-card
                  v-model:upstream="setting.upstreams[index]"
                  :features="features"
                  @remove="removeUpstream(index)"
                />
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
        <n-tab-pane name="proxies" :tab="$gettext('Proxies')">
          <n-flex vertical>
            <!-- 代理卡片列表 -->
            <draggable
              v-model="setting.proxies"
              item-key="_id"
              handle=".drag-handle"
              :animation="200"
              ghost-class="ghost-card"
            >
              <template #item="{ index }">
                <proxy-card
                  v-model:proxy="setting.proxies[index]"
                  :index="index"
                  :features="features"
                  :upstream-names="upstreamNames"
                  @remove="removeProxy(index)"
                />
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
          <n-flex vertical>
            <cert-summary
              v-if="setting.ssl && setting.ssl_issuer != ''"
              :not-before="setting.ssl_not_before"
              :not-after="setting.ssl_not_after"
              :issuer="setting.ssl_issuer"
              :domains="setting.ssl_dns_names ?? []"
            />
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
                    <n-form-item v-if="features.ocsp" :label="$gettext('OCSP Stapling')">
                      <n-switch v-model:value="setting.ocsp" />
                    </n-form-item>
                  </n-form>
                </n-gi>
                <n-form-item-gi :span="12" :label="$gettext('TLS Version')">
                  <n-select
                    v-model:value="setting.ssl_protocols"
                    :options="tlsVersionOptions"
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
        </n-tab-pane>
        <n-tab-pane v-if="setting.type == 'php'" name="rewrite" :tab="$gettext('Rewrite')">
          <n-flex vertical>
            <n-alert v-if="features.htaccess" type="info">
              {{
                $gettext(
                  'Rewrite rules are stored in the .htaccess file of the running directory, which the application itself may also modify',
                )
              }}
            </n-alert>
            <n-form v-if="rewriteOptions.length > 0" v-bind="presetFormLayout">
              <n-form-item :label="$gettext('Presets')">
                <n-select
                  v-model:value="rewriteValue"
                  clearable
                  :options="rewriteOptions"
                  @update-value="handleRewrite"
                />
              </n-form-item>
            </n-form>
            <common-editor v-model:value="setting.rewrite" height="60vh" />
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
                      <n-form-item-gi
                        v-if="redirect.type !== '404' || features.redirect404Options"
                        :span="12"
                        :label="$gettext('Status Code')"
                      >
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
                      <n-form-item-gi
                        v-if="redirect.type !== '404' || features.redirect404Options"
                        :span="12"
                        :label="$gettext('Keep URI')"
                      >
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
          <advanced-tab
            v-model:setting="setting"
            :features="features"
            :php-options="installedEnvironment.php"
            :switching="switchTypeLoading"
            :loading="loading"
            @switch-type="handleSwitchType"
          />
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
          v-if="current === 'log' || current === 'error_log'"
          type="danger"
          :content="$gettext('Are you sure you want to clear?')"
          @confirm="clearLog"
        >
          <template #trigger>
            <app-button type="primary">
              {{ $gettext('Clear Logs') }}
            </app-button>
          </template>
        </ConfirmDialog>
        <app-button
          v-if="current === 'https' && setting.domains.length > 0"
          :loading="isObtainCert"
          :disabled="isObtainCert"
          type="info"
          @click="handleObtainCert"
        >
          {{ $gettext('One-click Certificate Issuance') }}
        </app-button>
        <ConfirmDialog
          type="danger"
          :content="$gettext('Are you sure you want to reset the configuration?')"
          @confirm="handleReset"
        >
          <template #trigger>
            <app-button type="warning">
              {{ $gettext('Reset Configuration') }}
            </app-button>
          </template>
        </ConfirmDialog>
        <n-button @click="show = false">
          {{ $gettext('Cancel') }}
        </n-button>
        <app-button
          v-if="current !== 'log' && current !== 'error_log'"
          type="primary"
          @click="handleSave"
        >
          {{ $gettext('Save') }}
        </app-button>
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

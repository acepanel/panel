<script setup lang="ts">
defineOptions({
  name: 'website-edit'
})

import type { MessageReactive } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

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
    upstreams: {},
    proxies: []
  }
})
const { data: installedDbAndPhp } = useRequest(home.installedDbAndPhp, {
  initialData: {
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
  // 如果没有任何监听地址设置了https，则自动添加443
  if (setting.value.https && !setting.value.listens.some((item: any) => item.https)) {
    setting.value.listens.push({
      address: '443',
      args: ['ssl', 'quic']
    })
  }
  // 如果关闭了https，自动禁用所有https和quic
  if (!setting.value.https) {
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
              :options="installedDbAndPhp.php"
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
          <common-editor v-if="setting" v-model:content="setting.rewrite" height="60vh" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Configuration')">
        <n-flex vertical>
          <n-alert type="info" w-full>
            {{
              $gettext(
                'If you modify the original text, other modifications will not take effect after clicking save!'
              )
            }}
          </n-alert>
          <n-alert type="warning" w-full>
            {{
              $gettext(
                'If you do not understand the configuration rules, please do not modify them arbitrarily, otherwise it may cause the website to be inaccessible or panel function abnormalities! If you have already encountered a problem, try resetting the configuration!'
              )
            }}
          </n-alert>
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
            <n-popconfirm @positive-click="clearLog">
              <template #trigger>
                <n-button type="primary">
                  {{ $gettext('Clear Logs') }}
                </n-button>
              </template>
              {{ $gettext('Are you sure you want to clear?') }}
            </n-popconfirm>
          </n-flex>
          <realtime-log :path="setting.access_log" />
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
          <realtime-log :path="setting.error_log" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>
    <n-button v-if="current !== 'log'" type="primary" @click="handleSave">
      {{ $gettext('Save') }}
    </n-button>
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

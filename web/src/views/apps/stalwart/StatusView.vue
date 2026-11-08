<script setup lang="ts">
defineOptions({
  name: 'stalwart-status',
})

import { useGettext } from 'vue3-gettext'

import stalwart from '@/api/apps/stalwart'
import cert from '@/api/panel/cert'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { formatDateTime } from '@/utils'

const props = defineProps<{
  status: {
    installed: boolean
    initialized: boolean
    running: boolean
    hostname: string
    domain_count: number
    http_port: number
    log_path: string
    proxy_website_id: number
    proxy_website_name: string
    proxy_domains: string[]
    proxy_missing_domains: string[]
  }
}>()

const emit = defineEmits<{ refresh: [] }>()

const { $gettext } = useGettext()
const formLayout = useFormLayout()

const initLoading = ref(false)
const initModel = ref({
  hostname: '',
  domain: '',
})

const checkLoading = ref(false)
const checkItems = ref<any[]>([])

const tagType = (status: string) => {
  switch (status) {
    case 'ok':
      return 'success'
    case 'warn':
      return 'warning'
    case 'fail':
      return 'error'
    default:
      return 'default'
  }
}

const tagText = (status: string) => {
  switch (status) {
    case 'ok':
      return $gettext('Passed')
    case 'warn':
      return $gettext('Warning')
    case 'fail':
      return $gettext('Failed')
    default:
      return $gettext('Skipped')
  }
}

const handleInit = () => {
  initLoading.value = true
  useRequest(stalwart.init(initModel.value.hostname, initModel.value.domain))
    .onSuccess(() => {
      window.$message.success($gettext('Initialized successfully'))
    })
    .onComplete(() => {
      initLoading.value = false
      // 初始化要一两分钟，前置反代超时断开时服务端仍可能已完成，统一刷新状态
      emit('refresh')
    })
}

const handleCheck = () => {
  checkLoading.value = true
  useRequest(stalwart.check())
    .onSuccess(({ data }) => {
      checkItems.value = data
    })
    .onComplete(() => {
      checkLoading.value = false
    })
}

// 反代站点：未创建时可选证书一键创建，已创建后新增域名要同步子域
const proxyLoading = ref(false)
const proxyCertID = ref(0)
const certs = ref<any[]>([])
const certOptions = computed(() => [
  { label: $gettext('Do not bind a certificate for now'), value: 0 },
  ...certs.value
    .filter((item: any) => item.cert && item.key)
    .map((item: any) => ({
      label: `${item.domains.join(', ')} (${formatDateTime(item.not_after)})`,
      value: item.id,
    })),
])
const proxyMissing = computed(() => props.status.proxy_missing_domains ?? [])
const proxyDomains = computed(() => props.status.proxy_domains ?? [])

const loadCerts = () => {
  useRequest(cert.certs(1, 10000)).onSuccess(({ data }) => {
    certs.value = data.items
  })
}

const handleProxy = () => {
  proxyLoading.value = true
  useRequest(stalwart.proxy(proxyCertID.value))
    .onSuccess(({ data }) => {
      if (data.warnings?.length) {
        data.warnings.forEach((item: string) => window.$message.warning(item))
      }
      window.$message.success($gettext('Reverse proxy website is ready'))
      emit('refresh')
    })
    .onComplete(() => {
      proxyLoading.value = false
    })
}

watch(
  () => props.status.initialized,
  (initialized) => {
    if (initialized) loadCerts()
  },
  { immediate: true },
)
</script>

<template>
  <n-flex vertical>
    <n-card v-if="!status.initialized" :title="$gettext('Initialize')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'The mail server is installed but not initialized. Enter the public hostname of this server and the first email domain to serve. Initialization takes about a minute and restarts the service several times.',
            )
          }}
        </n-alert>
        <n-form :model="initModel" v-bind="formLayout">
          <n-form-item path="hostname" :label="$gettext('Hostname')">
            <n-input
              v-model:value="initModel.hostname"
              placeholder="mail.example.com"
              @keydown.enter.prevent
            />
          </n-form-item>
          <n-form-item path="domain" :label="$gettext('Email Domain')">
            <n-input
              v-model:value="initModel.domain"
              placeholder="example.com"
              @keydown.enter.prevent
            />
          </n-form-item>
        </n-form>
        <n-button
          type="primary"
          :loading="initLoading"
          :disabled="initLoading || !initModel.hostname || !initModel.domain"
          @click="handleInit"
        >
          {{ $gettext('Initialize') }}
        </n-button>
      </n-flex>
    </n-card>
    <template v-else>
      <n-card :title="$gettext('Overview')">
        <n-descriptions :column="3" label-placement="left" bordered>
          <n-descriptions-item :label="$gettext('Hostname')">
            {{ status.hostname || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Domains')">
            {{ status.domain_count }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('HTTP Port')">
            127.0.0.1:{{ status.http_port }}
          </n-descriptions-item>
        </n-descriptions>
      </n-card>
      <n-card :title="$gettext('Self Check')">
        <n-flex vertical>
          <n-text depth="3">
            {{
              $gettext(
                'Checks outbound port 25, hostname resolution, reverse DNS, IP blacklists, listening ports and conflicting services. Fix failed items before sending mail.',
              )
            }}
          </n-text>
          <n-flex>
            <n-button
              type="primary"
              :loading="checkLoading"
              :disabled="checkLoading"
              @click="handleCheck"
            >
              {{ $gettext('Run Check') }}
            </n-button>
          </n-flex>
          <n-list v-if="checkItems.length" bordered>
            <n-list-item v-for="item in checkItems" :key="item.key">
              <n-flex vertical size="small">
                <n-flex align="center">
                  <n-tag :type="tagType(item.status)" size="small" :bordered="false">
                    {{ tagText(item.status) }}
                  </n-tag>
                  <n-text strong>{{ item.title }}</n-text>
                </n-flex>
                <n-text depth="3">{{ item.detail }}</n-text>
              </n-flex>
            </n-list-item>
          </n-list>
        </n-flex>
      </n-card>
      <n-card :title="$gettext('Reverse Proxy')">
        <n-flex vertical>
          <n-text>
            {{
              $gettext(
                'The Stalwart admin UI, client autoconfig and MTA-STS policy are served over plain HTTP on 127.0.0.1:%{ port } and are not exposed directly. The panel can create one reverse proxy website covering all hostnames below and point it to http://127.0.0.1:%{ port }. Mail protocols (25, 465, 587, 993) are served by Stalwart directly and do not go through it.',
                { port: String(status.http_port) },
              )
            }}
          </n-text>
          <n-flex size="small">
            <n-tag v-for="domain in proxyDomains" :key="domain" size="small" :bordered="false">
              {{ domain }}
            </n-tag>
          </n-flex>
          <template v-if="status.proxy_website_id">
            <n-alert v-if="proxyMissing.length" type="warning">
              {{
                $gettext(
                  'Website %{ name } exists but is missing hostnames for newly added domains: %{ domains }',
                  { name: status.proxy_website_name, domains: proxyMissing.join(', ') },
                )
              }}
            </n-alert>
            <n-alert v-else type="success">
              {{
                $gettext('Website %{ name } covers all hostnames above', {
                  name: status.proxy_website_name,
                })
              }}
            </n-alert>
            <n-flex>
              <n-button
                v-if="proxyMissing.length"
                type="primary"
                :loading="proxyLoading"
                :disabled="proxyLoading"
                @click="handleProxy"
              >
                {{ $gettext('Sync Hostnames') }}
              </n-button>
              <router-link :to="{ name: 'website-index' }">
                <n-button>{{ $gettext('Open Website Management') }}</n-button>
              </router-link>
            </n-flex>
          </template>
          <template v-else>
            <n-form v-bind="formLayout">
              <n-form-item :label="$gettext('Certificate')">
                <n-select
                  v-model:value="proxyCertID"
                  :options="certOptions"
                  :placeholder="$gettext('Select a certificate covering the hostnames above')"
                />
              </n-form-item>
            </n-form>
            <n-text depth="3">
              {{
                $gettext(
                  'The certificate is deployed to the website with HTTPS enabled and, if the mail server has no certificate yet, also bound to it for SMTP and IMAP.',
                )
              }}
            </n-text>
            <n-flex>
              <n-button
                type="primary"
                :loading="proxyLoading"
                :disabled="proxyLoading"
                @click="handleProxy"
              >
                {{ $gettext('Create Reverse Proxy Website') }}
              </n-button>
            </n-flex>
          </template>
        </n-flex>
      </n-card>
    </template>
  </n-flex>
</template>

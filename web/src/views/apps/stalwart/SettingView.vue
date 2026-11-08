<script setup lang="ts">
defineOptions({
  name: 'stalwart-settings',
})

import copy2clipboard from '@vavt/copy2clipboard'
import { useGettext } from 'vue3-gettext'

import stalwart from '@/api/apps/stalwart'
import cert from '@/api/panel/cert'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { formatDateTime } from '@/utils'

const emit = defineEmits<{ refresh: [] }>()

const { $gettext } = useGettext()
const formLayout = useFormLayout(140)

const hostname = ref('')
const hostnameLoading = ref(false)

const certID = ref(0)
const certs = ref<any[]>([])
const certLoading = ref(false)

const listeners = ref({ submission: true, imap: false, pop3: false })
const listenersLoading = ref(false)

const relay = ref({
  enabled: false,
  host: '',
  port: 587,
  username: '',
  password: '',
  implicit_tls: false,
})
const relayLoading = ref(false)

const admin = ref<{ username: string; password: string } | null>(null)
const adminLoading = ref(false)

const certOptions = computed(() => [
  { label: $gettext('Not bound (self-signed certificate)'), value: 0 },
  ...certs.value
    .filter((item: any) => item.cert && item.key)
    .map((item: any) => ({
      label: `${item.domains.join(', ')} (${$gettext('expires %{ date }', { date: formatDateTime(item.not_after, 'yyyy-MM-dd') })})`,
      value: item.id,
    })),
])

const loadSettings = async () => {
  const data = await stalwart.settings()
  hostname.value = data.hostname
  certID.value = data.cert_id
  listeners.value = data.listeners
  relay.value = { ...data.relay, password: '' }
}

const handleSaveHostname = () => {
  hostnameLoading.value = true
  useRequest(stalwart.saveHostname(hostname.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
      emit('refresh')
    })
    .onComplete(() => {
      hostnameLoading.value = false
    })
}

const handleSaveCert = () => {
  certLoading.value = true
  useRequest(stalwart.saveCert(certID.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      certLoading.value = false
    })
}

const handleSaveListeners = () => {
  listenersLoading.value = true
  useRequest(stalwart.saveListeners(listeners.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
      emit('refresh')
    })
    .onComplete(() => {
      listenersLoading.value = false
    })
}

const handleSaveRelay = () => {
  relayLoading.value = true
  useRequest(stalwart.saveRelay(relay.value))
    .onSuccess(() => {
      relay.value.password = ''
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      relayLoading.value = false
    })
}

const handleViewAdmin = () => {
  adminLoading.value = true
  useRequest(stalwart.admin())
    .onSuccess(({ data }) => {
      admin.value = data
    })
    .onComplete(() => {
      adminLoading.value = false
    })
}

const copy = (text: string) => {
  copy2clipboard(text).then(() => {
    window.$message.success($gettext('Copied successfully'))
  })
}

onMounted(() => {
  loadSettings()
  useRequest(cert.certs(1, 10000)).onSuccess(({ data }) => {
    certs.value = data.items
  })
})
</script>

<template>
  <n-flex vertical>
    <n-card :title="$gettext('Hostname')">
      <n-flex vertical>
        <n-text depth="3">
          {{
            $gettext(
              'Public hostname of this mail server, used in the SMTP greeting, MX records and client autoconfig. It must resolve to this server and its reverse DNS must point back to it.',
            )
          }}
        </n-text>
        <n-flex>
          <n-input v-model:value="hostname" class="w-80" placeholder="mail.example.com" />
          <n-button
            type="primary"
            :loading="hostnameLoading"
            :disabled="hostnameLoading || !hostname"
            @click="handleSaveHostname"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>
    <n-card :title="$gettext('TLS Certificate')">
      <n-flex vertical>
        <n-text depth="3">
          {{
            $gettext(
              'Choose a certificate from Certificate management that covers the hostname. It is used for SMTP, IMAP and POP3 connections, and the panel pushes renewed certificates automatically.',
            )
          }}
        </n-text>
        <n-flex>
          <n-select v-model:value="certID" class="w-120" :options="certOptions" />
          <n-button
            type="primary"
            :loading="certLoading"
            :disabled="certLoading"
            @click="handleSaveCert"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>
    <n-card :title="$gettext('Listening Ports')">
      <n-flex vertical>
        <n-text depth="3">
          {{
            $gettext(
              'Ports 25 (SMTP), 465 (SMTPS), 993 (IMAPS), 995 (POP3S) and 4190 (ManageSieve) are always on. The STARTTLS ports below are optional and saving restarts the service.',
            )
          }}
        </n-text>
        <n-form v-bind="formLayout">
          <n-form-item :label="$gettext('587 Submission')">
            <n-flex align="center">
              <n-switch v-model:value="listeners.submission" />
              <n-text depth="3">{{
                $gettext('STARTTLS submission, used by most mail clients')
              }}</n-text>
            </n-flex>
          </n-form-item>
          <n-form-item :label="$gettext('143 IMAP')">
            <n-flex align="center">
              <n-switch v-model:value="listeners.imap" />
              <n-text depth="3">{{ $gettext('STARTTLS IMAP, 993 is preferred') }}</n-text>
            </n-flex>
          </n-form-item>
          <n-form-item :label="$gettext('110 POP3')">
            <n-flex align="center">
              <n-switch v-model:value="listeners.pop3" />
              <n-text depth="3">{{ $gettext('STARTTLS POP3, 995 is preferred') }}</n-text>
            </n-flex>
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="listenersLoading"
            :disabled="listenersLoading"
            @click="handleSaveListeners"
          >
            {{ $gettext('Save and Restart') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>
    <n-card :title="$gettext('Outbound Relay')">
      <n-flex vertical>
        <n-text depth="3">
          {{
            $gettext(
              'Most cloud providers block outbound port 25. When enabled, all outgoing mail to other domains is handed to the relay (smarthost) over port 587 (STARTTLS) or 465 (implicit TLS) instead of being delivered directly.',
            )
          }}
        </n-text>
        <n-form :model="relay" v-bind="formLayout">
          <n-form-item :label="$gettext('Enable')">
            <n-switch v-model:value="relay.enabled" />
          </n-form-item>
          <template v-if="relay.enabled">
            <n-form-item path="host" :label="$gettext('Relay Host')">
              <n-input
                v-model:value="relay.host"
                placeholder="smtp.example.com"
                @keydown.enter.prevent
              />
            </n-form-item>
            <n-form-item path="port" :label="$gettext('Port')">
              <n-input-number
                v-model:value="relay.port"
                :min="1"
                :max="65535"
                @keydown.enter.prevent
              />
            </n-form-item>
            <n-form-item path="username" :label="$gettext('Username')">
              <n-input v-model:value="relay.username" @keydown.enter.prevent />
            </n-form-item>
            <n-form-item path="password" :label="$gettext('Password')">
              <n-input
                v-model:value="relay.password"
                type="password"
                show-password-on="click"
                :placeholder="$gettext('Leave empty to keep the current password')"
                @keydown.enter.prevent
              />
            </n-form-item>
            <n-form-item path="implicit_tls" :label="$gettext('Implicit TLS')">
              <n-flex align="center">
                <n-switch v-model:value="relay.implicit_tls" />
                <n-text depth="3">{{
                  $gettext('Turn on for port 465, off for 587 STARTTLS')
                }}</n-text>
              </n-flex>
            </n-form-item>
          </template>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="relayLoading"
            :disabled="relayLoading"
            @click="handleSaveRelay"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>
    <n-card :title="$gettext('Administrator Credentials')">
      <n-flex vertical>
        <n-text depth="3">
          {{
            $gettext(
              'The panel manages the server with this account. You can also use it to sign in to the built-in Stalwart WebUI at /admin once a reverse proxy website for the hostname exists.',
            )
          }}
        </n-text>
        <n-flex v-if="admin" vertical size="small">
          <n-flex align="center">
            <n-text>{{ $gettext('Username') }}:</n-text>
            <n-text code class="cursor-pointer" @click="copy(admin.username)">{{
              admin.username
            }}</n-text>
          </n-flex>
          <n-flex align="center">
            <n-text>{{ $gettext('Password') }}:</n-text>
            <n-text code class="cursor-pointer" @click="copy(admin.password)">{{
              admin.password
            }}</n-text>
          </n-flex>
          <n-text depth="3">{{ $gettext('Click a value to copy it') }}</n-text>
        </n-flex>
        <n-flex v-else>
          <n-button :loading="adminLoading" :disabled="adminLoading" @click="handleViewAdmin">
            {{ $gettext('View') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>
  </n-flex>
</template>

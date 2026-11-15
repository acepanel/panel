<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  dnsProviders: any[]
  editItem?: any
}>()

const isEdit = computed(() => !!props.editItem)
const loading = ref(false)

interface CredentialField {
  key: 'ak' | 'sk'
  label: string
  placeholder: string
}

// 各服务商的凭据字段，顺序即表单中的展示顺序
const credentialFields: Record<string, CredentialField[]> = {
  aliyun: [
    { key: 'ak', label: 'AccessKey ID', placeholder: $gettext('Enter Aliyun AccessKey ID') },
    {
      key: 'sk',
      label: 'AccessKey Secret',
      placeholder: $gettext('Enter Aliyun AccessKey Secret'),
    },
  ],
  tencent: [
    { key: 'ak', label: 'SecretId', placeholder: $gettext('Enter Tencent Cloud SecretId') },
    { key: 'sk', label: 'SecretKey', placeholder: $gettext('Enter Tencent Cloud SecretKey') },
  ],
  huawei: [
    { key: 'ak', label: 'AccessKeyId', placeholder: $gettext('Enter Huawei Cloud AccessKeyId') },
    {
      key: 'sk',
      label: 'SecretAccessKey',
      placeholder: $gettext('Enter Huawei Cloud SecretAccessKey'),
    },
  ],
  westcn: [
    { key: 'sk', label: 'Username', placeholder: $gettext('Enter West.cn Username') },
    { key: 'ak', label: 'API Password', placeholder: $gettext('Enter West.cn API Password') },
  ],
  cloudflare: [{ key: 'ak', label: 'API Key', placeholder: $gettext('Enter Cloudflare API Key') }],
  gcore: [{ key: 'ak', label: 'API Key', placeholder: $gettext('Enter G-Core API Key') }],
  porkbun: [
    { key: 'ak', label: 'API Key', placeholder: $gettext('Enter Porkbun API Key') },
    { key: 'sk', label: 'Secret Key', placeholder: $gettext('Enter Porkbun Secret Key') },
  ],
  namesilo: [{ key: 'ak', label: 'API Token', placeholder: $gettext('Enter NameSilo API Token') }],
  cloudns: [
    {
      key: 'ak',
      label: 'Auth ID',
      placeholder: $gettext('Enter ClouDNS Auth ID (use Sub Auth ID by adding sub-prefix)'),
    },
    { key: 'sk', label: 'Auth Password', placeholder: $gettext('Enter ClouDNS Auth Password') },
  ],
}

const defaultModel = () => ({
  data: {
    ak: '',
    sk: '',
    dns_server: '8.8.8.8',
    skip_verify: false,
  },
  type: 'aliyun',
  name: '',
})

const model = ref<any>(defaultModel())

watch(show, (val) => {
  if (!val) return
  const item = props.editItem
  model.value = item
    ? {
        data: {
          ak: item.dns_param.ak,
          sk: item.dns_param.sk,
          dns_server: item.dns_param.dns_server || '8.8.8.8',
          skip_verify: item.dns_param.skip_verify || false,
        },
        type: item.type,
        name: item.name,
      }
    : defaultModel()
})

const handleSubmit = () => {
  loading.value = true
  const req = isEdit.value
    ? cert.dnsUpdate(props.editItem.id, model.value)
    : cert.dnsCreate(model.value)
  useRequest(req)
    .onSuccess(() => {
      window.$bus.emit('cert:refresh-dns')
      window.$bus.emit('cert:refresh-async')
      show.value = false
      window.$message.success(
        isEdit.value ? $gettext('Update successful') : $gettext('Created successfully'),
      )
    })
    .onComplete(() => {
      loading.value = false
    })
}
</script>

<template>
  <app-modal v-model:show="show" :title="isEdit ? $gettext('Modify DNS') : $gettext('Create DNS')">
    <n-space vertical>
      <n-form :model="model">
        <n-form-item path="name" :label="$gettext('Note Name')">
          <n-input
            v-model:value="model.name"
            type="text"
            :placeholder="$gettext('Enter note name')"
          />
        </n-form-item>
        <n-form-item path="type" :label="$gettext('DNS')">
          <n-select
            v-model:value="model.type"
            :placeholder="$gettext('Select DNS')"
            clearable
            :options="dnsProviders"
          />
        </n-form-item>
        <n-form-item
          v-for="field in credentialFields[model.type] ?? []"
          :key="field.key"
          :path="field.key"
          :label="field.label"
        >
          <n-input
            v-model:value="model.data[field.key]"
            type="text"
            :placeholder="field.placeholder"
          />
        </n-form-item>
        <n-form-item path="dns_server" :label="$gettext('DNS Server')">
          <n-input
            v-model:value="model.data.dns_server"
            type="text"
            :placeholder="$gettext('DNS server for propagation check, e.g. 8.8.8.8')"
          />
        </n-form-item>
        <n-form-item path="skip_verify" :label="$gettext('Skip DNS Verification')">
          <n-switch v-model:value="model.data.skip_verify" />
          <n-text depth="3" class="ml-2">
            {{ $gettext('For intranet environments, will wait 60s instead of polling DNS') }}
          </n-text>
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
        {{ $gettext('Submit') }}
      </n-button>
    </n-space>
  </app-modal>
</template>

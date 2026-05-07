<script setup lang="ts">
import { NButton, NInput, NSpace, NSwitch, NText } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const props = defineProps({
  dnsProviders: {
    type: Array<any>,
    required: true,
  },
})

const { dnsProviders } = toRefs(props)

const model = ref<any>({
  data: {
    ak: '',
    sk: '',
    dns_server: '8.8.8.8',
    skip_verify: false,
  },
  type: 'aliyun',
  name: '',
})

const loading = ref(false)

const handleCreateDNS = async () => {
  loading.value = true
  useRequest(cert.dnsCreate(model.value))
    .onSuccess(() => {
      window.$bus.emit('cert:refresh-dns')
      window.$bus.emit('cert:refresh-async')
      show.value = false
      model.value.data.ak = ''
      model.value.data.sk = ''
      model.value.data.dns_server = '8.8.8.8'
      model.value.data.skip_verify = false
      model.value.name = ''
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      loading.value = false
    })
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Create DNS')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-form :model="model">
        <n-form-item path="name" :label="$gettext('Comment Name')">
          <n-input
            v-model:value="model.name"
            type="text"
            :placeholder="$gettext('Enter comment name')"
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
        <n-form-item v-if="model.type == 'aliyun'" path="ak" label="Access Key">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter Aliyun Access Key')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'aliyun'" path="sk" label="Secret Key">
          <n-input
            v-model:value="model.data.sk"
            type="text"
            :placeholder="$gettext('Enter Aliyun Secret Key')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'tencent'" path="ak" label="SecretId">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter Tencent Cloud SecretId')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'tencent'" path="sk" label="SecretKey">
          <n-input
            v-model:value="model.data.sk"
            type="text"
            :placeholder="$gettext('Enter Tencent Cloud SecretKey')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'huawei'" path="ak" label="AccessKeyId">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter Huawei Cloud AccessKeyId')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'huawei'" path="sk" label="SecretAccessKey">
          <n-input
            v-model:value="model.data.sk"
            type="text"
            :placeholder="$gettext('Enter Huawei Cloud SecretAccessKey')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'westcn'" path="sk" label="Username">
          <n-input
            v-model:value="model.data.sk"
            type="text"
            :placeholder="$gettext('Enter West.cn Username')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'westcn'" path="ak" label="API Password">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter West.cn API Password')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'cloudflare'" path="ak" label="API Key">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter Cloudflare API Key')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'gcore'" path="ak" label="API Key">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter G-Core API Key')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'porkbun'" path="ak" label="API Key">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter Porkbun API Key')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'porkbun'" path="sk" label="Secret Key">
          <n-input
            v-model:value="model.data.sk"
            type="text"
            :placeholder="$gettext('Enter Porkbun Secret Key')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'namesilo'" path="ak" label="API Token">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter NameSilo API Token')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'cloudns'" path="ak" label="Auth ID">
          <n-input
            v-model:value="model.data.ak"
            type="text"
            :placeholder="$gettext('Enter ClouDNS Auth ID (use Sub Auth ID by adding sub-prefix)')"
          />
        </n-form-item>
        <n-form-item v-if="model.type == 'cloudns'" path="sk" label="Auth Password">
          <n-input
            v-model:value="model.data.sk"
            type="text"
            :placeholder="$gettext('Enter ClouDNS Auth Password')"
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
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreateDNS">{{
        $gettext('Submit')
      }}</n-button>
    </n-space>
  </n-modal>
</template>

<style scoped lang="scss"></style>

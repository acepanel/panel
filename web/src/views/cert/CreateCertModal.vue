<script setup lang="ts">
import cert from '@/api/panel/cert'
import { NButton, NSpace } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const props = defineProps({
  algorithms: {
    type: Array<any>,
    required: true
  },
  websites: {
    type: Array<any>,
    required: true
  },
  accounts: {
    type: Array<any>,
    required: true
  },
  dns: {
    type: Array<any>,
    required: true
  }
})

const { algorithms, websites, accounts, dns } = toRefs(props)

const model = ref<any>({
  domains: [],
  dns_id: null,
  type: 'P256',
  account_id: null,
  website_id: null,
  auto_renewal: true
})

const loading = ref(false)

const handleCreateCert = () => {
  loading.value = true
  useRequest(cert.certCreate(model.value))
    .onSuccess(() => {
      window.$bus.emit('cert:refresh-cert')
      window.$bus.emit('cert:refresh-async')
      show.value = false
      model.value.domains = []
      model.value.dns_id = null
      model.value.type = 'P256'
      model.value.account_id = null
      model.value.website_id = null
      model.value.auto_renewal = true
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
    :title="$gettext('Create Certificate')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-alert type="info">
        {{
          $gettext(
            'You can automatically issue and deploy certificates by selecting either Website or DNS, or you can manually enter domain names and set up DNS resolution to issue certificates'
          )
        }}
      </n-alert>
      <n-form :model="model">
        <n-form-item :label="$gettext('Domain')">
          <n-dynamic-input
            v-model:value="model.domains"
            placeholder="example.com"
            :min="1"
            show-sort-button
          />
        </n-form-item>
        <n-form-item path="type" :label="$gettext('Key Type')">
          <n-select
            v-model:value="model.type"
            :placeholder="$gettext('Select key type')"
            clearable
            :options="algorithms"
          />
        </n-form-item>
        <n-form-item path="website_id" :label="$gettext('Website')">
          <n-select
            v-model:value="model.website_id"
            :placeholder="$gettext('Select website for certificate deployment')"
            clearable
            :options="websites"
          />
        </n-form-item>
        <n-form-item path="account_id" :label="$gettext('Account')">
          <n-select
            v-model:value="model.account_id"
            :placeholder="$gettext('Select account for certificate issuance')"
            clearable
            :options="accounts"
          />
        </n-form-item>
        <n-form-item path="account_id" :label="$gettext('DNS')">
          <n-select
            v-model:value="model.dns_id"
            :placeholder="$gettext('Select DNS for certificate issuance')"
            clearable
            :options="dns"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreateCert">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
</template>

<style scoped lang="scss"></style>

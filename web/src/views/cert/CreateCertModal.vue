<script setup lang="ts">
import { NButton, NInput, NSpace } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const props = defineProps({
  algorithms: {
    type: Array<any>,
    required: true,
  },
  websites: {
    type: Array<any>,
    required: true,
  },
  accounts: {
    type: Array<any>,
    required: true,
  },
  dns: {
    type: Array<any>,
    required: true,
  },
})

const { algorithms, websites, accounts, dns } = toRefs(props)

const model = ref<any>({
  domains: [],
  alias: {},
  dns_id: null,
  type: 'P256',
  account_id: null,
  website_id: null,
  auto_renewal: true,
})

const aliasList = ref<{ key: string; value: string }[]>([])

// alias list ↔ model.alias 同步
watch(
  aliasList,
  (list) => {
    const map: Record<string, string> = {}
    for (const item of list) {
      if (item.key && item.value) {
        map[item.key] = item.value
      }
    }
    model.value.alias = map
  },
  { deep: true },
)

const loading = ref(false)

const handleCreateCert = () => {
  loading.value = true
  useRequest(cert.certCreate(model.value))
    .onSuccess(() => {
      window.$bus.emit('cert:refresh-cert')
      window.$bus.emit('cert:refresh-async')
      show.value = false
      model.value.domains = []
      model.value.alias = {}
      model.value.dns_id = null
      model.value.type = 'P256'
      model.value.account_id = null
      model.value.website_id = null
      model.value.auto_renewal = true
      aliasList.value = []
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
            'You can automatically issue and deploy certificates by selecting either Website or DNS, or you can manually enter domain names and set up DNS resolution to issue certificates',
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
        <n-form-item v-if="model.dns_id" :label="$gettext('DNS Alias')">
          <n-dynamic-input v-model:value="aliasList" :on-create="() => ({ key: '', value: '' })">
            <template #default="{ value }">
              <div style="display: flex; align-items: center; gap: 8px; width: 100%">
                <n-input
                  v-model:value="value.key"
                  :placeholder="$gettext('Original domain, e.g. example.com')"
                  style="flex: 1"
                />
                <span>→</span>
                <n-input
                  v-model:value="value.value"
                  :placeholder="$gettext('Alias record, e.g. _acme-challenge.delegated.com')"
                  style="flex: 1"
                />
              </div>
            </template>
          </n-dynamic-input>
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreateCert">
        {{ $gettext('Submit') }}
      </n-button>
    </n-space>
  </n-modal>
</template>

<style scoped lang="scss"></style>

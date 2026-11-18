<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'
import UploadFields from '@/views/cert/UploadFields.vue'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  algorithms: any[]
  websites: any[]
  accounts: any[]
  dns: any[]
  editItem?: any
}>()

const isEdit = computed(() => !!props.editItem)

const defaultModel = () => ({
  domains: [],
  alias: {},
  type: 'P256',
  dns_id: null,
  account_id: null,
  website_ids: [],
  auto_renewal: true,
})

const model = ref<any>(defaultModel())
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

watch(show, (val) => {
  if (!val) return
  const item = props.editItem
  if (!item) {
    model.value = defaultModel()
    aliasList.value = []
    return
  }
  // 证书、私钥、部署脚本只有更新接口接收
  model.value = {
    domains: item.domains,
    alias: item.alias || {},
    type: item.type,
    dns_id: item.dns_id == 0 ? null : item.dns_id,
    account_id: item.account_id == 0 ? null : item.account_id,
    website_ids: item.website_ids || [],
    auto_renewal: item.auto_renewal,
    cert: item.cert,
    key: item.key,
    cert_path: item.cert_path,
    key_path: item.key_path,
    script: item.script,
  }
  aliasList.value = Object.entries(item.alias || {}).map(([key, value]) => ({
    key,
    value: value as string,
  }))
})

const handleSubmit = async () => {
  const req = isEdit.value
    ? cert.certUpdate(props.editItem.id, model.value)
    : cert.certCreate(model.value)
  await req
  window.$bus.emit('cert:refresh-cert')
  window.$bus.emit('cert:refresh-async')
  show.value = false
  window.$message.success(
    isEdit.value ? $gettext('Update successful') : $gettext('Created successfully'),
  )
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="isEdit ? $gettext('Modify Certificate') : $gettext('Create Certificate')"
  >
    <n-space vertical>
      <n-alert v-if="model.type != 'upload'" type="info">
        {{
          isEdit
            ? $gettext(
                'You can automatically issue and deploy certificates by selecting any website/DNS, or manually enter domain names and set DNS resolution to issue certificates, or fill in deployment scripts to automatically deploy certificates.',
              )
            : $gettext(
                'You can automatically issue and deploy certificates by selecting either Website or DNS, or you can manually enter domain names and set up DNS resolution to issue certificates',
              )
        }}
      </n-alert>
      <n-form :model="model">
        <n-form-item v-if="model.type != 'upload'" path="domains" :label="$gettext('Domain')">
          <ListInput
            v-model:value="model.domains"
            placeholder="example.com"
            :min="1"
            show-sort-button
          />
        </n-form-item>
        <n-form-item v-if="model.type != 'upload'" path="type" :label="$gettext('Key Type')">
          <n-select
            v-model:value="model.type"
            :placeholder="$gettext('Select key type')"
            clearable
            :options="algorithms"
          />
        </n-form-item>
        <n-form-item path="website_ids" :label="$gettext('Website')">
          <n-select
            v-model:value="model.website_ids"
            :placeholder="$gettext('Select websites for certificate deployment')"
            multiple
            clearable
            :options="websites"
          />
        </n-form-item>
        <n-form-item v-if="model.type != 'upload'" path="account_id" :label="$gettext('Account')">
          <n-select
            v-model:value="model.account_id"
            :placeholder="$gettext('Select account for certificate issuance')"
            clearable
            :options="accounts"
          />
        </n-form-item>
        <n-form-item v-if="model.type != 'upload'" path="account_id" :label="$gettext('DNS')">
          <n-select
            v-model:value="model.dns_id"
            :placeholder="$gettext('Select DNS for certificate issuance')"
            clearable
            :options="dns"
          />
        </n-form-item>
        <n-form-item v-if="model.type != 'upload' && model.dns_id" :label="$gettext('DNS Alias')">
          <n-dynamic-input v-model:value="aliasList" :on-create="() => ({ key: '', value: '' })">
            <template #default="{ value }">
              <div class="flex gap-2 w-full items-center">
                <n-input
                  v-model:value="value.key"
                  :placeholder="$gettext('Original domain, e.g. example.com')"
                  class="flex-1"
                />
                <span>→</span>
                <n-input
                  v-model:value="value.value"
                  :placeholder="$gettext('Alias record, e.g. _acme-challenge.delegated.com')"
                  class="flex-1"
                />
              </div>
            </template>
          </n-dynamic-input>
        </n-form-item>
        <upload-fields v-if="model.type == 'upload'" v-model:value="model" />
        <n-form-item
          v-if="isEdit && model.type != 'upload'"
          path="key"
          :label="$gettext('Deployment Script')"
        >
          <n-input
            v-model:value="model.script"
            type="textarea"
            :placeholder="
              $gettext(
                'The {cert} and {key} in the script will be replaced with the certificate and private key content',
              )
            "
            :autosize="{ minRows: 5, maxRows: 10 }"
          />
        </n-form-item>
      </n-form>
      <app-button type="info" block @click="handleSubmit">
        {{ $gettext('Submit') }}
      </app-button>
    </n-space>
  </app-modal>
</template>

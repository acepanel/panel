<script setup lang="ts">
import type { MessageReactive } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  caProviders: any[]
  algorithms: any[]
  editItem?: any
}>()

const isEdit = computed(() => !!props.editItem)
let messageReactive: MessageReactive | null = null
const loading = ref(false)

const defaultModel = () => ({
  hmac_encoded: '',
  email: '',
  kid: '',
  key_type: 'P256',
  ca: 'letsencrypt',
})

const model = ref<any>(defaultModel())

const showEAB = computed(() => {
  return model.value.ca === 'google' || model.value.ca === 'litessl' || model.value.ca === 'sslcom'
})

watch(show, (val) => {
  if (!val) return
  const item = props.editItem
  model.value = item
    ? {
        hmac_encoded: item.hmac_encoded,
        email: item.email,
        kid: item.kid,
        key_type: item.key_type,
        ca: item.ca,
      }
    : defaultModel()
})

const handleSubmit = () => {
  loading.value = true
  messageReactive = window.$message.loading(
    $gettext('Registering account with CA, please wait patiently'),
    {
      duration: 0,
    },
  )
  const req = isEdit.value
    ? cert.accountUpdate(props.editItem.id, model.value)
    : cert.accountCreate(model.value)
  useRequest(req)
    .onSuccess(() => {
      window.$bus.emit('cert:refresh-account')
      window.$bus.emit('cert:refresh-async')
      show.value = false
      window.$message.success(
        isEdit.value ? $gettext('Update successful') : $gettext('Created successfully'),
      )
    })
    .onComplete(() => {
      loading.value = false
      messageReactive?.destroy()
    })
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="isEdit ? $gettext('Modify Account') : $gettext('Create Account')"
  >
    <n-space vertical>
      <n-alert type="info">{{
        $gettext(
          'LiteSSL, Google and SSL.com require obtaining EAB (KID and HMAC) from their official websites first',
        )
      }}</n-alert>
      <n-alert type="warning">
        {{
          $gettext(
            "Google is not accessible in mainland China, other CAs depend on network conditions, recommend using Let's Encrypt or LiteSSL",
          )
        }}
      </n-alert>
      <n-form :model="model">
        <n-form-item path="ca" :label="$gettext('CA')">
          <n-select
            v-model:value="model.ca"
            :placeholder="$gettext('Select CA')"
            clearable
            :options="caProviders"
          />
        </n-form-item>
        <n-form-item path="key_type" :label="$gettext('Key Type')">
          <n-select
            v-model:value="model.key_type"
            :placeholder="$gettext('Select key type')"
            clearable
            :options="algorithms"
          />
        </n-form-item>
        <n-form-item path="email" :label="$gettext('Email')">
          <n-input
            v-model:value="model.email"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter email address')"
          />
        </n-form-item>
        <n-form-item v-if="showEAB" path="kid" label="KID">
          <n-input
            v-model:value="model.kid"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter KID')"
          />
        </n-form-item>
        <n-form-item v-if="showEAB" path="hmac_encoded" label="HMAC">
          <n-input
            v-model:value="model.hmac_encoded"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter HMAC')"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
        {{ $gettext('Submit') }}
      </n-button>
    </n-space>
  </app-modal>
</template>

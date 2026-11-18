<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import notify from '@/api/panel/notify'
import { useFormLayout } from '@/components/system/composables/useFormLayout'

const { $gettext } = useGettext()
const formLayout = useFormLayout(120)

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  channel?: any
}>()
const emit = defineEmits(['saved'])

const isEdit = computed(() => !!props.channel)

const types = computed(() => [{ label: $gettext('SMTP Email'), value: 'smtp' }])

const encryptions = computed(() => [
  { label: $gettext('SSL/TLS (465)'), value: 'ssl' },
  { label: $gettext('STARTTLS (587)'), value: 'starttls' },
  { label: $gettext('None (25)'), value: 'none' },
])

const defaultConfig = () => ({
  host: '',
  port: 465,
  encryption: 'ssl',
  username: '',
  password: '',
  from: '',
  from_name: 'AcePanel',
  to: [] as string[],
  skip_verify: false,
})

const model = ref({
  name: '',
  type: 'smtp',
  config: defaultConfig(),
  enabled: true,
})

// 切换加密方式时同步常用端口
const handleEncryptionChange = (value: string) => {
  model.value.config.port = value === 'ssl' ? 465 : value === 'starttls' ? 587 : 25
}

watch(show, (val) => {
  if (!val) return
  if (props.channel) {
    model.value = {
      name: props.channel.name,
      type: props.channel.type,
      config: { ...defaultConfig(), ...props.channel.config },
      enabled: props.channel.enabled,
    }
  } else {
    model.value = { name: '', type: 'smtp', config: defaultConfig(), enabled: true }
  }
})

const handleSubmit = async () => {
  const req = isEdit.value
    ? notify.updateChannel(props.channel.id, model.value)
    : notify.createChannel(model.value)
  await req
  window.$message.success($gettext('Saved successfully'))
  show.value = false
  emit('saved')
}
</script>

<template>
  <app-modal
    size="sm"
    v-model:show="show"
    :title="isEdit ? $gettext('Edit Channel') : $gettext('Add Channel')"
  >
    <n-form v-bind="formLayout">
      <n-form-item :label="$gettext('Name')" required>
        <n-input v-model:value="model.name" :placeholder="$gettext('Channel name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Type')" required>
        <n-select v-model:value="model.type" :options="types" />
      </n-form-item>
      <n-form-item :label="$gettext('SMTP Server')" required>
        <n-input v-model:value="model.config.host" placeholder="smtp.example.com" />
      </n-form-item>
      <n-form-item :label="$gettext('Encryption')" required>
        <n-flex :size="8" class="w-full">
          <n-select
            v-model:value="model.config.encryption"
            :options="encryptions"
            class="flex-1"
            @update:value="handleEncryptionChange"
          />
          <n-input-number
            v-model:value="model.config.port"
            :min="1"
            :max="65535"
            class="md:!w-40"
            :show-button="false"
          />
        </n-flex>
      </n-form-item>
      <n-form-item :label="$gettext('Username')">
        <n-input v-model:value="model.config.username" :placeholder="$gettext('Login account')" />
      </n-form-item>
      <n-form-item :label="$gettext('Password')">
        <n-input
          v-model:value="model.config.password"
          type="password"
          show-password-on="click"
          :placeholder="$gettext('Login password or authorization code')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Sender')">
        <n-input
          v-model:value="model.config.from"
          :placeholder="$gettext('Defaults to the username')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Sender Name')">
        <n-input v-model:value="model.config.from_name" placeholder="AcePanel" />
      </n-form-item>
      <n-form-item :label="$gettext('Recipients')" required>
        <n-dynamic-tags v-model:value="model.config.to" />
      </n-form-item>
      <n-form-item :label="$gettext('Skip Cert Verify')">
        <n-switch v-model:value="model.config.skip_verify" />
      </n-form-item>
      <n-form-item :label="$gettext('Enabled')">
        <n-switch v-model:value="model.enabled" />
      </n-form-item>
    </n-form>
    <template #footer>
      <n-flex justify="end">
        <n-button @click="show = false">{{ $gettext('Cancel') }}</n-button>
        <app-button type="primary" @click="handleSubmit">
          {{ $gettext('Save') }}
        </app-button>
      </n-flex>
    </template>
  </app-modal>
</template>

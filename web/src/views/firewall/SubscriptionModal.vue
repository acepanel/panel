<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import type { CronType } from '@/components/common/cron'
import { useFormLayout } from '@/components/system/composables/useFormLayout'

const { $gettext } = useGettext()
const formLayout = useFormLayout(140)

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  subscription?: any
}>()
const emit = defineEmits(['saved'])

const isEdit = computed(() => !!props.subscription)

const types = [
  { label: $gettext('Blacklist: block the listed addresses'), value: 'block' },
  { label: $gettext('Whitelist: exempt the listed addresses from blacklists'), value: 'allow' },
]

// 更新周期按分钟检查，秒级和开机触发用不上，每分钟拉一次对列表源也不友好
const excludedSchedules: CronType[] = ['every-second', 'every-n-seconds', 'every-minute', 'reboot']

// 默认每天更新一次，时刻随机，避免所有面板挤在同一分钟请求列表源
const newModel = () => ({
  name: '',
  type: 'block',
  url: '',
  schedule: `${Math.floor(Math.random() * 60)} ${Math.floor(Math.random() * 24)} * * *`,
  status: true,
})

const model = ref(newModel())

watch(show, (value) => {
  if (!value) return
  const { name, type, url, schedule, status } = props.subscription ?? newModel()
  model.value = { name, type, url, schedule, status }
})

const handleSubmit = async () => {
  if (isEdit.value) {
    await firewall.updateSubscription(props.subscription.id, model.value)
  } else {
    await firewall.createSubscription(model.value)
  }
  window.$message.success($gettext('Saved successfully'))
  show.value = false
  emit('saved')
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="isEdit ? $gettext('Edit Subscription') : $gettext('Create Subscription')"
  >
    <n-form v-bind="formLayout">
      <n-form-item :label="$gettext('Name')" required>
        <n-input v-model:value="model.name" :placeholder="$gettext('Subscription name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Type')" required>
        <n-select v-model:value="model.type" :options="types" />
      </n-form-item>
      <n-form-item :label="$gettext('List URL')" required>
        <n-input
          v-model:value="model.url"
          :placeholder="$gettext('Plain text list with one IP, CIDR or IP range per line')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Update Schedule')" required>
        <cron-selector v-model:value="model.schedule" :exclude="excludedSchedules" />
      </n-form-item>
      <n-form-item :label="$gettext('Enabled')">
        <n-switch v-model:value="model.status" />
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

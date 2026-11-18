<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import log from '@/api/panel/log'

const props = defineProps<{
  type: 'app' | 'db' | 'http'
}>()

const emit = defineEmits<{
  (e: 'cleaned'): void
}>()

const { $gettext } = useGettext()

const show = ref(false)
const date = ref<string | null>(null)

const handleClean = async () => {
  await log.clean(props.type, date.value!)
  show.value = false
  date.value = null
  window.$message.success($gettext('Cleaned successfully'))
  emit('cleaned')
}
</script>

<template>
  <n-popover v-model:show="show" trigger="click" placement="bottom-start">
    <template #trigger>
      <n-button type="error" ghost>
        {{ $gettext('Clean') }}
      </n-button>
    </template>
    <n-flex vertical>
      <n-date-picker
        v-model:formatted-value="date"
        value-format="yyyy-MM-dd"
        type="date"
        panel
        :actions="null"
        :is-date-disabled="(ts: number) => ts > Date.now()"
      />
      <n-text depth="3">
        {{ $gettext('Delete logs on and before the selected date') }}
      </n-text>
      <app-button type="error" block :disabled="!date" @click="handleClean">
        {{ $gettext('Clean') }}
      </app-button>
    </n-flex>
  </n-popover>
</template>

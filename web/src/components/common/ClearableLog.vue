<script setup lang="ts">
import type { Method } from 'alova'
import { useGettext } from 'vue3-gettext'

import systemctl from '@/api/panel/systemctl'

// path 是文件日志；service 是 journald 里的运行日志，不传 clear 时走 systemctl 的清空接口
const props = defineProps<{
  path?: string
  service?: string
  clear?: () => Method
  label?: string
}>()

const { $gettext } = useGettext()
const logRef = ref<{ clear: () => void } | null>(null)

const { loading, send } = useRequest(
  () => (props.clear ? props.clear() : systemctl.clearLog(props.service ?? '')),
  { immediate: false },
).onSuccess(() => {
  logRef.value?.clear()
  window.$message.success($gettext('Cleared successfully'))
})
</script>

<template>
  <n-flex vertical>
    <n-flex>
      <slot />
      <n-button
        type="primary"
        :loading="loading"
        :disabled="loading || !(path || service)"
        @click="send()"
      >
        {{ label ?? $gettext('Clear Log') }}
      </n-button>
    </n-flex>
    <realtime-log ref="logRef" :path="path" :service="service" />
  </n-flex>
</template>

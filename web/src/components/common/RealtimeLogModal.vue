<script setup lang="ts">
import type { PropType } from 'vue'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps({
  path: {
    type: String,
    required: false,
  },
  service: {
    type: String,
    required: false,
  },
  container: {
    type: String,
    required: false,
  },
  clearable: {
    type: Boolean,
    default: false,
  },
  // 声明成属性而不是事件，才能把处理函数返回的 Promise 交给确认气泡
  onClear: {
    type: Function as PropType<() => unknown>,
    required: false,
  },
})

const logRef = ref<{ clear: () => void } | null>(null)

const clear = async () => {
  logRef.value?.clear()
}

const handleDownload = () => {
  if (!props.path) return
  window.open('/api/file/download?path=' + encodeURIComponent(props.path))
}

defineExpose({ clear })
</script>

<template>
  <app-modal v-model:show="show" size="lg" :title="$gettext('Logs')">
    <template v-if="clearable || props.path" #header-extra>
      <n-flex size="small" align="center">
        <n-button v-if="props.path" size="small" type="primary" @click="handleDownload">
          {{ $gettext('Download Log') }}
        </n-button>
        <ConfirmDialog
          v-if="clearable"
          type="danger"
          :content="$gettext('Are you sure you want to clear the log?')"
          @confirm="props.onClear?.()"
        >
          <template #trigger>
            <app-button size="small" type="warning">
              {{ $gettext('Clear Log') }}
            </app-button>
          </template>
        </ConfirmDialog>
      </n-flex>
    </template>
    <realtime-log
      v-if="show"
      ref="logRef"
      :path="props.path"
      :service="props.service"
      :container="props.container"
    />
  </app-modal>
</template>

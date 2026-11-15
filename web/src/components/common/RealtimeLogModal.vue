<script setup lang="ts">
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
})
const emit = defineEmits<{ clear: [] }>()

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
          @confirm="emit('clear')"
        >
          <template #trigger>
            <n-button size="small" type="warning">
              {{ $gettext('Clear Log') }}
            </n-button>
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

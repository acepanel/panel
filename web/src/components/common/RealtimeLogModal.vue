<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import RealtimeLog from './RealtimeLog.vue'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps({
  path: {
    type: String,
    required: true,
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

defineExpose({ clear })
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Logs')"
    style="width: 80vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <template v-if="clearable" #header-extra>
      <ConfirmDialog
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
    </template>
    <realtime-log v-if="show" ref="logRef" :path="props.path" />
  </n-modal>
</template>

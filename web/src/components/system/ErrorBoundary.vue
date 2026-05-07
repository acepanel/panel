<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

interface Props {
  fallbackTitle?: string
  fallbackDescription?: string
}

const props = withDefaults(defineProps<Props>(), {
  fallbackTitle: undefined,
  fallbackDescription: undefined,
})

const emit = defineEmits<{ (e: 'error', err: unknown): void }>()
const { $gettext } = useGettext()

const error = ref<Error | null>(null)

onErrorCaptured((err) => {
  error.value = err as Error
  emit('error', err)
  return false
})

const retry = () => {
  error.value = null
}
</script>

<template>
  <slot v-if="!error" />
  <n-result
    v-else
    status="error"
    :title="fallbackTitle ?? $gettext('Something went wrong')"
    :description="fallbackDescription ?? error?.message"
  >
    <template #footer>
      <n-button type="primary" @click="retry">{{ $gettext('Retry') }}</n-button>
    </template>
  </n-result>
</template>

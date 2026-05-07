<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

interface Props {
  loading?: boolean
  size?: 'tiny' | 'small' | 'medium'
  variant?: 'icon' | 'button'
  tooltip?: string
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  size: 'small',
  variant: 'icon',
  tooltip: undefined,
})

const emit = defineEmits<{ (e: 'refresh'): void }>()
const { $gettext } = useGettext()
</script>

<template>
  <n-tooltip v-if="variant === 'icon'" trigger="hover">
    <template #trigger>
      <n-button quaternary circle :size="size" :loading="loading" @click="emit('refresh')">
        <template #icon>
          <i-mdi-refresh />
        </template>
      </n-button>
    </template>
    {{ tooltip ?? $gettext('Refresh') }}
  </n-tooltip>
  <n-button v-else :size="size" :loading="loading" @click="emit('refresh')">
    <template #icon>
      <i-mdi-refresh />
    </template>
    {{ tooltip ?? $gettext('Refresh') }}
  </n-button>
</template>

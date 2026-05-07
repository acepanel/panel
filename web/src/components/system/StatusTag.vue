<script lang="ts" setup>
import type { StatusType } from './types'

interface Props {
  status: StatusType
  label?: string
  size?: 'tiny' | 'small' | 'medium' | 'large'
  dot?: boolean
  bordered?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  label: undefined,
  size: 'small',
  dot: false,
  bordered: false,
})

type NaiveType = 'default' | 'primary' | 'success' | 'warning' | 'error' | 'info'

const typeMap: Record<StatusType, NaiveType> = {
  success: 'success',
  warning: 'warning',
  error: 'error',
  info: 'info',
  neutral: 'default',
  running: 'success',
  stopped: 'default',
  partial: 'warning',
}

const dotColor = computed(() => {
  switch (props.status) {
    case 'success':
    case 'running':
      return 'var(--color-success-fg)'
    case 'warning':
    case 'partial':
      return 'var(--color-warning-fg)'
    case 'error':
      return 'var(--color-error-fg)'
    case 'info':
      return 'var(--color-info-fg)'
    case 'stopped':
    case 'neutral':
    default:
      return 'var(--color-text-tertiary)'
  }
})
</script>

<template>
  <n-tag :type="typeMap[status]" :size="size" :bordered="bordered">
    <template v-if="dot" #icon>
      <span class="rounded-full h-6 w-6 inline-block" :style="{ backgroundColor: dotColor }" />
    </template>
    <slot>{{ label ?? status }}</slot>
  </n-tag>
</template>

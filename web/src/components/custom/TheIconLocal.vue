<script lang="ts" setup>
import { renderLocalIcon } from '@/utils'

const props = withDefaults(defineProps<Props>(), {
  type: 'app',
  size: 14,
  color: undefined,
})

interface Props {
  type: string
  icon: string
  size?: number
  color?: string
}
</script>

<template>
  <component
    :is="renderLocalIcon(props.type, props.icon, { size: props.size, color: props.color })"
  />
</template>

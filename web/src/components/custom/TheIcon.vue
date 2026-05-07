<script lang="ts" setup>
import { renderIcon } from '@/utils'

const props = withDefaults(defineProps<Props>(), {
  size: 14,
  color: undefined,
})

interface Props {
  icon: string
  size?: number
  color?: string
}
</script>

<template>
  <component :is="renderIcon(props.icon, { size: props.size, color: props.color })" />
</template>

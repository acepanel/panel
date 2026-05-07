<script lang="ts" setup>
interface Props {
  type?: 'info' | 'success' | 'warning' | 'error'
  title?: string
  closable?: boolean
  showIcon?: boolean
  bordered?: boolean
}

withDefaults(defineProps<Props>(), {
  type: 'info',
  title: undefined,
  closable: false,
  showIcon: true,
  bordered: true,
})
</script>

<template>
  <n-alert
    :type="type"
    :title="title"
    :closable="closable"
    :show-icon="showIcon"
    :bordered="bordered"
  >
    <slot />
  </n-alert>
</template>

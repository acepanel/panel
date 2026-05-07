<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const { isFullscreen, toggle } = useFullscreen()
</script>

<template>
  <n-tooltip trigger="hover">
    <template #trigger>
      <n-icon mr-5 cursor-pointer size="20" @click="toggle">
        <i-mdi-fullscreen-exit v-if="isFullscreen" />
        <i-mdi-fullscreen v-else />
      </n-icon>
    </template>
    {{ $gettext('Fullscreen Display') }}
  </n-tooltip>
</template>

<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import { useThemeStore } from '@/stores'

const { $gettext } = useGettext()
const themeStore = useThemeStore()
</script>

<template>
  <n-tooltip trigger="hover">
    <template #trigger>
      <n-icon cursor-pointer size="22" @click="themeStore.toggleCollapsed()">
        <i-mdi-format-indent-increase v-if="themeStore.sider.collapsed" />
        <i-mdi-format-indent-decrease v-else />
      </n-icon>
    </template>
    {{ $gettext('Menu Zoom') }}
  </n-tooltip>
</template>

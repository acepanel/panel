<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import { useThemeStore } from '@/stores'

const { $gettext } = useGettext()
const theme = useThemeStore()
</script>

<template>
  <n-tooltip trigger="hover">
    <template #trigger>
      <n-icon class="mr-2 cursor-pointer" size="20" @click="theme.toggleDarkMode">
        <i-mdi-weather-sunny v-if="theme.darkMode" />
        <i-mdi-weather-night v-else />
      </n-icon>
    </template>
    {{ $gettext('Switch Theme') }}
  </n-tooltip>
</template>

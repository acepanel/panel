<script lang="ts" setup>
import MenuCollapse from '@/layouts/header/components/MenuCollapse.vue'
import ReloadPage from '@/layouts/header/components/ReloadPage.vue'
import StickyNote from '@/layouts/header/components/StickyNote.vue'
import TaskStatus from '@/layouts/header/components/TaskStatus.vue'
import AppTab from '@/layouts/tab/IndexView.vue'
import { useThemeStore } from '@/stores'

import FullScreen from './components/FullScreen.vue'
import ThemeMode from './components/ThemeMode.vue'
import UserAvatar from './components/UserAvatar.vue'

const themeStore = useThemeStore()
</script>

<template>
  <div class="flex w-full items-center justify-between">
    <menu-collapse v-if="themeStore.isMobile" />
    <section v-if="!themeStore.isMobile && themeStore.tab.visible" class="pr-3 flex-1 w-0">
      <app-tab />
    </section>
    <span
      v-if="!themeStore.isMobile && themeStore.tab.visible"
      class="text-text-tertiary mx-2 opacity-40"
      >|</span
    >
    <div class="ml-auto pl-2 flex flex-shrink-0 gap-1 items-center">
      <task-status />
      <sticky-note />
      <reload-page />
      <full-screen />
      <theme-mode />
      <user-avatar />
    </div>
  </div>
</template>

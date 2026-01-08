<script lang="ts" setup>
import ReloadPage from '@/layout/header/components/ReloadPage.vue'
import AppTab from '@/layout/tab/IndexView.vue'
import FullScreen from './components/FullScreen.vue'
import ThemeMode from './components/ThemeMode.vue'
import UserAvatar from './components/UserAvatar.vue'

import MenuCollapse from '@/layout/header/components/MenuCollapse.vue'
import { useThemeStore } from '@/store'

const themeStore = useThemeStore()
</script>

<template>
  <div     flex w-full items-center justify-between >
    <menu-collapse v-if="themeStore.isMobile" />
    <section v-if="!themeStore.isMobile && themeStore.tab.visible"    pr-12 flex-1 w-0 >
      <app-tab />
    </section>
    <span v-if="!themeStore.isMobile && themeStore.tab.visible" mx-6 opacity-20>|</span>
    <div      ml-auto px-12 flex flex-shrink-0 items-center >
      <reload-page />
      <full-screen />
      <theme-mode />
      <user-avatar />
    </div>
  </div>
</template>

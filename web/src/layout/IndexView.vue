<script lang="ts" setup>
import AppMain from './AppMain.vue'
import AppHeader from './header/IndexView.vue'
import SideBar from './sidebar/IndexView.vue'

import { useThemeStore } from '@/store'

const themeStore = useThemeStore()
</script>

<template>
  <n-layout has-sider wh-full>
    <n-layout-sider
      v-if="!themeStore.isMobile"
      :collapsed="themeStore.sider.collapsed"
      :collapsed-width="themeStore.sider.collapsedWidth"
      :native-scrollbar="false"
      :width="themeStore.sider.width"
      bordered
      collapse-mode="width"
    >
      <side-bar />
    </n-layout-sider>
    <n-drawer
      v-else
      :auto-focus="false"
      :show="!themeStore.sider.collapsed"
      :width="themeStore.sider.width"
      display-directive="show"
      placement="left"
      @mask-click="themeStore.setCollapsed(true)"
    >
      <n-scrollbar>
        <side-bar />
      </n-scrollbar>
    </n-drawer>

    <article flex-col flex-1 overflow-hidden>
      <header
        :style="`height: ${themeStore.header.height}px`"
        dark="bg-dark border-0"
        px-15
        border-b
        bg-white
        flex
        items-center
        bc-eee
      >
        <app-header />
      </header>
      <section bg="#f5f6fb" flex-1 overflow-hidden dark:bg-hex-101014>
        <app-main />
      </section>
    </article>
  </n-layout>
</template>

<style scoped lang="scss">
:deep(.n-scrollbar-content) {
  height: 100%;
}
</style>

<script lang="ts" setup>
import { useThemeStore } from '@/stores'

import AppMain from './AppMain.vue'
import AppHeader from './header/IndexView.vue'
import SideBar from './sidebar/IndexView.vue'

const themeStore = useThemeStore()

// 平板自动 collapsed
const handleResize = () => {
  const w = window.innerWidth
  if (w < 1024 && w >= 768 && !themeStore.sider.collapsed) {
    themeStore.setCollapsed(true)
  }
}
onMounted(() => {
  handleResize()
  window.addEventListener('resize', handleResize)
})
onBeforeUnmount(() => window.removeEventListener('resize', handleResize))
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

    <article class="flex flex-col flex-1 overflow-hidden">
      <header
        :style="`height: ${themeStore.header.height}px`"
        class="px-4 border-b border-border-default bg-bg-elevated flex items-center lg:px-6"
      >
        <app-header />
      </header>
      <section class="bg-bg-base flex flex-col flex-1 overflow-hidden">
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

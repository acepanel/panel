<script lang="ts" setup>
import { useResponsive } from '@/components/system/composables/useResponsive'
import { useThemeStore } from '@/stores'

import AppMain from './AppMain.vue'
import AppHeader from './header/IndexView.vue'
import SideBar from './sidebar/IndexView.vue'

const themeStore = useThemeStore()
const { breakpoint } = useResponsive()

// 平板自动折叠侧栏；手机上 collapsed 控制抽屉开合，持久化的 false 会让抽屉一进页面就弹出
watch(
  breakpoint,
  (value) => {
    if (value === 'mobile' || value === 'tablet') themeStore.setCollapsed(true)
  },
  { immediate: true },
)
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
      :width="240"
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
      <health-banner />
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

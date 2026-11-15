<script lang="ts" setup>
import { useWindowWidth } from '@/components/system/composables/useResponsive'
import { naiveBreakpoints } from '@/design'
import { useThemeStore } from '@/stores'

const themeStore = useThemeStore()

// 同步 html.dark class,初次挂载即生效
themeStore.applyDarkClass()

watch(
  () => themeStore.darkMode,
  () => themeStore.applyDarkClass(),
  { immediate: true },
)

// 阈值与 uno 的 md 断点对齐；immediate 让首次渲染前就判定，不沿用持久化的旧值
const windowWidth = useWindowWidth()
watch(windowWidth, (width) => themeStore.setIsMobile(width < naiveBreakpoints.m), {
  immediate: true,
})
</script>

<template>
  <n-config-provider
    :theme="themeStore.naiveTheme"
    :theme-overrides="themeStore.naiveThemeOverrides"
    :locale="themeStore.naiveLocale"
    :date-locale="themeStore.naiveDateLocale"
    :breakpoints="naiveBreakpoints"
    wh-full
  >
    <slot />
  </n-config-provider>
</template>

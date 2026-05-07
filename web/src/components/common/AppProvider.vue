<script lang="ts" setup>
import hljs from 'highlight.js/lib/core'
import log from 'highlight.js/lib/languages/accesslog'

import { useThemeStore } from '@/stores'
import systemdlog from '@/utils/hljs/systemdlog'

hljs.registerLanguage('accesslog', log)
hljs.registerLanguage('systemdlog', systemdlog)

const themeStore = useThemeStore()

// 同步 html.dark class,初次挂载即生效
themeStore.applyDarkClass()

watch(
  () => themeStore.darkMode,
  () => themeStore.applyDarkClass(),
  { immediate: true },
)

function handleWindowResize() {
  themeStore.setIsMobile(document.body.offsetWidth <= 768)
}

onMounted(() => {
  handleWindowResize()
  window.addEventListener('resize', handleWindowResize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', handleWindowResize)
})
</script>

<template>
  <n-config-provider
    :hljs="hljs"
    :theme="themeStore.naiveTheme"
    :theme-overrides="themeStore.naiveThemeOverrides"
    :locale="themeStore.naiveLocale"
    :date-locale="themeStore.naiveDateLocale"
    wh-full
  >
    <slot />
  </n-config-provider>
</template>

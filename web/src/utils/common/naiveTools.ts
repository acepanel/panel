import mitt from 'mitt'
import * as NaiveUI from 'naive-ui'

import { useThemeStore } from '@/stores'

export async function setupNaiveDiscreteApi() {
  const themeStore = useThemeStore()
  const configProviderProps = computed(() => ({
    theme: themeStore.naiveTheme,
    themeOverrides: themeStore.naiveThemeOverrides,
    locale: themeStore.naiveLocale,
    dateLocale: themeStore.naiveDateLocale,
  }))
  const { message, dialog, notification, loadingBar } = NaiveUI.createDiscreteApi(
    ['message', 'dialog', 'notification', 'loadingBar'],
    { configProviderProps },
  )

  window.$loadingBar = loadingBar
  window.$notification = notification
  window.$message = message
  window.$dialog = dialog
  window.$bus = mitt()
}

import '@/styles/index.scss'
import '@/styles/reset.css'
import 'virtual:uno.css'
import { createApp } from 'vue'

import home from '@/api/panel/home'
import { setupRouter } from '@/router'
import { setupStore, usePermissionStore, useThemeStore } from '@/stores'
import { gettext, setCurrent, setupNaiveDiscreteApi } from '@/utils'

import App from './App.vue'

async function setupApp() {
  const app = createApp(App)
  await setupStore(app)
  await setupNaiveDiscreteApi()

  await setupPanel()
  app.use(gettext)

  await setupRouter(app)
  app.mount('#app')
}

const setupPanel = async () => {
  const themeStore = useThemeStore()
  const permissionStore = usePermissionStore()
  setCurrent(themeStore.locale)

  // 请求失败时沿用本地已存的语言和名称继续启动，错误提示由全局拦截器弹出
  const data = await home
    .panel()
    .send()
    .catch(() => null)
  if (!data) return

  setCurrent(data.locale)
  themeStore.setLocale(data.locale)
  themeStore.setName(data.name)
  themeStore.setLogo(data.custom_logo || '')
  permissionStore.setHiddenRoutes(data.hidden_menu || [])
}

setupApp()

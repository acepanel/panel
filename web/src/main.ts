import '@/styles/index.scss'
import '@/styles/reset.css'
import 'virtual:uno.css'

import { createApp } from 'vue'
import App from './App.vue'

import { setupRouter } from '@/router'
import { setupStore, usePermissionStore, useThemeStore } from '@/store'
import { gettext, setCurrent, setupNaiveDiscreteApi } from '@/utils'

import home from '@/api/panel/home'

async function setupApp() {
  const app = createApp(App)
  await setupStore(app)
  await setupNaiveDiscreteApi()

  await setupPanel().then(() => {
    app.use(gettext)
  })

  await setupRouter(app)
  app.mount('#app')
}

const setupPanel = async () => {
  const themeStore = useThemeStore()
  const permissionStore = usePermissionStore()
  setCurrent(themeStore.locale)

  return new Promise<void>((resolve) => {
    useRequest(home.panel, {
      initialData: {
        name: import.meta.env.VITE_APP_TITLE,
        locale: 'en',
        hidden_menu: [],
        custom_logo: ''
      }
    }).onSuccess(async ({ data }: { data: any }) => {
      setCurrent(data.locale)
      themeStore.setLocale(data.locale)
      themeStore.setName(data.name)
      // 设置隐藏菜单和自定义 Logo
      themeStore.setLogo(data.custom_logo || '')
      permissionStore.setHiddenRoutes(data.hidden_menu || [])

      resolve()
    })
  })
}

setupApp()

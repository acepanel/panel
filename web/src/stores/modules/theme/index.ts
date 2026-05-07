import type { GlobalTheme, GlobalThemeOverrides, NDateLocale, NLocale } from 'naive-ui'
import {
  darkTheme,
  dateEnUS,
  dateJaJP,
  dateZhCN,
  dateZhTW,
  enUS,
  jaJP,
  lightTheme,
  zhCN,
  zhTW,
} from 'naive-ui'

import {
  darkSemantic,
  darkThemeOverrides,
  lightSemantic,
  lightThemeOverrides,
  type SemanticPalette,
} from '@/design'

import { defaultSettings } from './helpers'

type ThemeState = Theme.Setting

const locales: Record<string, { locale: NLocale; dateLocale: NDateLocale }> = {
  en: { locale: enUS, dateLocale: dateEnUS },
  zh_CN: { locale: zhCN, dateLocale: dateZhCN },
  zh_TW: { locale: zhTW, dateLocale: dateZhTW },
  ja_JP: { locale: jaJP, dateLocale: dateJaJP },
}

export const useThemeStore = defineStore('theme', {
  state: (): ThemeState => defaultSettings(),
  getters: {
    naiveTheme(): GlobalTheme {
      return this.darkMode ? darkTheme : lightTheme
    },
    naiveThemeOverrides(): GlobalThemeOverrides {
      return this.darkMode ? darkThemeOverrides : lightThemeOverrides
    },
    semanticColors(state): SemanticPalette {
      return state.darkMode ? darkSemantic : lightSemantic
    },
    naiveLocale(): NLocale {
      return locales[this.locale]?.locale ?? enUS
    },
    naiveDateLocale(): NDateLocale {
      return locales[this.locale]?.dateLocale ?? dateEnUS
    },
  },
  actions: {
    setIsMobile(isMobile: boolean) {
      this.isMobile = isMobile
    },
    /** 设置暗黑模式 */
    setDarkMode(darkMode: boolean) {
      this.darkMode = darkMode
      this.applyDarkClass()
    },
    /** 切换/关闭 暗黑模式 */
    toggleDarkMode() {
      this.setDarkMode(!this.darkMode)
    },
    /** 同步 html.dark class,作为 CSS 变量切换的中枢 */
    applyDarkClass() {
      if (typeof document === 'undefined') return
      document.documentElement.classList.toggle('dark', this.darkMode)
    },
    /** 切换/关闭 折叠侧边栏 */
    toggleCollapsed() {
      this.sider.collapsed = !this.sider.collapsed
    },
    /** 设置 折叠侧边栏 */
    setCollapsed(collapsed: boolean) {
      this.sider.collapsed = collapsed
    },
    /** 设置语言 */
    setLocale(locale: string) {
      this.locale = locale
    },
    /** 设置名称 */
    setName(name: string) {
      this.name = name
    },
    /** 设置 Logo */
    setLogo(logo: string) {
      this.logo = logo
    },
  },
  persist: {
    pick: ['isMobile', 'darkMode', 'sider', 'header', 'tab', 'locale', 'name'],
  },
})

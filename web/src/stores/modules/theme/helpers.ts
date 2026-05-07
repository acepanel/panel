import themeSetting from '~/config/theme.json'

/** 初始化主题配置 */
export function defaultSettings(): Theme.Setting {
  const isMobile = themeSetting.isMobile || false
  const darkMode = themeSetting.darkMode || false
  const sider = themeSetting.sider || {
    width: 160,
    collapsedWidth: 64,
    collapsed: false,
  }
  const header = themeSetting.header || { visible: true, height: 60 }
  const tab = themeSetting.tab || { visible: true, height: 50 }
  const locale = themeSetting.locale || 'zh_CN'
  const name = themeSetting.name || import.meta.env.VITE_APP_TITLE
  const logo = ''
  return { isMobile, darkMode, sider, header, tab, locale, name, logo }
}

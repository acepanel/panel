import { createGettext as vue3Gettext } from 'vue3-gettext'

import translations from '@/locales/translations.json'

export const locales = {
  en: 'English',
  zh_CN: '简体中文',
  zh_TW: '繁體中文',
}

export const gettext: any = vue3Gettext({
  availableLanguages: locales,
  defaultLanguage: 'zh_CN',
  translations: translations,
  silent: true,
})

export function $gettext(msgid: string, params?: Record<string, string | number>) {
  return gettext.$gettext(msgid, params)
}

export function $ngettext(
  msgid: string,
  plural: string,
  n: number,
  params?: Record<string, string | number>,
) {
  return gettext.$ngettext(msgid, plural, n, params)
}

export function setCurrent(language: string) {
  gettext.current = language
}

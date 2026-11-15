import { useGettext } from 'vue3-gettext'

import { toOptions } from '@/utils/tune'

// frps 与 frpc 的认证、日志选项必须一致，集中在此避免两边漏改
export const authMethodOptions = toOptions(['token', 'oidc'])

export const logLevelOptions = toOptions(['trace', 'debug', 'info', 'warn', 'error'])

export const useBoolOptions = () => {
  const { $gettext } = useGettext()
  return [
    { label: $gettext('Enabled'), value: 'true' },
    { label: $gettext('Disabled'), value: 'false' },
  ]
}

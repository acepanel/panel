import { useUserStore } from '@/stores'
import { $gettext } from '@/utils/gettext'

export function resolveResError(code: number | string | undefined, msg = ''): string {
  switch (code) {
    case 400:
    case 422:
      return msg || $gettext('Invalid request parameters')
    case 401:
      useUserStore().logout()
      return msg || $gettext('Login expired')
    case 418:
      useUserStore().refresh()
      return msg || $gettext('Login expired')
    case 403:
      return msg || $gettext('Permission denied')
    case 404:
      return msg || $gettext('Resource not found')
    case 500:
      return msg || $gettext('Server error')
    default:
      return msg || $gettext('Unknown error') + ` (${code})`
  }
}

// 适配器对断网和超时抛的是固定的英文原文，换成可翻译的文案
export function resolveNetworkError(error: any): string {
  switch (error?.message) {
    case 'Network Error':
      return $gettext('Network error, please check your connection')
    case 'Network Timeout':
      return $gettext('Request timed out')
    default:
      return error?.message || $gettext('Unknown error')
  }
}

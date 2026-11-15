import { DateTime } from 'luxon'

import { $gettext } from '@/utils/gettext'

/** 格式化时间，默认格式：yyyy-MM-dd HH:mm:ss */
export function formatDateTime(time: any, format = 'yyyy-MM-dd HH:mm:ss'): string {
  const dateTime = time ? DateTime.fromJSDate(new Date(time)) : DateTime.now()
  return dateTime.toFormat(format)
}

/** 格式化持续时间，为 0 的高位单位省略 */
export function formatDuration(seconds: number) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  // 提取工具识别不到模板字符串里的 $gettext，所以用加号拼接
  const parts: string[] = []
  if (d > 0) parts.push(d + ' ' + $gettext('days'))
  if (h > 0) parts.push(h + ' ' + $gettext('hours'))
  if (m > 0) parts.push(m + ' ' + $gettext('min'))
  parts.push(s + ' ' + $gettext('sec'))
  return parts.join(' ')
}

/** 转时间戳 */
export function toTimestamp(time: any) {
  return DateTime.fromJSDate(new Date(time)).toSeconds()
}

/** 生成随机字符串 */
export function generateRandomString(length: number) {
  const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  const charactersLength = characters.length
  let result = ''
  if (!window.crypto || !window.crypto.getRandomValues) {
    // fallback to insecure Math.random if crypto is not available
    for (let i = 0; i < length; i++) {
      const randomIndex = Math.floor(Math.random() * charactersLength)
      result += characters[randomIndex]
    }
    return result
  }
  // Use Uint8Array for random bytes
  while (result.length < length) {
    const randomBytes = new Uint8Array(length - result.length)
    window.crypto.getRandomValues(randomBytes)
    for (let i = 0; i < randomBytes.length && result.length < length; i++) {
      // Only use values that map evenly to the character set to avoid bias
      const maxValue = Math.floor(256 / charactersLength) * charactersLength
      const byteValue = randomBytes[i] ?? 0
      if (byteValue < maxValue) {
        const randomIndex = byteValue % charactersLength
        result += characters[randomIndex] ?? ''
      }
    }
  }
  return result
}

/** 为裸 IPv6 地址套上方括号（如 ::1 → [::1]），用于拼接 URL；非 IPv6 或已带方括号则原样返回 */
export function wrapIPv6(host: string): string {
  return host.includes(':') && !host.startsWith('[') ? `[${host}]` : host
}

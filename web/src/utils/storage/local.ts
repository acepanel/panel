interface StorageData {
  value: any
  expire: number | null
}

/** 默认保存期限为永久 */
const DEFAULT_CACHE_TIME = 0

export function setLocal(key: string, value: any, expire: number | null = DEFAULT_CACHE_TIME) {
  const storageData: StorageData = {
    value,
    expire: expire !== 0 && expire !== null ? new Date().getTime() + expire * 1000 : 0,
  }
  const json = JSON.stringify(storageData)
  window.localStorage.setItem(key, json)
}

export function getLocal<T>(key: string) {
  const json = window.localStorage.getItem(key)
  if (json) {
    const storageData = JSON.parse(json)
    if (storageData) {
      const { value, expire } = storageData
      // 没有过期时间或者在有效期内则直接返回
      if (expire === 0 || expire >= Date.now()) return value as T
    }
    removeLocal(key)
    return null
  }
  return null
}

export function getLocalExpire(key: string): number | null {
  const json = window.localStorage.getItem(key)
  if (json) {
    const storageData = JSON.parse(json)
    if (storageData) {
      return storageData.expire
    }
    return null
  }
  return null
}

export function removeLocal(key: string) {
  window.localStorage.removeItem(key)
}

export function clearLocal() {
  window.localStorage.clear()
}

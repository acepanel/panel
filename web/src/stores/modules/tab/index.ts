import { router } from '@/router'

export const WITHOUT_TAB_PATHS = ['/404', '/login']

export interface Tab {
  active: string
  reloading: boolean
  tabs: Array<TabItem>
}

export interface TabItem {
  name: string
  path: string
  title: string
  keepAlive: boolean
}

export const useTabStore = defineStore('tab', {
  state: (): Tab => {
    return {
      active: '',
      reloading: false,
      tabs: [],
    }
  },
  actions: {
    async setActiveTab(path: string) {
      await nextTick()
      this.active = path
    },
    setTabs(tabs: Array<TabItem>) {
      this.tabs = tabs
    },
    addTab(tab: TabItem) {
      this.setActiveTab(tab.path)
      if (WITHOUT_TAB_PATHS.includes(tab.path) || this.tabs.some((item) => item.path === tab.path))
        return
      this.setTabs([...this.tabs, tab])
    },
    async reloadTab(path: string) {
      const findItem = this.tabs.find((item) => item.path === path)
      if (!findItem) return
      const keepLive = findItem.keepAlive
      findItem.keepAlive = false // 取消keepAlive
      window.$loadingBar.start()
      this.reloading = true
      await nextTick()
      this.reloading = false
      await nextTick()
      findItem.keepAlive = keepLive // 恢复keepAlive原状态
      setTimeout(() => {
        document.documentElement.scrollTo({ left: 0, top: 0 })
        window.$loadingBar.finish()
      }, 100)
    },
    pinTab(path: string) {
      const findItem = this.tabs.find((item) => item.path === path)
      if (findItem) findItem.keepAlive = true
    },
    unpinTab(path: string) {
      const findItem = this.tabs.find((item) => item.path === path)
      if (findItem) findItem.keepAlive = false
    },
    removeTab(path: string) {
      if (path === this.active) {
        const activeIndex = this.tabs.findIndex((item) => item.path === path)
        if (activeIndex > 0) router.push(this.tabs[activeIndex - 1]?.path ?? '/')
        else router.push(this.tabs[activeIndex + 1]?.path ?? '/')
      }
      this.setTabs(this.tabs.filter((tab) => tab.path !== path))
    },
    removeOther(curPath: string) {
      this.setTabs(this.tabs.filter((tab) => tab.path === curPath))
      if (curPath !== this.active) router.push(this.tabs[this.tabs.length - 1]?.path ?? '/')
    },
    removeLeft(curPath: string) {
      const curIndex = this.tabs.findIndex((item) => item.path === curPath)
      const filterTabs = this.tabs.filter((item, index) => index >= curIndex)
      this.setTabs(filterTabs)
      if (!filterTabs.find((item) => item.path === this.active))
        router.push(filterTabs[filterTabs.length - 1]?.path ?? '/')
    },
    removeRight(curPath: string) {
      const curIndex = this.tabs.findIndex((item) => item.path === curPath)
      const filterTabs = this.tabs.filter((item, index) => index <= curIndex)
      this.setTabs(filterTabs)
      if (!filterTabs.find((item) => item.path === this.active))
        router.push(filterTabs[filterTabs.length - 1]?.path ?? '/')
    },
    resetTabs() {
      this.setTabs([])
      this.setActiveTab('')
    },
  },
  persist: {
    storage: sessionStorage,
  },
})

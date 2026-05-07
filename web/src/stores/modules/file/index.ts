import type { Marked } from '@/views/file/types'

export interface FileTab {
  id: string // 标签页唯一标识
  label: string // 显示名（路径末级目录名）
  path: string // 当前路径
  keyword: string // 搜索关键词
  sub: boolean // 搜索是否包含子目录
  history: string[] // 浏览历史栈
  historyCursor: number // 历史指针
}

export interface FileState {
  tabs: FileTab[]
  activeTabId: string
  // 全局偏好（跨标签页共享）
  showHidden: boolean
  viewType: 'list' | 'grid'
  sortKey: string
  sortOrder: 'asc' | 'desc'
  // 全局剪贴板（跨标签页共享）
  clipboard: {
    marked: Marked[]
    markedType: 'copy' | 'move'
  }
}

// 根据路径生成标签页显示名
const getLabelFromPath = (path: string): string => {
  if (path === '/') return '/'
  return path.split('/').pop() || '/'
}

const generateTabId = (): string => `tab-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

// 创建新标签页
const createNewTab = (path: string): FileTab => ({
  id: generateTabId(),
  label: getLabelFromPath(path),
  path,
  keyword: '',
  sub: false,
  history: [path],
  historyCursor: 0,
})

export const useFileStore = defineStore('file', {
  state: (): FileState => {
    const initialTab = createNewTab('/opt')
    return {
      tabs: [initialTab],
      activeTabId: initialTab.id,
      showHidden: false,
      viewType: 'list',
      sortKey: '',
      sortOrder: 'asc',
      clipboard: {
        marked: [],
        markedType: 'copy',
      },
    }
  },
  getters: {
    sort(): string {
      if (!this.sortKey) return ''
      return this.sortOrder === 'desc' ? `-${this.sortKey}` : this.sortKey
    },
    activeTab(): FileTab | undefined {
      return this.tabs.find((t) => t.id === this.activeTabId)
    },
  },
  actions: {
    // 新建标签页
    createTab(path?: string) {
      const tabPath = path ?? this.activeTab?.path ?? '/opt'
      const tab = createNewTab(tabPath)
      this.tabs.push(tab)
      this.activeTabId = tab.id
    },
    // 关闭标签页
    closeTab(tabId: string) {
      if (this.tabs.length <= 1) return
      const index = this.tabs.findIndex((t) => t.id === tabId)
      if (index === -1) return
      this.tabs.splice(index, 1)
      // 如果关闭的是当前活跃标签页，切换到相邻标签页
      if (this.activeTabId === tabId) {
        const newIndex = Math.min(index, this.tabs.length - 1)
        this.activeTabId = this.tabs[newIndex]!.id
      }
    },
    // 切换标签页
    switchTab(tabId: string) {
      if (this.tabs.some((t) => t.id === tabId)) {
        this.activeTabId = tabId
      }
    },
    // 更新标签页路径
    updateTabPath(tabId: string, path: string) {
      const tab = this.tabs.find((t) => t.id === tabId)
      if (!tab) return
      tab.path = path
      tab.label = getLabelFromPath(path)
      tab.keyword = ''
      tab.sub = false
      this.pushHistory(tabId, path)
    },
    // 推入历史记录
    pushHistory(tabId: string, path: string) {
      const tab = this.tabs.find((t) => t.id === tabId)
      if (!tab) return
      // 如果当前位置就是这个路径，不重复推入
      if (tab.history[tab.historyCursor] === path) return
      // 截断 cursor 后的 future
      tab.history.splice(tab.historyCursor + 1)
      tab.history.push(path)
      tab.historyCursor = tab.history.length - 1
      // 限制历史栈长度，防止无限增长
      if (tab.history.length > 100) {
        const excess = tab.history.length - 100
        tab.history.splice(0, excess)
        tab.historyCursor -= excess
      }
    },
    // 历史后退
    historyBack(tabId: string) {
      const tab = this.tabs.find((t) => t.id === tabId)
      if (!tab || tab.historyCursor <= 0) return
      tab.historyCursor--
      tab.path = tab.history[tab.historyCursor] ?? '/'
      tab.label = getLabelFromPath(tab.path)
      tab.keyword = ''
      tab.sub = false
    },
    // 历史前进
    historyForward(tabId: string) {
      const tab = this.tabs.find((t) => t.id === tabId)
      if (!tab || tab.historyCursor >= tab.history.length - 1) return
      tab.historyCursor++
      tab.path = tab.history[tab.historyCursor] ?? '/'
      tab.label = getLabelFromPath(tab.path)
      tab.keyword = ''
      tab.sub = false
    },
    // 设置剪贴板
    setClipboard(marked: Marked[], markedType: 'copy' | 'move') {
      this.clipboard.marked = marked
      this.clipboard.markedType = markedType
    },
    // 清空剪贴板
    clearClipboard() {
      this.clipboard.marked = []
    },
    toggleShowHidden() {
      this.showHidden = !this.showHidden
    },
    toggleViewType() {
      this.viewType = this.viewType === 'list' ? 'grid' : 'list'
    },
    setSort(key: string) {
      if (this.sortKey === key) {
        if (this.sortOrder === 'asc') {
          this.sortOrder = 'desc'
        } else {
          this.sortKey = ''
          this.sortOrder = 'asc'
        }
      } else {
        this.sortKey = key
        this.sortOrder = 'asc'
      }
    },
  },
  persist: {
    afterHydrate(ctx: any) {
      const store = ctx.store as ReturnType<typeof useFileStore>
      // 恢复后清空剪贴板
      store.clipboard = { marked: [], markedType: 'copy' }
      // 确保 activeTabId 有效
      if (!store.tabs || store.tabs.length === 0) {
        const tab = createNewTab('/opt')
        store.tabs = [tab]
        store.activeTabId = tab.id
      } else if (!store.tabs.some((t) => t.id === store.activeTabId)) {
        store.activeTabId = store.tabs[0]!.id
      }
    },
  },
})

export interface File {
  path: string
  keyword: string
  sub: boolean
  showHidden: boolean
  viewType: 'list' | 'grid'
  sortKey: string
  sortOrder: 'asc' | 'desc'
}

export const useFileStore = defineStore('file', {
  state: (): File => {
    return {
      path: '/opt',
      keyword: '',
      sub: false,
      showHidden: false,
      viewType: 'list',
      sortKey: '',
      sortOrder: 'asc'
    }
  },
  getters: {
    sort(): string {
      if (!this.sortKey) return ''
      return this.sortOrder === 'desc' ? `-${this.sortKey}` : this.sortKey
    }
  },
  actions: {
    set(info: File) {
      this.path = info.path
      this.keyword = info.keyword
      this.sub = info.sub
      this.showHidden = info.showHidden
      this.viewType = info.viewType
      this.sortKey = info.sortKey
      this.sortOrder = info.sortOrder
    },
    toggleShowHidden() {
      this.showHidden = !this.showHidden
    },
    toggleViewType() {
      this.viewType = this.viewType === 'list' ? 'grid' : 'list'
    },
    setSort(key: string) {
      if (this.sortKey === key) {
        // 同一列：切换排序方向，或取消排序
        if (this.sortOrder === 'asc') {
          this.sortOrder = 'desc'
        } else {
          this.sortKey = ''
          this.sortOrder = 'asc'
        }
      } else {
        // 不同列：设置新的排序列
        this.sortKey = key
        this.sortOrder = 'asc'
      }
    }
  },
  persist: true
})

export interface File {
  path: string
  keyword: string
  sub: boolean
  showHidden: boolean
  viewType: 'list' | 'grid'
}

export const useFileStore = defineStore('file', {
  state: (): File => {
    return {
      path: '/opt',
      keyword: '',
      sub: false,
      showHidden: false,
      viewType: 'list'
    }
  },
  actions: {
    set(info: File) {
      this.path = info.path
      this.keyword = info.keyword
      this.sub = info.sub
      this.showHidden = info.showHidden
      this.viewType = info.viewType
    },
    toggleShowHidden() {
      this.showHidden = !this.showHidden
    },
    toggleViewType() {
      this.viewType = this.viewType === 'list' ? 'grid' : 'list'
    }
  },
  persist: true
})

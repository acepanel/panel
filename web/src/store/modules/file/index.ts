export interface File {
  path: string
  keyword: string
  sub: boolean
  showHidden: boolean
}

export const useFileStore = defineStore('file', {
  state: (): File => {
    return {
      path: '/opt',
      keyword: '',
      sub: false,
      showHidden: false
    }
  },
  actions: {
    set(info: File) {
      this.path = info.path
      this.keyword = info.keyword
      this.sub = info.sub
      this.showHidden = info.showHidden
    },
    toggleShowHidden() {
      this.showHidden = !this.showHidden
    }
  },
  persist: true
})

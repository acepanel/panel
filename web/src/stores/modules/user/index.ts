import { resetRouter } from '@/router'
import { usePermissionStore, useTabStore } from '@/stores'
import { toLogin } from '@/utils'

export interface UserInfo {
  id?: string
  username?: string
  role?: Array<string>
}

export const useUserStore = defineStore('user', {
  state: (): UserInfo => {
    return {
      id: '',
      username: '',
      role: [],
    }
  },
  actions: {
    set(info: UserInfo) {
      this.id = info.id
      this.username = info.username
      this.role = info.role
    },
    logout() {
      const { resetTabs } = useTabStore()
      const { resetPermission } = usePermissionStore()
      resetPermission()
      resetTabs()
      resetRouter()
      this.$reset()
      toLogin()
    },
    refresh() {
      const { resetTabs } = useTabStore()
      const { resetPermission } = usePermissionStore()
      resetPermission()
      resetTabs()
      resetRouter()
      this.$reset()
      window.location.reload()
    },
  },
  persist: true,
})

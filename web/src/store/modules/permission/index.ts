import { asyncRoutes, basicRoutes } from '@/router/routes'
import type { RoutesType } from '~/types/router'
import { filterAsyncRoutes } from './helpers'

export const usePermissionStore = defineStore('permission', {
  state() {
    return {
      accessRoutes: <RoutesType>[],
      hiddenRoutes: <string[]>[]
    }
  },
  getters: {
    routes(): RoutesType {
      return basicRoutes.concat(this.accessRoutes)
    },
    menus(): RoutesType {
      return this.routes
        .filter((route) => route.name && !route.isHidden && !this.hiddenRoutes.includes(route.name))
        .sort((a, b) => (a.meta?.order || 0) - (b.meta?.order || 0))
    },
    allMenus(): RoutesType {
      return this.routes
        .filter((route) => route.name && !route.isHidden)
        .sort((a, b) => (a.meta?.order || 0) - (b.meta?.order || 0))
    }
  },
  actions: {
    generateRoutes(role: Array<string> = []): RoutesType {
      const accessRoutes = filterAsyncRoutes(asyncRoutes, role)
      this.accessRoutes = accessRoutes
      return accessRoutes
    },
    resetPermission() {
      this.$reset()
    },
    /** 设置隐藏的菜单 */
    setHiddenRoutes(hiddenRoutes: string[]) {
      this.hiddenRoutes = hiddenRoutes
    }
  }
})

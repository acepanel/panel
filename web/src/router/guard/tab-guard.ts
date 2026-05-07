import type { Router } from 'vue-router'

import { useTabStore } from '@/stores'

export const EXCLUDE_TAB = ['/404', '/403', '/login']

export function createTabGuard(router: Router) {
  router.afterEach((to) => {
    if (EXCLUDE_TAB.includes(to.path)) return
    const tabStore = useTabStore()
    const { name, path } = to
    const title = String(to.meta?.title)
    tabStore.addTab({
      name: String(name),
      path,
      title,
      keepAlive: false,
    })
  })
}

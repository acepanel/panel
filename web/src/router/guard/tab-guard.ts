import type { Router } from 'vue-router'

import { useTabStore } from '@/stores'

const EXCLUDE_TAB = ['/404', '/login']

export function createTabGuard(router: Router) {
  router.afterEach((to) => {
    if (EXCLUDE_TAB.includes(to.path)) return
    const tabStore = useTabStore()
    const { path } = to
    const title = String(to.meta?.title)
    tabStore.addTab({
      path,
      title,
      keepAlive: to.meta?.keepAlive === true,
    })
  })
}

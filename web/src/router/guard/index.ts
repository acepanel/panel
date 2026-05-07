import type { Router } from 'vue-router'

import { createTabGuard } from '@/router/guard/tab-guard'

import { createAppInstallGuard } from './app-install-guard'
import { createPageLoadingGuard } from './page-loading-guard'
import { createPageTitleGuard } from './page-title-guard'

export function setupRouterGuard(router: Router) {
  createPageLoadingGuard(router)
  createPageTitleGuard(router)
  createTabGuard(router)
  createAppInstallGuard(router)
}

import type { Router } from 'vue-router'

import app from '@/api/panel/app'

// 防止重复显示错误消息
let lastErrorMsg = ''
let lastErrorTime = 0
const ERROR_COOLDOWN = 2000

function showErrorMessage(message: string) {
  const now = Date.now()
  if (lastErrorMsg !== message || now - lastErrorTime > ERROR_COOLDOWN) {
    window.$message.error(message)
    lastErrorMsg = message
    lastErrorTime = now
  }
}

export function createAppInstallGuard(router: Router) {
  router.beforeEach(async (to) => {
    const slug = to.path.split('/').pop()
    if (to.path.startsWith('/apps/') && slug) {
      await useRequest(app.isInstalled(slug)).onSuccess(({ data }) => {
        if (!data) {
          showErrorMessage(`应用未安装`)
          return router.push({ name: 'app-index' })
        }
      })
    }

    // 网站
    if (to.path.startsWith('/website')) {
      await useRequest(app.isInstalled('nginx,openresty,apache,openlitespeed,caddy')).onSuccess(
        ({ data }) => {
          if (!data) {
            showErrorMessage(`Web 服务器未安装`)
            return router.push({ name: 'app-index' })
          }
        },
      )
    }

    // 容器
    if (to.path.startsWith('/container')) {
      await useRequest(app.isInstalled('docker,podman')).onSuccess(({ data }) => {
        if (!data) {
          showErrorMessage(`容器引擎未安装`)
          return router.push({ name: 'app-index' })
        }
      })
    }
  })
}

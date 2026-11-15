import type { Router } from 'vue-router'

import app from '@/api/panel/app'
import { $gettext, WEBSERVER_SLUGS } from '@/utils'

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

// 页面依赖的应用（逗号分隔，装了任意一个即可）与未安装时的提示
function requirement(path: string): [slugs: string, message: string] | null {
  const slug = path.split('/').pop()
  if (path.startsWith('/apps/') && slug) return [slug, $gettext('App not installed')]
  if (path.startsWith('/website')) return [WEBSERVER_SLUGS, $gettext('Web server not installed')]
  if (path.startsWith('/container'))
    return ['docker,podman', $gettext('Container engine not installed')]
  return null
}

export function createAppInstallGuard(router: Router) {
  router.beforeEach(async (to) => {
    const required = requirement(to.path)
    if (!required) return
    const [slugs, message] = required
    // 请求失败时放行，错误提示由全局拦截器弹出
    const installed = await app.isInstalled(slugs).catch(() => true)
    if (installed) return
    showErrorMessage(message)
    return { name: 'app-index' }
  })
}

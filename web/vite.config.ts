import { defineConfig, loadEnv } from 'vite'

import { setupVitePlugins } from './config/plugins.ts'
import { createViteProxy } from './config/proxy.ts'
import { convertEnv, getRootPath, getSrcPath } from './config/utils.ts'

export default defineConfig(({ mode }) => {
  const srcPath = getSrcPath()
  const rootPath = getRootPath()

  const viteEnv = convertEnv(loadEnv(mode, process.cwd()))

  const { VITE_PORT, VITE_PUBLIC_PATH, VITE_USE_PROXY, VITE_PROXY_TYPE } = viteEnv
  return {
    base: VITE_PUBLIC_PATH,
    resolve: {
      alias: {
        '~': rootPath,
        '@': srcPath,
        // 剔除 vue3-gettext 附带的 pofile
        pofile: `${rootPath}/config/pofile-stub.ts`
      }
    },
    plugins: setupVitePlugins(),
    server: {
      host: '0.0.0.0',
      port: VITE_PORT,
      open: false,
      proxy: createViteProxy(VITE_USE_PROXY, VITE_PROXY_TYPE as ProxyType)
    },
    build: {
      reportCompressedSize: false,
      sourcemap: false,
      chunkSizeWarningLimit: 1024
    }
  }
})

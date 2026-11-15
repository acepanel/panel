import type { PluginOption } from 'vite'
import vue from '@vitejs/plugin-vue'
import unocss from 'unocss/vite'
import { compression } from 'vite-plugin-compression2'
import vueDevTools from 'vite-plugin-vue-devtools'

import { setupMdiIcons } from './icons.ts'
import unplugins from './unplugin.ts'

export function setupVitePlugins(): PluginOption[] {
  return [
    vue(),
    vueDevTools(),
    ...unplugins,
    unocss(),
    setupMdiIcons(),
    compression({
      algorithms: ['brotliCompress'],
      deleteOriginalAssets: true,
      skipIfLargerOrEqual: true
    })
  ]
}

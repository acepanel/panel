import { globSync, readFileSync } from 'node:fs'

import { icons as mdi, type IconifyJSON } from '@iconify-json/mdi'
import type { Plugin } from 'vite'

const id = 'virtual:mdi-icons'
const resolvedId = `\0${id}`

// 完整 mdi 图标集有 3MB，这里只收源码里以 mdi:xxx、mdi-xxx 字面量出现的图标
// <i-mdi-xxx /> 是 unplugin-icons 的编译期组件，不走运行时图标集
function collect(): IconifyJSON {
  const { prefix, width, height, icons, aliases = {} } = mdi
  const subset: IconifyJSON = { prefix, width, height, icons: {}, aliases: {} }
  const add = (name: string) => {
    if (icons[name]) subset.icons[name] = icons[name]
    else if (aliases[name]) {
      subset.aliases![name] = aliases[name]
      add(aliases[name].parent)
    }
  }
  for (const file of globSync('src/**/*.{vue,ts}')) {
    const names = readFileSync(file, 'utf8').matchAll(/(?<!i-)mdi[:-]([a-z\d]+(?:-[a-z\d]+)*)/g)
    for (const [, name] of names) add(name)
  }
  return subset
}

export function setupMdiIcons(): Plugin {
  return {
    name: 'mdi-icons',
    resolveId: (source) => (source === id ? resolvedId : undefined),
    load: (loadId) => (loadId === resolvedId ? `export default ${JSON.stringify(collect())}` : undefined),
    // 开发时新增图标，刷新页面即重新扫描
    handleHotUpdate({ file, server }) {
      const mod = /\.(vue|ts)$/.test(file) && server.moduleGraph.getModuleById(resolvedId)
      if (mod) server.moduleGraph.invalidateModule(mod)
    }
  }
}

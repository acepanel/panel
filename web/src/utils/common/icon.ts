import { icons as mdi } from '@iconify-json/mdi'
import { addCollection, Icon } from '@iconify/vue'
import { NIcon } from 'naive-ui'

addCollection(mdi)

const localIcons = import.meta.glob<string>('@/assets/icons/**/*.svg', {
  eager: true,
  query: '?raw',
  import: 'default',
})

function getLocalIconSvg(type: string, icon: string): string {
  const path = `/src/assets/icons/${type}/${icon}.svg`
  const defaultPath = `/src/assets/icons/${type}/${type}.svg`

  return localIcons[path] ?? localIcons[defaultPath] ?? ''
}

interface Props {
  size?: number
  color?: string
  class?: string
}

export function renderIcon(icon: string, props: Props = { size: 12 }) {
  return () => h(NIcon, props, { default: () => h(Icon, { icon }) })
}

export function renderLocalIcon(type: string, icon: string, props: Props = { size: 12 }) {
  const svgContent = getLocalIconSvg(type, icon)
  return () => h(NIcon, { ...props, innerHTML: svgContent })
}

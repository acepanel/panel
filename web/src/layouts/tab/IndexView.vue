<script lang="ts" setup>
import { useThemeVars } from 'naive-ui'

import { translateTitle } from '@/locales/menu'
import type { TabItem } from '@/stores'
import { useTabStore } from '@/stores'

import ContextMenu from './components/ContextMenu.vue'

const themeVars = useThemeVars()
const router = useRouter()
const tabStore = useTabStore()

interface ContextMenuOption {
  show: boolean
  keepAlive: boolean
  x: number
  y: number
  currentPath: string
}

const contextMenuOption = reactive<ContextMenuOption>({
  show: false,
  keepAlive: false,
  x: 0,
  y: 0,
  currentPath: '',
})

function handleTagClick(path: string) {
  tabStore.setActiveTab(path)
  router.push(path)
}

function showContextMenu() {
  contextMenuOption.show = true
}
function hideContextMenu() {
  contextMenuOption.show = false
}
function setContextMenu(x: number, y: number, keepAlive: boolean, currentPath: string) {
  Object.assign(contextMenuOption, { x, y, keepAlive, currentPath })
}

async function handleContextMenu(e: MouseEvent, tabItem: TabItem) {
  const { clientX, clientY } = e
  hideContextMenu()
  setContextMenu(clientX, clientY, tabItem.keepAlive, tabItem.path)
  await nextTick()
  showContextMenu()
}
</script>

<template>
  <div>
    <n-tabs
      :value="tabStore.active"
      :closable="tabStore.tabs.length > 1"
      type="card"
      @close="(path: string) => tabStore.removeTab(path)"
    >
      <n-tab
        v-for="item in tabStore.tabs"
        :key="item.path"
        :name="item.path"
        @click="handleTagClick(item.path)"
        @contextmenu.prevent="handleContextMenu($event, item)"
      >
        {{ translateTitle(String(item.title)) }}
      </n-tab>
    </n-tabs>
    <ContextMenu
      v-model:show="contextMenuOption.show"
      :current-path="contextMenuOption.currentPath"
      :keep-alive="contextMenuOption.keepAlive"
      :x="contextMenuOption.x"
      :y="contextMenuOption.y"
    />
  </div>
</template>

<style scoped lang="scss">
:deep(.n-tabs) {
  .n-tabs-tab {
    padding-left: 16px;
    height: 36px;
    background: transparent !important;
    border-radius: 4px !important;
    margin-right: 4px;

    &:hover {
      border: 1px solid v-bind('themeVars.primaryColor') !important;
    }
  }

  .n-tabs-tab--active {
    border: 1px solid v-bind('themeVars.primaryColor') !important;
  }

  .n-tabs-pad,
  .n-tabs-tab-pad,
  .n-tabs-scroll-padding {
    border: none !important;
  }
}
</style>

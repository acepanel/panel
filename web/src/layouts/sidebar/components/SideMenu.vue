<script lang="ts" setup>
import type { MenuInst, MenuOption } from 'naive-ui'
import { useThemeVars } from 'naive-ui'
import type { VNodeChild } from 'vue'
import { RouterLink } from 'vue-router'

import { translateTitle } from '@/locales/menu'
import { usePermissionStore, useTabStore, useThemeStore } from '@/stores'
import type { Meta, RouteType } from '@/types/router'
import { isUrl, renderIcon } from '@/utils'

const themeVars = useThemeVars()
const router = useRouter()
const currentRoute = useRoute()
const permissionStore = usePermissionStore()
const themeStore = useThemeStore()
const tabStore = useTabStore()

const menu = ref<MenuInst>()
watch(currentRoute, async () => {
  await nextTick()
  menu.value?.showOption()
})

const menuOptions = computed(() => {
  return permissionStore.menus.map((item) => getMenuItem(item))
})

function resolvePath(basePath: string, path: string) {
  if (isUrl(path)) return path
  return `/${[basePath, path]
    .filter((path) => !!path && path !== '/')
    .map((path) => path.replace(/(^\/)|(\/$)/g, ''))
    .join('/')}`
}

type MenuItem = MenuOption & {
  label: () => VNodeChild
  key: string
  path: string
  children?: Array<MenuItem>
}

function getMenuItem(route: RouteType, basePath = ''): MenuItem {
  let menuItem: MenuItem = {
    label: () =>
      h(
        RouterLink,
        {
          to: { name: route.name as string },
        },
        {
          default: () => (route.meta?.title ? translateTitle(route.meta.title) : route.name),
        },
      ),
    key: route.name,
    path: resolvePath(basePath, route.path),
    icon: getIcon(route.meta),
  }

  const visibleChildren = route.children
    ? route.children.filter(
        (item: RouteType) =>
          item.name && !item.isHidden && !permissionStore.hiddenRoutes.includes(item.name),
      )
    : []

  if (!visibleChildren.length) return menuItem

  if (visibleChildren.length === 1 && visibleChildren[0]) {
    // 单个子路由处理
    const singleRoute = visibleChildren[0]
    menuItem = {
      label: () =>
        h(
          RouterLink,
          {
            to: { name: singleRoute.name as string },
          },
          {
            default: () =>
              singleRoute.meta?.title ? translateTitle(singleRoute.meta.title) : singleRoute?.name,
          },
        ),
      key: singleRoute.name,
      path: resolvePath(menuItem.path, singleRoute.path),
      icon: getIcon(singleRoute.meta),
    }
    const visibleItems = singleRoute.children
      ? singleRoute.children.filter((item: RouteType) => item.name && !item.isHidden)
      : []

    if (visibleItems.length === 1) menuItem = getMenuItem(visibleItems[0]!, menuItem.path)
    else if (visibleItems.length > 1)
      menuItem.children = visibleItems.map((item) => getMenuItem(item, menuItem.path))
  } else {
    menuItem.children = visibleChildren.map((item) => getMenuItem(item, menuItem.path))
  }

  return menuItem
}

function getIcon(meta?: Meta): (() => VNodeChild) | undefined {
  if (meta?.icon) return renderIcon(meta.icon, { size: 14, class: `${meta.icon} text-base` })
  return undefined
}

function handleMenuSelect(key: string, item: MenuOption) {
  const menuItem = item as MenuItem & MenuOption
  if (isUrl(menuItem.path)) {
    window.open(menuItem.path)
    return
  }
  if (menuItem.path === currentRoute.path) {
    // 检查 tab 的 keepAlive 状态，而不是路由元数据
    // 这样用户固定的标签页不会被意外刷新
    const tab = tabStore.tabs.find((t) => t.path === menuItem.path)
    if (!tab?.keepAlive) {
      tabStore.reloadTab(currentRoute.path)
    }
  } else {
    router.push(menuItem.path)
  }

  // 手机端自动收起菜单
  if (themeStore.isMobile) themeStore.setCollapsed(true)
}
</script>

<template>
  <n-menu
    ref="menu"
    :collapsed-icon-size="22"
    :collapsed-width="64"
    :indent="18"
    :options="menuOptions"
    :value="currentRoute.name as string"
    accordion
    class="side-menu"
    @update:value="handleMenuSelect"
  />
</template>

<style lang="scss">
.side-menu {
  .n-menu-item-content__icon {
    border: 1px solid var(--color-border-default);
    border-radius: var(--radius-sm);
  }

  .n-menu-item-content--child-active,
  .n-menu-item-content--selected {
    .n-menu-item-content__icon {
      border-color: v-bind('themeVars.primaryColor');
      background-color: v-bind('themeVars.primaryColor');

      i {
        color: #fff;
      }
    }
  }
}
</style>

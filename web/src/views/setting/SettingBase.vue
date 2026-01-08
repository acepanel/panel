<script setup lang="ts">
import type { TreeSelectOption } from 'naive-ui'

import { translateTitle } from '@/locales/menu'
import { usePermissionStore } from '@/store'
import { locales as availableLocales } from '@/utils'
import { useGettext } from 'vue3-gettext'
import type { RouteType } from '~/types/router'

const { $gettext } = useGettext()
const permissionStore = usePermissionStore()

const model = defineModel<any>('model', { type: Object, required: true })

const locales = computed(() => {
  return Object.entries(availableLocales).map(([code, name]: [string, string]) => {
    return {
      label: name,
      value: code
    }
  })
})

const channels = [
  {
    label: $gettext('Stable'),
    value: 'stable'
  },
  {
    label: $gettext('Beta'),
    value: 'beta'
  }
]

// 获取菜单选项
const getOption = (route: RouteType): TreeSelectOption => {
  let menuItem: TreeSelectOption = {
    label: route.meta?.title ? translateTitle(route.meta.title) : route.name,
    key: route.name
  }

  const visibleChildren = route.children
    ? route.children.filter((item: RouteType) => item.name && !item.isHidden)
    : []

  if (!visibleChildren.length) return menuItem

  if (visibleChildren.length === 1) {
    // 单个子路由处理
    const singleRoute = visibleChildren[0]
    menuItem.label = singleRoute.meta?.title
      ? translateTitle(singleRoute.meta.title)
      : singleRoute.name
    const visibleItems = singleRoute.children
      ? singleRoute.children.filter((item: RouteType) => item.name && !item.isHidden)
      : []

    if (visibleItems.length === 1) menuItem = getOption(visibleItems[0])
    else if (visibleItems.length > 1)
      menuItem.children = visibleItems.map((item) => getOption(item))
  } else {
    menuItem.children = visibleChildren.map((item) => getOption(item))
  }

  return menuItem
}

const menus = computed<TreeSelectOption[]>(() => {
  return permissionStore.allMenus.map((item) => getOption(item))
})
</script>

<template>
  <n-flex vertical>
    <n-form>
      <n-form-item :label="$gettext('Panel Name')">
        <n-input v-model:value="model.name" :placeholder="$gettext('Panel Name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Language')">
        <n-select v-model:value="model.locale" :options="locales"> </n-select>
      </n-form-item>
      <n-form-item :label="$gettext('Update Channel')">
        <n-select v-model:value="model.channel" :options="channels"> </n-select>
      </n-form-item>
      <n-form-item :label="$gettext('Port')">
        <n-input-number v-model:value="model.port" :placeholder="$gettext('8888')" w-full />
      </n-form-item>
      <n-form-item :label="$gettext('Default Website Directory')">
        <n-input v-model:value="model.website_path" :placeholder="$gettext('/opt/ace/sites')" />
      </n-form-item>
      <n-form-item :label="$gettext('Default Backup Directory')">
        <n-input v-model:value="model.backup_path" :placeholder="$gettext('/opt/ace/backup')" />
      </n-form-item>
      <n-form-item :label="$gettext('Custom Logo')">
        <n-input
          v-model:value="model.custom_logo"
          :placeholder="$gettext('Please enter the complete URL')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Hide Menu')">
        <n-tree-select
          cascade
          checkable
          clearable
          multiple
          :options="menus"
          v-model:value="model.hidden_menu"
        />
      </n-form-item>
    </n-form>
  </n-flex>
</template>

<style scoped lang="scss"></style>

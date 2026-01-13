<script setup lang="ts">
import type { TreeSelectOption } from 'naive-ui'

import PathSelector from '@/components/common/PathSelector.vue'
import { translateTitle } from '@/locales/menu'
import { usePermissionStore } from '@/store'
import { locales as availableLocales } from '@/utils'
import { useGettext } from 'vue3-gettext'
import type { RouteType } from '~/types/router'

const { $gettext } = useGettext()
const permissionStore = usePermissionStore()

const model = defineModel<any>('model', { type: Object, required: true })

// 目录选择器
const showPathSelector = ref(false)
const pathSelectorPath = ref('/opt/ace')
const pathSelectorTarget = ref<'website' | 'backup'>('website')

const handleSelectPath = (target: 'website' | 'backup') => {
  pathSelectorTarget.value = target
  pathSelectorPath.value =
    target === 'website'
      ? model.value.website_path || '/opt/ace/sites'
      : model.value.backup_path || '/opt/ace/backup'
  showPathSelector.value = true
}

watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value) {
    if (pathSelectorTarget.value === 'website') {
      model.value.website_path = pathSelectorPath.value
    } else {
      model.value.backup_path = pathSelectorPath.value
    }
  }
})

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

// 不允许隐藏的菜单项（首页 home/home-index 和设置页 setting/setting-index）
const forbiddenHiddenMenus = ['home', 'home-index', 'setting', 'setting-index']

// 获取菜单选项
const getOption = (route: RouteType): TreeSelectOption => {
  const isDisabled = forbiddenHiddenMenus.includes(route.name as string)
  let menuItem: TreeSelectOption = {
    label: route.meta?.title ? translateTitle(route.meta.title) : route.name,
    key: route.name,
    disabled: isDisabled
  }

  const visibleChildren = route.children
    ? route.children.filter((item: RouteType) => item.name && !item.isHidden)
    : []

  if (!visibleChildren.length) return menuItem

  if (visibleChildren.length === 1) {
    // 单个子路由处理
    const singleRoute = visibleChildren[0]
    const isSingleDisabled = forbiddenHiddenMenus.includes(singleRoute.name as string)
    menuItem.label = singleRoute.meta?.title
      ? translateTitle(singleRoute.meta.title)
      : singleRoute.name
    // 父路由或子路由任一被禁止则禁用该菜单项
    menuItem.disabled = isDisabled || isSingleDisabled
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
        <n-input-group>
          <n-input v-model:value="model.website_path" :placeholder="$gettext('/opt/ace/sites')" />
          <n-button @click="handleSelectPath('website')">
            <template #icon>
              <i-mdi-folder-open />
            </template>
          </n-button>
        </n-input-group>
      </n-form-item>
      <n-form-item :label="$gettext('Default Backup Directory')">
        <n-input-group>
          <n-input v-model:value="model.backup_path" :placeholder="$gettext('/opt/ace/backup')" />
          <n-button @click="handleSelectPath('backup')">
            <template #icon>
              <i-mdi-folder-open />
            </template>
          </n-button>
        </n-input-group>
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

  <!-- 目录选择器 -->
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="true" />
</template>

<style scoped lang="scss"></style>

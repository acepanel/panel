<script setup lang="ts">
import type { TreeSelectOption } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import PathSelector from '@/components/common/PathSelector.vue'
import { translateTitle } from '@/locales/menu'
import { usePermissionStore } from '@/stores'
import type { RouteType } from '@/types/router'
import { locales as availableLocales } from '@/utils'

const { $gettext } = useGettext()
const permissionStore = usePermissionStore()

const model = defineModel<any>('model', { type: Object, required: true })

// 目录选择器
const showPathSelector = ref(false)
const pathSelectorPath = ref('/opt/ace')
const pathSelectorTarget = ref<'website' | 'backup' | 'project' | 'ipdb'>('website')

const handleSelectPath = (target: 'website' | 'backup' | 'project' | 'ipdb') => {
  pathSelectorTarget.value = target
  if (target === 'website') {
    pathSelectorPath.value = model.value.website_path || '/opt/ace/sites'
  } else if (target === 'backup') {
    pathSelectorPath.value = model.value.backup_path || '/opt/ace/backup'
  } else if (target === 'ipdb') {
    pathSelectorPath.value = model.value.ipdb_path || '/opt/ace'
  } else {
    pathSelectorPath.value = model.value.project_path || '/opt/ace/projects'
  }
  showPathSelector.value = true
}

watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value) {
    if (pathSelectorTarget.value === 'website') {
      model.value.website_path = pathSelectorPath.value
    } else if (pathSelectorTarget.value === 'backup') {
      model.value.backup_path = pathSelectorPath.value
    } else if (pathSelectorTarget.value === 'ipdb') {
      model.value.ipdb_path = pathSelectorPath.value
    } else {
      model.value.project_path = pathSelectorPath.value
    }
  }
})

const locales = computed(() => {
  return Object.entries(availableLocales).map(([code, name]: [string, string]) => {
    return {
      label: name,
      value: code,
    }
  })
})

const channels = [
  {
    label: $gettext('Stable'),
    value: 'stable',
  },
  {
    label: $gettext('Beta'),
    value: 'beta',
  },
]

const backupFormats = [
  { label: 'tar.xz', value: 'tar.xz' },
  { label: 'tar.gz', value: 'tar.gz' },
  { label: 'zip', value: 'zip' },
  { label: '7z', value: '7z' },
]

// 不允许隐藏的菜单项（首页 home/home-index 和设置页 setting/setting-index）
const forbiddenHiddenMenus = ['home', 'home-index', 'setting', 'setting-index']

// 获取菜单选项
const getOption = (route: RouteType): TreeSelectOption => {
  const isDisabled = forbiddenHiddenMenus.includes(route.name as string)
  let menuItem: TreeSelectOption = {
    label: route.meta?.title ? translateTitle(route.meta.title) : route.name,
    key: route.name,
    disabled: isDisabled,
  }

  const visibleChildren = route.children
    ? route.children.filter((item: RouteType) => item.name && !item.isHidden)
    : []

  if (!visibleChildren.length) return menuItem

  if (visibleChildren.length === 1) {
    // 单个子路由处理
    const singleRoute = visibleChildren[0]
    if (!singleRoute) return menuItem
    const isSingleDisabled = forbiddenHiddenMenus.includes(singleRoute.name as string)
    menuItem.label = singleRoute.meta?.title
      ? translateTitle(singleRoute.meta.title)
      : singleRoute.name
    // 父路由或子路由任一被禁止则禁用该菜单项
    menuItem.disabled = isDisabled || isSingleDisabled
    const visibleItems = singleRoute.children
      ? singleRoute.children.filter((item: RouteType) => item.name && !item.isHidden)
      : []

    const firstVisibleItem = visibleItems[0]
    if (visibleItems.length === 1 && firstVisibleItem) menuItem = getOption(firstVisibleItem)
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
      <n-form-item :label="$gettext('Backup Compression Format')">
        <n-select v-model:value="model.backup_format" :options="backupFormats" />
      </n-form-item>
      <n-form-item :label="$gettext('Default Project Directory')">
        <n-input-group>
          <n-input
            v-model:value="model.project_path"
            :placeholder="$gettext('/opt/ace/projects')"
          />
          <n-button @click="handleSelectPath('project')">
            <template #icon>
              <i-mdi-folder-open />
            </template>
          </n-button>
        </n-input-group>
      </n-form-item>
      <n-form-item :label="$gettext('Container Socket')">
        <n-input v-model:value="model.container_sock" :placeholder="'/var/run/docker.sock'" />
      </n-form-item>
      <n-form-item :label="$gettext('Custom Logo')">
        <n-input
          v-model:value="model.custom_logo"
          :placeholder="$gettext('Please enter the complete URL')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('IP Database')">
        <n-radio-group v-model:value="model.ipdb_type">
          <n-radio-button value="">{{ $gettext('Disabled') }}</n-radio-button>
          <n-radio-button value="subscribe">{{ $gettext('Subscribe') }}</n-radio-button>
          <n-radio-button value="custom">{{ $gettext('Custom File') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-form-item v-if="model.ipdb_type === 'subscribe'" :label="$gettext('Subscribe URL')">
        <n-input
          v-model:value="model.ipdb_url"
          placeholder="https://fastly.jsdelivr.net/npm/qqwry.ipdb/qqwry.ipdb"
        />
        <template #feedback>
          {{ $gettext('Auto-update weekly, compatible with IPIP.NET format (.ipdb)') }}
        </template>
      </n-form-item>
      <n-form-item v-if="model.ipdb_type === 'custom'" :label="$gettext('IPDB Path')">
        <n-input-group>
          <n-input
            v-model:value="model.ipdb_path"
            :placeholder="$gettext('GeoIP database file path (.ipdb)')"
          />
          <n-button @click="handleSelectPath('ipdb')">
            <template #icon>
              <i-mdi-folder-open />
            </template>
          </n-button>
        </n-input-group>
        <template #feedback>
          {{ $gettext('Compatible with IPIP.NET format (.ipdb)') }}
        </template>
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

  <!-- 文件/目录选择器 -->
  <path-selector
    v-model:show="showPathSelector"
    v-model:path="pathSelectorPath"
    :dir="pathSelectorTarget !== 'ipdb'"
  />
</template>

<style scoped lang="scss"></style>

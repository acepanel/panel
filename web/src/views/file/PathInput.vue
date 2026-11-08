<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import type { InputInst } from 'naive-ui'
import { useGettext } from 'vue3-gettext'
import draggable from 'vuedraggable'

import { useResponsive } from '@/components/system/composables/useResponsive'
import { useFileStore } from '@/stores'
import { checkPath } from '@/utils/file'
import { useFileOps } from '@/views/file/composables/useFileOps'
import type { FileFavorite } from '@/views/file/types'

const { $gettext } = useGettext()
const fileStore = useFileStore()
const { toggleFavorite } = useFileOps()
const { isMobile } = useResponsive()

const props = defineProps<{
  tabId: string
}>()

const tab = computed(() => fileStore.tabs.find((t) => t.id === props.tabId)!)
const path = computed(() => tab.value.path)

const isInput = ref(false)
const pathInput = ref<InputInst | null>(null)
const input = ref('')

const handleInput = () => {
  isInput.value = true
  nextTick(() => {
    pathInput.value?.focus()
  })
}

// 双击地址栏复制路径
const handlePathDoubleClick = async () => {
  try {
    await copy2clipboard(path.value)
    window.$message.success($gettext('Path copied to clipboard'))
  } catch (error) {
    window.$message.error($gettext('Failed to copy path'))
  }
}

// 输入框呈现完整绝对路径，方便直接全选复制；失焦时容错补前导 / 并去掉尾部 /
const handleBlur = () => {
  const trimmed = input.value.trim().replace(/\/+$/, '')
  const full = trimmed.startsWith('/') ? trimmed : '/' + trimmed
  if (!checkPath(full.slice(1))) {
    window.$message.error($gettext('Invalid path'))
    return
  }

  isInput.value = false
  fileStore.updateTabPath(props.tabId, full)
}

const handleRefresh = () => {
  window.$bus.emit('file:refresh')
}

// 切换显示隐藏文件
const toggleHidden = () => {
  fileStore.toggleShowHidden()
}

const handleUp = () => {
  const count = splitPath(path.value, '/').length
  setPath(count - 2)
}

const handleBack = () => {
  fileStore.historyBack(props.tabId)
}

const handleForward = () => {
  fileStore.historyForward(props.tabId)
}

const splitPath = (str: string, delimiter: string) => {
  if (str === delimiter || str === '') {
    return []
  }
  return str.split(delimiter).slice(1)
}

const setPath = (index: number) => {
  const newPath = splitPath(path.value, '/')
    .slice(0, index + 1)
    .join('/')
  fileStore.updateTabPath(props.tabId, '/' + newPath)
}

const handleSearch = () => {
  window.$bus.emit('file:search')
}

// 收藏目录
const showFavorites = ref(false)
const isFavorite = computed(() => fileStore.isFavorite(path.value))

const favoriteName = (item: FileFavorite) => item.remark || item.path.split('/').pop() || '/'

const handleGoFavorite = (p: string) => {
  showFavorites.value = false
  fileStore.updateTabPath(props.tabId, p)
}

// 行内编辑备注，回车或失焦保存，Esc 取消
const editingId = ref<number | null>(null)
const editingRemark = ref('')

const startEditRemark = (item: FileFavorite) => {
  editingId.value = item.id
  editingRemark.value = item.remark
}

const saveRemark = () => {
  if (editingId.value === null) return
  const id = editingId.value
  editingId.value = null
  fileStore.updateFavoriteRemark(id, editingRemark.value.trim())
}

watch(
  path,
  (value) => {
    input.value = value
  },
  { immediate: true },
)
</script>

<template>
  <n-flex>
    <n-button-group>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleBack">
            <i-mdi-arrow-left class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Back') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleForward">
            <i-mdi-arrow-right class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Forward') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleUp">
            <i-mdi-arrow-up class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Up') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="handleRefresh">
            <i-mdi-refresh class="text-base" />
          </n-button>
        </template>
        {{ $gettext('Refresh') }}
      </n-tooltip>
      <n-tooltip>
        <template #trigger>
          <n-button @click="toggleHidden" :type="fileStore.showHidden ? 'primary' : 'default'">
            <i-mdi-eye v-if="fileStore.showHidden" class="text-base" />
            <i-mdi-eye-off v-else class="text-base" />
          </n-button>
        </template>
        {{ fileStore.showHidden ? $gettext('Hide hidden files') : $gettext('Show hidden files') }}
      </n-tooltip>
      <n-popover
        v-model:show="showFavorites"
        trigger="click"
        placement="bottom-start"
        :show-arrow="false"
        content-class="!p-0"
        @update:show="editingId = null"
      >
        <template #trigger>
          <n-button :type="isFavorite ? 'primary' : 'default'">
            <i-mdi-star v-if="isFavorite" class="text-base" />
            <i-mdi-star-outline v-else class="text-base" />
          </n-button>
        </template>
        <div class="favorites" :class="{ 'is-mobile': isMobile }">
          <div class="favorites-header">
            <span>{{ $gettext('Favorites') }}</span>
            <n-button size="tiny" quaternary type="primary" @click="toggleFavorite(path)">
              {{
                isFavorite
                  ? $gettext('Remove current directory')
                  : $gettext('Add current directory')
              }}
            </n-button>
          </div>
          <div v-if="!fileStore.favorites.length" class="favorites-empty">
            {{ $gettext('No favorites yet') }}
          </div>
          <draggable
            v-model="fileStore.favorites"
            item-key="id"
            handle=".favorites-drag"
            class="favorites-list"
            @end="fileStore.sortFavorites()"
          >
            <template #item="{ element: item }">
              <div
                class="favorites-item"
                :class="{ active: item.path === path }"
                @click="handleGoFavorite(item.path)"
              >
                <i-mdi-drag class="favorites-drag" />
                <i-mdi-folder class="text-amber-500 shrink-0" />
                <div class="favorites-text">
                  <n-input
                    v-if="editingId === item.id"
                    v-model:value="editingRemark"
                    size="tiny"
                    autofocus
                    @click.stop
                    :placeholder="$gettext('Remark')"
                    @keyup.enter="saveRemark"
                    @keyup.esc="editingId = null"
                    @blur="saveRemark"
                  />
                  <div v-else class="favorites-name">{{ favoriteName(item) }}</div>
                  <div class="favorites-path">{{ item.path }}</div>
                </div>
                <div class="favorites-actions" @click.stop>
                  <n-button size="tiny" quaternary circle @click="startEditRemark(item)">
                    <i-mdi-pencil />
                  </n-button>
                  <n-button size="tiny" quaternary circle @click="toggleFavorite(item.path)">
                    <i-mdi-close />
                  </n-button>
                </div>
              </div>
            </template>
          </draggable>
        </div>
      </n-popover>
    </n-button-group>
    <n-input-group flex-1>
      <n-tag
        size="large"
        v-if="!isInput"
        flex-1
        @click="handleInput"
        @dblclick="handlePathDoubleClick"
      >
        <n-breadcrumb separator=">">
          <n-breadcrumb-item @click.stop="setPath(-1)">
            {{ $gettext('Root Directory') }}
          </n-breadcrumb-item>
          <n-breadcrumb-item
            v-for="(item, index) in splitPath(path, '/')"
            :key="index"
            @click.stop="setPath(index)"
          >
            {{ item }}
          </n-breadcrumb-item>
        </n-breadcrumb>
      </n-tag>
      <n-input
        ref="pathInput"
        v-model:value="input"
        v-if="isInput"
        @keyup.enter="handleBlur"
        @blur="handleBlur"
      />
    </n-input-group>
    <n-input-group class="md:!w-100">
      <n-input v-model:value="tab.keyword" :placeholder="$gettext('Enter search content')">
        <template #suffix>
          <n-checkbox v-model:checked="tab.sub">
            {{ $gettext('Include subdirectories') }}
          </n-checkbox>
        </template>
      </n-input>
      <n-button type="primary" @click="handleSearch">
        <i-mdi-search class="text-base" />
      </n-button>
    </n-input-group>
  </n-flex>
</template>

<style scoped lang="scss">
.favorites {
  width: 320px;
  max-width: 90vw;
  font-size: 13px;
}

.favorites-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 8px 8px 12px;
  font-weight: 500;
  border-bottom: 1px solid var(--color-border);
}

.favorites-empty {
  padding: 16px;
  text-align: center;
  color: var(--color-text-tertiary);
}

.favorites-list {
  max-height: 320px;
  overflow-y: auto;
  padding: 4px;
}

.favorites-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;

  &:hover {
    background: var(--color-bg-subtle);

    .favorites-actions {
      opacity: 1;
    }
  }

  &.active {
    background: var(--color-brand-subtle);
  }
}

.favorites-drag {
  flex-shrink: 0;
  cursor: grab;
  color: var(--color-text-tertiary);
}

.favorites-text {
  flex: 1;
  min-width: 0;
}

.favorites-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.favorites-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.favorites-actions {
  display: flex;
  flex-shrink: 0;
  opacity: 0;
}

// 触屏没有悬停，操作按钮常显
.favorites.is-mobile .favorites-actions {
  opacity: 1;
}
</style>

<script setup lang="ts">
import type { TreeOption } from 'naive-ui'
import { NInput, useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import TheIcon from '@/components/custom/TheIcon.vue'
import { useEditorOps } from '@/components/file-editor/composables/useEditorOps'
import { useEditorStore } from '@/stores'
import { getExt, getIconByExt } from '@/utils/file'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
const { openInEditor } = useEditorOps()
const themeVars = useThemeVars()

const props = defineProps<{
  rootPath: string
}>()

const emit = defineEmits<{
  (e: 'update:rootPath', path: string): void
}>()

// 文件树数据
const treeData = ref<TreeOption[]>([])
const expandedKeys = ref<string[]>([])
const selectedKeys = ref<string[]>([])
const loading = ref(false)
const searchKeyword = ref('')
const searchLoading = ref(false)
const searchResults = ref<TreeOption[]>([])
const isSearchMode = computed(() => searchKeyword.value.trim().length > 0)

// 内联新建状态
const inlineCreateType = ref<'file' | 'dir' | null>(null)
const inlineCreateName = ref('')
const inlineCreateParentPath = ref('')
const inlineCreateLoading = ref(false)

// 内联新建节点的特殊 key
const INLINE_CREATE_KEY = '__inline_create__'

// 创建内联新建节点
function createInlineNode(): TreeOption {
  return {
    key: INLINE_CREATE_KEY,
    label: '',
    isLeaf: true,
    prefix: () =>
      h(TheIcon, {
        icon: inlineCreateType.value === 'dir' ? 'mdi:folder' : 'mdi:file-outline',
        size: 18,
        color: inlineCreateType.value === 'dir' ? '#f59e0b' : '#6b7280',
      }),
  }
}

// 目录在前，同类型按名称排序；搜索结果一律作叶子，点击文件夹是跳转而不是展开
function toNodes(items: any[], flat = false): TreeOption[] {
  return [...items]
    .sort((a, b) => {
      if (a.dir && !b.dir) return -1
      if (!a.dir && b.dir) return 1
      return a.name.localeCompare(b.name)
    })
    .map((item) => ({
      key: item.full,
      label: item.name,
      isLeaf: flat || !item.dir,
      isDir: item.dir,
      prefix: () =>
        h(TheIcon, {
          icon: item.dir ? 'mdi:folder' : getIconByExt(getExt(item.name)),
          size: 18,
          color: item.dir ? '#f59e0b' : '#6b7280',
        }),
    }))
}

// 加载目录内容
async function loadDirectory(path: string): Promise<TreeOption[]> {
  const data = await file.list(path, '', false, 'name', 1, 1000)
  return toNodes(data.items || [])
}

// 序号保护：根目录快速切换时，先发后到的旧结果不覆盖新结果
let treeSeq = 0

// 初始化加载根目录
// 保留 expandedKeys，n-tree 会对已展开但未加载的节点自动触发 on-load，刷新后目录不会折叠
async function initTree() {
  const seq = ++treeSeq
  loading.value = true
  const nodes = await loadDirectory(props.rootPath).catch(() => [])
  if (seq !== treeSeq) return
  treeData.value = nodes
  loading.value = false
}

// 刷新父目录，并通知文件列表同步
async function reloadParent(parentPath: string) {
  if (parentPath === props.rootPath) {
    await initTree()
  } else {
    const parentNode = findNode(treeData.value, parentPath)
    if (parentNode) {
      parentNode.children = await loadDirectory(parentPath)
    }
  }
  window.$bus.emit('file:refresh')
}

// 懒加载子目录
async function handleLoad(node: TreeOption): Promise<void> {
  if (node.isLeaf) return
  try {
    node.children = await loadDirectory(node.key as string)
  } catch {
    node.children = []
  }
}

// 展开节点
function handleExpandedKeysUpdate(keys: string[]) {
  expandedKeys.value = keys
}

// 选择节点（打开文件）
function handleSelect(keys: string[], option: (TreeOption | null)[]) {
  if (keys.length === 0) return
  selectedKeys.value = keys

  const node = option[0]
  if (!node) return

  const isDir = (node as any)?.isDir

  // 搜索模式下点击文件夹，跳转到该目录
  if (isSearchMode.value && isDir) {
    const path = node.key as string
    searchKeyword.value = ''
    emit('update:rootPath', path)
    return
  }

  if (node && node.isLeaf && !isDir) {
    openInEditor(node.key as string)
  }
}

// 上一级目录
function handleGoUp() {
  const parts = props.rootPath.split('/').filter(Boolean)
  if (parts.length > 0) {
    parts.pop()
    const newPath = '/' + parts.join('/')
    emit('update:rootPath', newPath || '/')
  }
}

// 刷新
function handleRefresh() {
  initTree()
}

// 显示内联新建
function showCreate(type: 'file' | 'dir') {
  // 如果已经在新建中，先取消
  if (inlineCreateType.value) {
    cancelInlineCreate()
  }

  inlineCreateType.value = type
  inlineCreateName.value = ''

  // 确定父目录
  if (selectedKeys.value.length > 0) {
    const selectedKey = selectedKeys.value[0] ?? ''
    const selectedNode = findNode(treeData.value, selectedKey)
    if (selectedNode && !selectedNode.isLeaf) {
      // 选中的是目录，在该目录下新建
      inlineCreateParentPath.value = selectedKey
      // 确保目录已展开
      if (!expandedKeys.value.includes(selectedKey)) {
        expandedKeys.value = [...expandedKeys.value, selectedKey]
      }
    } else {
      // 选中的是文件，在其父目录下新建
      const parts = selectedKey.split('/')
      parts.pop()
      inlineCreateParentPath.value = parts.join('/') || props.rootPath
    }
  } else {
    // 没有选中，在根目录下新建
    inlineCreateParentPath.value = props.rootPath
  }

  // 插入内联新建节点
  insertInlineNode()
}

// 查找节点
function findNode(nodes: TreeOption[], key: string): TreeOption | null {
  for (const node of nodes) {
    if (node.key === key) return node
    if (node.children) {
      const found = findNode(node.children, key)
      if (found) return found
    }
  }
  return null
}

// 插入内联新建节点
function insertInlineNode() {
  const inlineNode = createInlineNode()

  if (inlineCreateParentPath.value === props.rootPath) {
    // 在根目录下新建，插入到 treeData 开头
    removeInlineNode()
    treeData.value = [inlineNode, ...treeData.value]
  } else {
    // 在子目录下新建
    const parentNode = findNode(treeData.value, inlineCreateParentPath.value)
    if (parentNode) {
      removeInlineNode()
      if (!parentNode.children) {
        parentNode.children = []
      }
      parentNode.children = [inlineNode, ...parentNode.children]
    }
  }
}

// 移除内联新建节点
function removeInlineNode() {
  // 从根目录移除
  treeData.value = treeData.value.filter((n) => n.key !== INLINE_CREATE_KEY)

  // 从所有子目录移除
  function removeFromChildren(nodes: TreeOption[]) {
    for (const node of nodes) {
      if (node.children) {
        node.children = node.children.filter((n) => n.key !== INLINE_CREATE_KEY)
        removeFromChildren(node.children)
      }
    }
  }
  removeFromChildren(treeData.value)
}

// 取消内联新建
function cancelInlineCreate() {
  removeInlineNode()
  inlineCreateType.value = null
  inlineCreateName.value = ''
  inlineCreateParentPath.value = ''
}

// 确认内联新建
async function confirmInlineCreate() {
  if (!inlineCreateName.value.trim()) {
    cancelInlineCreate()
    return
  }

  const fullPath = `${inlineCreateParentPath.value}/${inlineCreateName.value}`.replace(/\/+/g, '/')
  const isDir = inlineCreateType.value === 'dir'

  inlineCreateLoading.value = true
  useRequest(file.create(fullPath, isDir))
    .onSuccess(async () => {
      window.$message.success($gettext('Created successfully'))

      // 保存当前新建的类型和路径
      const parentPath = inlineCreateParentPath.value
      const createdPath = fullPath

      // 取消内联状态
      cancelInlineCreate()

      await reloadParent(parentPath)

      // 如果是文件，自动打开
      if (!isDir) {
        editorStore.openFile(createdPath, '')
      }
    })
    .onComplete(() => {
      inlineCreateLoading.value = false
    })
}

// 搜索文件，序号保护同 initTree
let searchSeq = 0

async function doSearch(keyword: string) {
  const seq = ++searchSeq
  if (!keyword.trim()) {
    searchResults.value = []
    searchLoading.value = false
    return
  }

  searchLoading.value = true
  const data = await file.list(props.rootPath, keyword, true, 'name', 1, 100).catch(() => null)
  if (seq !== searchSeq) return
  searchResults.value = toNodes(data?.items || [], true)
  searchLoading.value = false
}

watchDebounced(searchKeyword, doSearch, { debounce: 300 })

// 自定义渲染 label
function renderLabel({ option }: { option: TreeOption }) {
  // 内联新建节点
  if (option.key === INLINE_CREATE_KEY) {
    return h(NInput, {
      value: inlineCreateName.value,
      'onUpdate:value': (v: string) => {
        inlineCreateName.value = v
      },
      size: 'tiny',
      placeholder:
        inlineCreateType.value === 'dir' ? $gettext('Folder name') : $gettext('File name'),
      autofocus: true,
      disabled: inlineCreateLoading.value,
      style: { width: '120px' },
      onKeyup: (e: KeyboardEvent) => {
        e.stopPropagation()
        if (e.key === 'Enter') {
          confirmInlineCreate()
        } else if (e.key === 'Escape') {
          cancelInlineCreate()
        }
      },
      onBlur: () => {
        setTimeout(() => {
          if (inlineCreateType.value && !inlineCreateLoading.value) {
            if (inlineCreateName.value.trim()) {
              confirmInlineCreate()
            } else {
              cancelInlineCreate()
            }
          }
        }, 150)
      },
      onClick: (e: MouseEvent) => {
        e.stopPropagation()
      },
    })
  }

  // 内联重命名节点
  if (option.key === inlineRenameKey.value) {
    return h(NInput, {
      value: inlineRenameName.value,
      'onUpdate:value': (v: string) => {
        inlineRenameName.value = v
      },
      size: 'tiny',
      autofocus: true,
      disabled: inlineRenameLoading.value,
      style: { width: '120px' },
      onKeyup: (e: KeyboardEvent) => {
        e.stopPropagation()
        if (e.key === 'Enter') {
          confirmInlineRename()
        } else if (e.key === 'Escape') {
          cancelInlineRename()
        }
      },
      onBlur: () => {
        setTimeout(() => {
          if (inlineRenameKey.value && !inlineRenameLoading.value) {
            if (inlineRenameName.value.trim()) {
              confirmInlineRename()
            } else {
              cancelInlineRename()
            }
          }
        }, 150)
      },
      onClick: (e: MouseEvent) => {
        e.stopPropagation()
      },
    })
  }

  return option.label as string
}

// 搜索结果渲染 label（显示完整路径）
function renderSearchLabel({ option }: { option: TreeOption }) {
  const fullPath = option.key as string
  // 移除 rootPath 前缀，显示相对路径
  const relativePath = fullPath.startsWith(props.rootPath)
    ? fullPath.slice(props.rootPath.length).replace(/^\//, '')
    : fullPath
  return relativePath || option.label
}

// 路径编辑
const isEditingPath = ref(false)
const editingPath = ref('')
const pathInputRef = ref<HTMLInputElement | null>(null)

// 右键菜单
const showContextMenu = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuNode = ref<TreeOption | null>(null)

// 内联重命名状态
const inlineRenameKey = ref<string | null>(null)
const inlineRenameName = ref('')
const inlineRenameLoading = ref(false)

// 右键菜单选项
const contextMenuOptions = computed(() => {
  if (!contextMenuNode.value) return []
  const isDir = !contextMenuNode.value.isLeaf
  const options = [
    { label: $gettext('Rename'), key: 'rename' },
    { label: $gettext('Delete'), key: 'delete', props: { style: { color: 'red' } } },
  ]
  if (!isDir) {
    options.unshift({ label: $gettext('Download'), key: 'download' })
  }
  return options
})

// 处理右键菜单
function handleContextMenu(e: MouseEvent, option: TreeOption) {
  // 忽略内联新建节点
  if (option.key === INLINE_CREATE_KEY) return

  e.preventDefault()
  e.stopPropagation()
  showContextMenu.value = false
  nextTick(() => {
    contextMenuNode.value = option
    contextMenuX.value = e.clientX
    contextMenuY.value = e.clientY
    showContextMenu.value = true
  })
}

// 关闭右键菜单
function handleContextMenuClose() {
  showContextMenu.value = false
  contextMenuNode.value = null
}

// 处理右键菜单选择
function handleContextMenuSelect(key: string) {
  if (!contextMenuNode.value) return

  const nodePath = contextMenuNode.value.key as string
  const nodeName = contextMenuNode.value.label as string

  switch (key) {
    case 'rename':
      startInlineRename(nodePath, nodeName)
      break
    case 'delete':
      handleDelete(nodePath, nodeName)
      break
    case 'download':
      handleDownload(nodePath)
      break
  }

  handleContextMenuClose()
}

// 开始内联重命名
function startInlineRename(path: string, name: string) {
  inlineRenameKey.value = path
  inlineRenameName.value = name
}

// 取消内联重命名
function cancelInlineRename() {
  inlineRenameKey.value = null
  inlineRenameName.value = ''
}

// 确认内联重命名
function confirmInlineRename() {
  if (!inlineRenameKey.value || !inlineRenameName.value.trim()) {
    cancelInlineRename()
    return
  }

  const oldPath = inlineRenameKey.value
  const oldName = oldPath.split('/').pop() || ''
  const newName = inlineRenameName.value.trim()

  if (oldName === newName) {
    cancelInlineRename()
    return
  }

  const parentPath = oldPath.substring(0, oldPath.lastIndexOf('/')) || '/'
  const newPath = `${parentPath}/${newName}`.replace(/\/+/g, '/')

  inlineRenameLoading.value = true
  // 后端对已存在的目标会静默跳过，先检查避免假成功
  useRequest(file.exist([newPath]))
    .onSuccess(({ data: exists }) => {
      if (exists[0]) {
        inlineRenameLoading.value = false
        window.$message.error($gettext('Target name already exists'))
        return
      }

      useRequest(file.move([{ source: oldPath, target: newPath, force: false }]))
        .onSuccess(async () => {
          window.$message.success($gettext('Renamed successfully'))

          // 同步编辑器中已打开的标签页路径(含重命名目录下的子文件)
          editorStore.movePath(oldPath, newPath)

          // 同步树的展开与选中状态，重命名后的目录保持展开
          const migrate = (key: string) => {
            if (key === oldPath) return newPath
            if (key.startsWith(oldPath + '/')) return newPath + key.slice(oldPath.length)
            return key
          }
          expandedKeys.value = expandedKeys.value.map(migrate)
          selectedKeys.value = selectedKeys.value.map(migrate)

          cancelInlineRename()

          await reloadParent(parentPath)
        })
        .onComplete(() => {
          inlineRenameLoading.value = false
        })
    })
    .onError(() => {
      inlineRenameLoading.value = false
    })
}

// 删除文件/目录
function handleDelete(path: string, name: string) {
  window.$dialog.warning({
    title: $gettext('Delete'),
    content: $gettext('Are you sure you want to delete %{ name }?', { name }),
    positiveText: $gettext('Delete'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: async () => {
      useRequest(file.delete(path)).onSuccess(async () => {
        window.$message.success($gettext('Deleted successfully'))

        // 关闭编辑器中已打开的该文件及该目录下所有文件的标签页
        editorStore.closePath(path)

        await reloadParent(path.substring(0, path.lastIndexOf('/')) || '/')
      })
    },
  })
}

// 下载文件
function handleDownload(path: string) {
  window.open('/api/file/download?path=' + encodeURIComponent(path))
}

// 节点属性（用于绑定右键菜单）
function nodeProps({ option }: { option: TreeOption }) {
  return {
    onContextmenu: (e: MouseEvent) => handleContextMenu(e, option),
  }
}

function startEditPath() {
  editingPath.value = props.rootPath
  isEditingPath.value = true
  nextTick(() => {
    pathInputRef.value?.focus()
    pathInputRef.value?.select()
  })
}

function confirmEditPath() {
  const newPath = editingPath.value.trim()
  if (newPath && newPath !== props.rootPath) {
    // 确保路径以 / 开头
    const normalizedPath = newPath.startsWith('/') ? newPath : '/' + newPath
    emit('update:rootPath', normalizedPath)
  }
  isEditingPath.value = false
}

function cancelEditPath() {
  isEditingPath.value = false
  editingPath.value = ''
}

// 监听根目录变化
watch(
  () => props.rootPath,
  () => {
    initTree()
  },
)

// 树的选中项跟随编辑器当前标签页，新建文件、切换标签等方式打开的文件都会高亮
watch(
  () => editorStore.activeTabPath,
  (path) => {
    if (path) {
      selectedKeys.value = [path]
    }
  },
  { immediate: true },
)

onMounted(() => {
  initTree()
})

defineExpose({
  refresh: handleRefresh,
})
</script>

<template>
  <div class="file-tree">
    <!-- 工具栏 -->
    <div class="file-tree-toolbar">
      <n-button-group size="small">
        <n-button quaternary @click="handleGoUp" :title="$gettext('Go Up')">
          <template #icon>
            <i-mdi-arrow-up />
          </template>
        </n-button>
        <n-button quaternary @click="handleRefresh" :title="$gettext('Refresh')">
          <template #icon>
            <i-mdi-refresh />
          </template>
        </n-button>
        <n-popselect
          :options="[
            { label: $gettext('New File'), value: 'file' },
            { label: $gettext('New Folder'), value: 'dir' },
          ]"
          @update:value="showCreate"
        >
          <n-button quaternary :title="$gettext('New')">
            <template #icon>
              <i-mdi-plus />
            </template>
          </n-button>
        </n-popselect>
      </n-button-group>
      <n-input
        v-model:value="searchKeyword"
        size="small"
        :placeholder="$gettext('Search')"
        clearable
        class="flex-1 min-w-0"
      >
        <template #prefix>
          <i-mdi-magnify />
        </template>
      </n-input>
    </div>

    <!-- 当前目录 -->
    <div class="current-path" @click="startEditPath" v-if="!isEditingPath">
      <the-icon icon="mdi:folder-open" :size="14" class="path-icon" />
      <n-ellipsis :tooltip="{ width: 300 }">
        {{ rootPath }}
      </n-ellipsis>
      <the-icon icon="mdi:pencil" :size="12" class="edit-icon" />
    </div>
    <div class="current-path editing" v-else>
      <n-input
        ref="pathInputRef"
        v-model:value="editingPath"
        size="tiny"
        :placeholder="$gettext('Enter path')"
        @keyup.enter="confirmEditPath"
        @keydown.escape="cancelEditPath"
        @blur="confirmEditPath"
      >
        <template #prefix>
          <the-icon icon="mdi:folder-open" :size="14" />
        </template>
      </n-input>
    </div>

    <!-- 文件树 -->
    <div class="tree-container">
      <!-- 搜索模式：显示搜索结果 -->
      <n-spin v-if="isSearchMode" :show="searchLoading" class="tree-spin">
        <n-tree
          v-if="searchResults.length > 0"
          block-line
          :data="searchResults"
          :selected-keys="selectedKeys"
          :render-label="renderSearchLabel"
          :node-props="nodeProps"
          @update:selected-keys="handleSelect"
          selectable
          virtual-scroll
          class="file-tree-content"
          style="height: 100%"
        />
        <n-empty
          v-else-if="!searchLoading"
          :description="$gettext('No results found')"
          class="px-5 py-10"
        />
      </n-spin>
      <!-- 普通模式：显示文件树 -->
      <n-spin v-else :show="loading" class="tree-spin">
        <n-tree
          v-if="treeData.length > 0"
          block-line
          :data="treeData"
          :expanded-keys="expandedKeys"
          :selected-keys="selectedKeys"
          :on-load="handleLoad"
          :render-label="renderLabel"
          :node-props="nodeProps"
          @update:expanded-keys="handleExpandedKeysUpdate"
          @update:selected-keys="handleSelect"
          selectable
          expand-on-click
          virtual-scroll
          class="file-tree-content"
          style="height: 100%"
        />
        <n-empty
          v-else-if="!loading"
          :description="$gettext('No data')"
          class="flex flex-col h-full items-center justify-center"
        />
      </n-spin>
    </div>

    <!-- 右键菜单 -->
    <n-dropdown
      placement="bottom-start"
      trigger="manual"
      :x="contextMenuX"
      :y="contextMenuY"
      :options="contextMenuOptions"
      :show="showContextMenu"
      @select="handleContextMenuSelect"
      @clickoutside="handleContextMenuClose"
    />
  </div>
</template>

<style scoped lang="scss">
.file-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.file-tree-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border-bottom: 1px solid v-bind('themeVars.borderColor');
  flex-shrink: 0;
}

.current-path {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  min-height: 32px;
  box-sizing: border-box;
  font-size: 12px;
  color: v-bind('themeVars.textColor3');
  border-bottom: 1px solid v-bind('themeVars.borderColor');
  flex-shrink: 0;
  cursor: pointer;
  transition: background-color 0.2s;

  &:hover:not(.editing) {
    background-color: v-bind('themeVars.buttonColor2Hover');

    .edit-icon {
      opacity: 1;
    }
  }

  &.editing {
    cursor: default;
    padding: 4px 8px;
  }

  .path-icon {
    flex-shrink: 0;
    color: #f59e0b;
  }

  .edit-icon {
    flex-shrink: 0;
    margin-left: auto;
    opacity: 0;
    transition: opacity 0.2s;
  }

  :deep(.n-input) {
    flex: 1;
  }
}

.tree-container {
  flex: 1;
  overflow: hidden;
  min-height: 0;
}

.tree-spin {
  height: 100%;

  :deep(.n-spin-content) {
    height: 100%;
  }
}

.file-tree-content {
  height: 100%;
  overflow: auto;
}
</style>

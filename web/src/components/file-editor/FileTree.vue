<script setup lang="ts">
import file from '@/api/panel/file'
import TheIcon from '@/components/custom/TheIcon.vue'
import { useEditorStore } from '@/store'
import { decodeBase64 } from '@/utils'
import { getExt, getIconByExt } from '@/utils/file'
import type { TreeOption } from 'naive-ui'
import { NInput, useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
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
        color: inlineCreateType.value === 'dir' ? '#f59e0b' : '#6b7280'
      })
  }
}

// 加载目录内容
async function loadDirectory(path: string): Promise<TreeOption[]> {
  return new Promise((resolve, reject) => {
    useRequest(file.list(path, '', false, 'name', 1, 1000))
      .onSuccess(({ data }) => {
        const items = data.items || []
        // 排序：目录在前，文件在后，同类型按名称排序
        const sortedItems = [...items].sort((a: any, b: any) => {
          if (a.dir && !b.dir) return -1
          if (!a.dir && b.dir) return 1
          return a.name.localeCompare(b.name)
        })
        resolve(
          sortedItems.map((item: any) => ({
            key: item.full,
            label: item.name,
            isLeaf: !item.dir,
            prefix: () =>
              h(TheIcon, {
                icon: item.dir ? 'mdi:folder' : getIconByExt(getExt(item.name)),
                size: 18,
                color: item.dir ? '#f59e0b' : '#6b7280'
              }),
            isDir: item.dir,
            children: item.dir ? undefined : undefined
          }))
        )
      })
      .onError(() => {
        reject(new Error('Failed to load directory'))
      })
  })
}

// 初始化加载根目录
async function initTree() {
  loading.value = true
  try {
    treeData.value = await loadDirectory(props.rootPath)
    expandedKeys.value = []
  } catch {
    treeData.value = []
  } finally {
    loading.value = false
  }
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
    // 打开文件
    const path = node.key as string
    const existingTab = editorStore.tabs.find((t) => t.path === path)
    if (existingTab) {
      editorStore.switchTab(path)
    } else {
      // 加载文件内容
      editorStore.openFile(path, '')
      editorStore.setLoading(path, true)
      useRequest(file.content(encodeURIComponent(path)))
        .onSuccess(({ data }) => {
          const content = decodeBase64(data.content)
          editorStore.reloadFile(path, content)
        })
        .onError(() => {
          editorStore.closeTab(path)
          window.$message.error($gettext('Failed to load file'))
        })
        .onComplete(() => {
          editorStore.setLoading(path, false)
        })
    }
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
    const selectedNode = findNode(treeData.value, selectedKeys.value[0])
    if (selectedNode && !selectedNode.isLeaf) {
      // 选中的是目录，在该目录下新建
      inlineCreateParentPath.value = selectedKeys.value[0]
      // 确保目录已展开
      if (!expandedKeys.value.includes(selectedKeys.value[0])) {
        expandedKeys.value = [...expandedKeys.value, selectedKeys.value[0]]
      }
    } else {
      // 选中的是文件，在其父目录下新建
      const parts = selectedKeys.value[0].split('/')
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

      // 刷新父目录
      if (parentPath === props.rootPath) {
        await initTree()
      } else {
        const parentNode = findNode(treeData.value, parentPath)
        if (parentNode) {
          parentNode.children = await loadDirectory(parentPath)
        }
      }

      // 如果是文件，自动打开
      if (!isDir) {
        editorStore.openFile(createdPath, '')
      }
    })
    .onError(() => {
      window.$message.error($gettext('Failed to create'))
    })
    .onComplete(() => {
      inlineCreateLoading.value = false
    })
}

// 搜索文件
let searchTimer: ReturnType<typeof setTimeout> | null = null

function doSearch(keyword: string) {
  if (!keyword.trim()) {
    searchResults.value = []
    return
  }

  searchLoading.value = true
  useRequest(file.list(props.rootPath, keyword, true, 'name', 1, 100))
    .onSuccess(({ data }) => {
      const items = data.items || []
      // 排序：目录在前，文件在后
      const sortedItems = [...items].sort((a: any, b: any) => {
        if (a.dir && !b.dir) return -1
        if (!a.dir && b.dir) return 1
        return a.name.localeCompare(b.name)
      })
      searchResults.value = sortedItems.map((item: any) => ({
        key: item.full,
        label: item.name,
        fullPath: item.full,
        isLeaf: true, // 搜索结果都设为叶子节点，文件夹点击时跳转而不是展开
        prefix: () =>
          h(TheIcon, {
            icon: item.dir ? 'mdi:folder' : getIconByExt(getExt(item.name)),
            size: 18,
            color: item.dir ? '#f59e0b' : '#6b7280'
          }),
        isDir: item.dir
      }))
    })
    .onError(() => {
      searchResults.value = []
    })
    .onComplete(() => {
      searchLoading.value = false
    })
}

// 监听搜索关键词变化（防抖）
watch(searchKeyword, (keyword) => {
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
  searchTimer = setTimeout(() => {
    doSearch(keyword)
  }, 300)
})

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
      }
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
      }
    })
  }

  return option.label as string
}

// 搜索结果渲染 label（显示完整路径）
function renderSearchLabel({ option }: { option: TreeOption }) {
  const fullPath = (option as any).fullPath as string
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
    { label: $gettext('Delete'), key: 'delete', props: { style: { color: 'red' } } }
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
  const isDir = !contextMenuNode.value.isLeaf

  switch (key) {
    case 'rename':
      startInlineRename(nodePath, nodeName)
      break
    case 'delete':
      handleDelete(nodePath, nodeName, isDir)
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
  useRequest(file.move([{ source: oldPath, target: newPath, force: false }]))
    .onSuccess(async () => {
      window.$message.success($gettext('Renamed successfully'))

      // 如果重命名的文件在编辑器中打开，更新标签页
      const tab = editorStore.tabs.find((t) => t.path === oldPath)
      if (tab) {
        editorStore.closeTab(oldPath)
        editorStore.openFile(newPath, tab.content)
        if (!tab.modified) {
          editorStore.markSaved(newPath)
        }
      }

      cancelInlineRename()

      // 刷新父目录
      if (parentPath === props.rootPath) {
        await initTree()
      } else {
        const parentNode = findNode(treeData.value, parentPath)
        if (parentNode) {
          parentNode.children = await loadDirectory(parentPath)
        }
      }
    })
    .onError(() => {
      window.$message.error($gettext('Failed to rename'))
    })
    .onComplete(() => {
      inlineRenameLoading.value = false
    })
}

// 删除文件/目录
function handleDelete(path: string, name: string, isDir: boolean) {
  window.$dialog.warning({
    title: $gettext('Delete'),
    content: $gettext('Are you sure you want to delete %{ name }?', { name }),
    positiveText: $gettext('Delete'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: async () => {
      useRequest(file.delete(path))
        .onSuccess(async () => {
          window.$message.success($gettext('Deleted successfully'))

          // 如果删除的文件在编辑器中打开，关闭标签页
          if (!isDir) {
            editorStore.closeTab(path)
          }

          // 刷新父目录
          const parentPath = path.substring(0, path.lastIndexOf('/')) || '/'
          if (parentPath === props.rootPath) {
            await initTree()
          } else {
            const parentNode = findNode(treeData.value, parentPath)
            if (parentNode) {
              parentNode.children = await loadDirectory(parentPath)
            }
          }
        })
        .onError(() => {
          window.$message.error($gettext('Failed to delete'))
        })
    }
  })
}

// 下载文件
function handleDownload(path: string) {
  window.open('/api/file/download?path=' + encodeURIComponent(path))
}

// 节点属性（用于绑定右键菜单）
function nodeProps({ option }: { option: TreeOption }) {
  return {
    onContextmenu: (e: MouseEvent) => handleContextMenu(e, option)
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
  }
)

onMounted(() => {
  initTree()
})

defineExpose({
  refresh: handleRefresh
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
            { label: $gettext('New Folder'), value: 'dir' }
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
        @keyup.escape="cancelEditPath"
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
          virtual-scrollå
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

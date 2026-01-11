<script setup lang="ts">
import { NEllipsis, NFlex, NInput, NPopconfirm, NSpin, useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import type { DropdownOption } from 'naive-ui'

import file from '@/api/panel/file'
import TheIcon from '@/components/custom/TheIcon.vue'
import PtyTerminalModal from '@/components/common/PtyTerminalModal.vue'
import {
  checkName,
  checkPath,
  getExt,
  getFilename,
  getIconByExt,
  isCompress,
  isImage
} from '@/utils/file'
import EditModal from '@/views/file/EditModal.vue'
import PreviewModal from '@/views/file/PreviewModal.vue'
import PropertyModal from '@/views/file/PropertyModal.vue'
import type { FileInfo, Marked } from '@/views/file/types'
import { useFileStore } from '@/store'

const { $gettext } = useGettext()
const themeVars = useThemeVars()
const fileStore = useFileStore()
const sort = ref<string>('')
const path = defineModel<string>('path', { type: String, required: true })
const keyword = defineModel<string>('keyword', { type: String, default: '' })
const sub = defineModel<boolean>('sub', { type: Boolean, default: false })
const selected = defineModel<any[]>('selected', { type: Array, default: () => [] })
const marked = defineModel<Marked[]>('marked', { type: Array, default: () => [] })
const markedType = defineModel<string>('markedType', { type: String, required: true })
const compress = defineModel<boolean>('compress', { type: Boolean, required: true })
const permission = defineModel<boolean>('permission', { type: Boolean, required: true })
const permissionFileInfoList = defineModel<FileInfo[]>('permissionFileInfoList', {
  type: Array,
  default: () => []
})

const editorModal = ref(false)
const previewModal = ref(false)
const currentFile = ref('')
const propertyModal = ref(false)
const propertyFileInfo = ref<FileInfo | null>(null)
const terminalModal = ref(false)
const terminalPath = ref('')

const showDropdown = ref(false)
const selectedRow = ref<any>()
const dropdownX = ref(0)
const dropdownY = ref(0)

const renameModal = ref(false)
const renameModel = ref({
  source: '',
  target: ''
})
const unCompressModal = ref(false)
const unCompressModel = ref({
  path: '',
  file: ''
})

// 框选相关状态
const gridContainerRef = ref<HTMLElement | null>(null)
const isSelecting = ref(false)
const selectionStart = ref({ x: 0, y: 0 })
const selectionEnd = ref({ x: 0, y: 0 })
const selectionBox = computed(() => {
  if (!isSelecting.value) return null
  const left = Math.min(selectionStart.value.x, selectionEnd.value.x)
  const top = Math.min(selectionStart.value.y, selectionEnd.value.y)
  const width = Math.abs(selectionEnd.value.x - selectionStart.value.x)
  const height = Math.abs(selectionEnd.value.y - selectionStart.value.y)
  return { left, top, width, height }
})

// 检查是否有 immutable 属性
const confirmImmutableOperation = (row: any, operation: string, callback: () => void) => {
  if (row.immutable) {
    window.$dialog.warning({
      title: $gettext('Warning'),
      content: $gettext(
        '%{ name } has immutable attribute. The panel will temporarily remove the immutable attribute, perform the operation, and then restore the immutable attribute. Do you want to continue?',
        { name: row.name }
      ),
      positiveText: $gettext('Continue'),
      negativeText: $gettext('Cancel'),
      onPositiveClick: callback
    })
  } else {
    callback()
  }
}

const options = computed<DropdownOption[]>(() => {
  if (selectedRow.value == null) return []
  const options = [
    {
      label: selectedRow.value.dir
        ? $gettext('Open')
        : isImage(selectedRow.value.name)
          ? $gettext('Preview')
          : isCompress(selectedRow.value.name)
            ? $gettext('Uncompress')
            : $gettext('Edit'),
      key: selectedRow.value.dir
        ? 'open'
        : isImage(selectedRow.value.name)
          ? 'preview'
          : isCompress(selectedRow.value.name)
            ? 'uncompress'
            : 'edit'
    },
    { label: $gettext('Copy'), key: 'copy' },
    { label: $gettext('Move'), key: 'move' },
    { label: $gettext('Permission'), key: 'permission' },
    {
      label: selectedRow.value.dir ? $gettext('Compress') : $gettext('Download'),
      key: selectedRow.value.dir ? 'compress' : 'download'
    },
    {
      label: $gettext('Uncompress'),
      key: 'uncompress',
      show: isCompress(selectedRow.value.full),
      disabled: !isCompress(selectedRow.value.full)
    },
    { label: $gettext('Rename'), key: 'rename' },
    {
      label: $gettext('Terminal'),
      key: 'terminal',
      show: selectedRow.value.dir
    },
    { label: $gettext('Properties'), key: 'properties' },
    { label: () => h('span', { style: { color: 'red' } }, $gettext('Delete')), key: 'delete' }
  ]
  if (marked.value.length) {
    options.unshift({
      label: $gettext('Paste'),
      key: 'paste'
    })
  }
  return options
})

const openPermissionModal = (row: any) => {
  selected.value = [row.full]
  permissionFileInfoList.value = [row as FileInfo]
  permission.value = true
}

const openFile = (row: any) => {
  if (row.dir) {
    path.value = row.full
    return
  }

  if (isImage(row.name)) {
    currentFile.value = row.full
    previewModal.value = true
  } else if (isCompress(row.name)) {
    unCompressModel.value.file = row.full
    unCompressModel.value.path = path.value
    unCompressModal.value = true
  } else {
    currentFile.value = row.full
    editorModal.value = true
  }
}

// 获取文件图标
const getFileIcon = (item: any) => {
  if (item.dir) {
    return 'mdi:folder'
  }
  return getIconByExt(getExt(item.name))
}

// 获取图标颜色
const getIconColor = (item: any) => {
  if (item.dir) {
    return '#f0a020'
  }
  return themeVars.value.primaryColor
}

// 检查项目是否被选中
const isSelected = (item: any) => {
  return selected.value.includes(item.full)
}

// 切换选择
const toggleSelect = (item: any, event: MouseEvent) => {
  event.stopPropagation()
  if (event.ctrlKey || event.metaKey) {
    // Ctrl/Cmd + 点击：多选
    const index = selected.value.indexOf(item.full)
    if (index > -1) {
      selected.value.splice(index, 1)
    } else {
      selected.value.push(item.full)
    }
  } else if (event.shiftKey && selected.value.length > 0) {
    // Shift + 点击：范围选择
    const lastSelected = selected.value[selected.value.length - 1]
    const lastIndex = data.value.findIndex((i: any) => i.full === lastSelected)
    const currentIndex = data.value.findIndex((i: any) => i.full === item.full)
    const start = Math.min(lastIndex, currentIndex)
    const end = Math.max(lastIndex, currentIndex)
    const newSelected = data.value.slice(start, end + 1).map((i: any) => i.full)
    selected.value = [...new Set([...selected.value, ...newSelected])]
  } else {
    // 普通点击：单选
    selected.value = [item.full]
  }
}

// 处理项目点击
const handleItemClick = (item: any, event: MouseEvent) => {
  if (item.dir) {
    // 目录单击进入
    path.value = item.full
  } else {
    // 文件单击选择
    toggleSelect(item, event)
  }
}

// 处理项目双击
const handleItemDblClick = (item: any) => {
  openFile(item)
}

// 处理右键菜单
const handleContextMenu = (item: any, event: MouseEvent) => {
  event.preventDefault()
  showDropdown.value = false
  nextTick().then(() => {
    showDropdown.value = true
    selectedRow.value = item
    dropdownX.value = event.clientX
    dropdownY.value = event.clientY
  })
}

// 框选开始
const onSelectionStart = (event: MouseEvent) => {
  // 只响应左键，且不在项目上
  if (event.button !== 0) return
  const target = event.target as HTMLElement
  if (target.closest('.grid-item')) return

  isSelecting.value = true
  const container = gridContainerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  selectionStart.value = {
    x: event.clientX - rect.left + container.scrollLeft,
    y: event.clientY - rect.top + container.scrollTop
  }
  selectionEnd.value = { ...selectionStart.value }

  // 如果没有按住 Ctrl/Cmd，清除已选
  if (!event.ctrlKey && !event.metaKey) {
    selected.value = []
  }
}

// 框选移动
const onSelectionMove = (event: MouseEvent) => {
  if (!isSelecting.value) return

  const container = gridContainerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  selectionEnd.value = {
    x: event.clientX - rect.left + container.scrollLeft,
    y: event.clientY - rect.top + container.scrollTop
  }

  // 更新选中的项目
  updateSelectionFromBox()
}

// 框选结束
const onSelectionEnd = () => {
  isSelecting.value = false
}

// 根据选择框更新选中的项目
const updateSelectionFromBox = () => {
  if (!selectionBox.value || !gridContainerRef.value) return

  const container = gridContainerRef.value
  const items = container.querySelectorAll('.grid-item')
  const newSelected: string[] = []

  items.forEach((item) => {
    const rect = item.getBoundingClientRect()
    const containerRect = container.getBoundingClientRect()

    const itemBox = {
      left: rect.left - containerRect.left + container.scrollLeft,
      top: rect.top - containerRect.top + container.scrollTop,
      right: rect.right - containerRect.left + container.scrollLeft,
      bottom: rect.bottom - containerRect.top + container.scrollTop
    }

    const selectBox = {
      left: selectionBox.value!.left,
      top: selectionBox.value!.top,
      right: selectionBox.value!.left + selectionBox.value!.width,
      bottom: selectionBox.value!.top + selectionBox.value!.height
    }

    // 检查是否相交
    if (
      !(
        itemBox.right < selectBox.left ||
        itemBox.left > selectBox.right ||
        itemBox.bottom < selectBox.top ||
        itemBox.top > selectBox.bottom
      )
    ) {
      const fullPath = item.getAttribute('data-path')
      if (fullPath) {
        newSelected.push(fullPath)
      }
    }
  })

  selected.value = newSelected
}

// 处理粘贴
const handlePaste = () => {
  if (!marked.value.length) {
    window.$message.error($gettext('Please mark the files/folders to copy or move first'))
    return
  }

  let flag = false
  const paths = marked.value.map((item) => ({
    name: item.name,
    source: item.source,
    target: path.value + '/' + item.name,
    force: false
  }))
  const sources = paths.map((item: any) => item.target)
  useRequest(file.exist(sources)).onSuccess(({ data }) => {
    for (let i = 0; i < data.length; i++) {
      if (data[i]) {
        flag = true
        paths[i].force = true
      }
    }
    if (flag) {
      window.$dialog.warning({
        title: $gettext('Warning'),
        content: $gettext(
          'There are items with the same name %{ items } Do you want to overwrite?',
          {
            items: `${paths
              .filter((item) => item.force)
              .map((item) => item.name)
              .join(', ')}`
          }
        ),
        positiveText: $gettext('Overwrite'),
        negativeText: $gettext('Cancel'),
        onPositiveClick: () => {
          if (markedType.value == 'copy') {
            useRequest(file.copy(paths)).onSuccess(() => {
              marked.value = []
              window.$bus.emit('file:refresh')
              window.$message.success($gettext('Copied successfully'))
            })
          } else {
            useRequest(file.move(paths)).onSuccess(() => {
              marked.value = []
              window.$bus.emit('file:refresh')
              window.$message.success($gettext('Moved successfully'))
            })
          }
        },
        onNegativeClick: () => {
          marked.value = []
          window.$message.info($gettext('Canceled'))
        }
      })
    } else {
      if (markedType.value == 'copy') {
        useRequest(file.copy(paths)).onSuccess(() => {
          marked.value = []
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Copied successfully'))
        })
      } else {
        useRequest(file.move(paths)).onSuccess(() => {
          marked.value = []
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Moved successfully'))
        })
      }
    }
  })
}

const handleSelect = (key: string) => {
  switch (key) {
    case 'paste':
      handlePaste()
      break
    case 'open':
    case 'edit':
    case 'preview':
    case 'uncompress':
      openFile(selectedRow.value)
      break
    case 'copy':
      markedType.value = 'copy'
      marked.value = [
        {
          name: selectedRow.value.name,
          source: selectedRow.value.full,
          force: false
        }
      ]
      window.$message.success(
        $gettext('Marked successfully, please navigate to the destination path to paste')
      )
      break
    case 'move':
      markedType.value = 'move'
      marked.value = [
        {
          name: selectedRow.value.name,
          source: selectedRow.value.full,
          force: false
        }
      ]
      window.$message.success(
        $gettext('Marked successfully, please navigate to the destination path to paste')
      )
      break
    case 'permission':
      selected.value = [selectedRow.value.full]
      permissionFileInfoList.value = [selectedRow.value as FileInfo]
      permission.value = true
      break
    case 'compress':
      selected.value = [selectedRow.value.full]
      compress.value = true
      break
    case 'download':
      window.open('/api/file/download?path=' + encodeURIComponent(selectedRow.value.full))
      break
    case 'rename':
      confirmImmutableOperation(selectedRow.value, 'rename', () => {
        renameModel.value.source = getFilename(selectedRow.value.name)
        renameModel.value.target = getFilename(selectedRow.value.name)
        renameModal.value = true
      })
      break
    case 'terminal':
      terminalPath.value = selectedRow.value.full
      terminalModal.value = true
      break
    case 'properties':
      propertyFileInfo.value = selectedRow.value as FileInfo
      propertyModal.value = true
      break
    case 'delete':
      confirmImmutableOperation(selectedRow.value, 'delete', () => {
        useRequest(file.delete(selectedRow.value.full)).onSuccess(() => {
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Deleted successfully'))
        })
      })
      break
  }
  onCloseDropdown()
}

const onCloseDropdown = () => {
  selectedRow.value = null
  showDropdown.value = false
}

const handleRename = () => {
  const source = path.value + '/' + renameModel.value.source
  const target = path.value + '/' + renameModel.value.target
  if (!checkName(renameModel.value.source) || !checkName(renameModel.value.target)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  useRequest(file.exist([target])).onSuccess(({ data }) => {
    if (data[0]) {
      window.$dialog.warning({
        title: $gettext('Warning'),
        content: $gettext('There are items with the same name. Do you want to overwrite?'),
        positiveText: $gettext('Overwrite'),
        negativeText: $gettext('Cancel'),
        onPositiveClick: () => {
          useRequest(file.move([{ source, target, force: true }]))
            .onSuccess(() => {
              window.$bus.emit('file:refresh')
              window.$message.success(
                $gettext('Renamed %{ source } to %{ target } successfully', {
                  source: renameModel.value.source,
                  target: renameModel.value.target
                })
              )
            })
            .onComplete(() => {
              renameModal.value = false
            })
        }
      })
    } else {
      useRequest(file.move([{ source, target, force: false }]))
        .onSuccess(() => {
          window.$bus.emit('file:refresh')
          window.$message.success(
            $gettext('Renamed %{ source } to %{ target } successfully', {
              source: renameModel.value.source,
              target: renameModel.value.target
            })
          )
        })
        .onComplete(() => {
          renameModal.value = false
        })
    }
  })
}

const handleUnCompress = () => {
  if (
    !unCompressModel.value.path.startsWith('/') ||
    !checkPath(unCompressModel.value.path.slice(1))
  ) {
    window.$message.error($gettext('Invalid path'))
    return
  }
  const message = window.$message.loading($gettext('Uncompressing...'), {
    duration: 0
  })
  useRequest(file.unCompress(unCompressModel.value.file, unCompressModel.value.path))
    .onSuccess(() => {
      unCompressModal.value = false
      window.$bus.emit('file:refresh')
      window.$message.success($gettext('Uncompressed successfully'))
    })
    .onComplete(() => {
      message?.destroy()
    })
}

const {
  loading,
  data: rawData,
  page,
  total,
  pageSize,
  pageCount,
  refresh
} = usePagination(
  (page, pageSize) =>
    file.list(
      encodeURIComponent(path.value),
      keyword.value,
      sub.value,
      sort.value,
      page,
      pageSize
    ),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 100,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const data = computed(() => {
  if (fileStore.showHidden) {
    return rawData.value
  }
  return rawData.value.filter((item: any) => !item.hidden)
})

onMounted(() => {
  watch(
    path,
    () => {
      selected.value = []
      keyword.value = ''
      sub.value = false
      nextTick(() => {
        refresh()
      })
      window.$bus.emit('file:push-history', path.value)
    },
    { immediate: true }
  )
  window.$bus.on('file:search', () => {
    selected.value = []
    nextTick(() => {
      refresh()
    })
    window.$bus.emit('file:push-history', path.value)
  })
  window.$bus.on('file:refresh', refresh)

  // 添加全局鼠标事件监听
  document.addEventListener('mousemove', onSelectionMove)
  document.addEventListener('mouseup', onSelectionEnd)
})

onUnmounted(() => {
  window.$bus.off('file:refresh')
  document.removeEventListener('mousemove', onSelectionMove)
  document.removeEventListener('mouseup', onSelectionEnd)
})
</script>

<template>
  <div class="grid-view-wrapper">
    <n-spin :show="loading">
      <div
        ref="gridContainerRef"
        class="grid-container"
        @mousedown="onSelectionStart"
      >
        <!-- 框选框 -->
        <div
          v-if="selectionBox"
          class="selection-box"
          :style="{
            left: selectionBox.left + 'px',
            top: selectionBox.top + 'px',
            width: selectionBox.width + 'px',
            height: selectionBox.height + 'px'
          }"
        />

        <!-- 文件/文件夹网格 -->
        <div
          v-for="item in data"
          :key="item.full"
          class="grid-item"
          :class="{ selected: isSelected(item) }"
          :data-path="item.full"
          @click="handleItemClick(item, $event)"
          @dblclick="handleItemDblClick(item)"
          @contextmenu="handleContextMenu(item, $event)"
        >
          <div class="icon-wrapper">
            <the-icon :icon="getFileIcon(item)" :size="48" :style="{ color: getIconColor(item) }" />
            <!-- 锁定图标 -->
            <the-icon
              v-if="item.immutable"
              icon="mdi:lock"
              :size="16"
              class="lock-icon"
            />
          </div>
          <n-ellipsis :line-clamp="2" class="item-name" :tooltip="{ width: 300 }">
            {{ item.name }}
          </n-ellipsis>
        </div>

        <!-- 空状态 -->
        <div v-if="data.length === 0 && !loading" class="empty-state">
          <the-icon icon="mdi:folder-open-outline" :size="64" style="opacity: 0.3" />
          <p>{{ $gettext('No files') }}</p>
        </div>
      </div>
    </n-spin>

    <!-- 分页 -->
    <n-flex justify="center" class="pagination-wrapper">
      <n-pagination
        v-model:page="page"
        v-model:page-size="pageSize"
        :page-count="pageCount"
        :item-count="total"
        show-quick-jumper
        show-size-picker
        :page-sizes="[100, 200, 500, 1000]"
      />
    </n-flex>
  </div>

  <!-- 右键菜单 -->
  <n-dropdown
    placement="bottom-start"
    trigger="manual"
    :x="dropdownX"
    :y="dropdownY"
    :options="options"
    :show="showDropdown"
    :on-clickoutside="onCloseDropdown"
    @select="handleSelect"
  />

  <!-- 编辑弹窗 -->
  <edit-modal v-model:show="editorModal" v-model:file="currentFile" />
  <!-- 预览弹窗 -->
  <preview-modal v-model:show="previewModal" v-model:path="currentFile" />
  <!-- 重命名弹窗 -->
  <n-modal
    v-model:show="renameModal"
    preset="card"
    :title="$gettext('Rename - %{ source }', { source: renameModel.source })"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('New Name')">
          <n-input v-model:value="renameModel.target" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="handleRename">{{ $gettext('Save') }}</n-button>
    </n-flex>
  </n-modal>
  <!-- 解压弹窗 -->
  <n-modal
    v-model:show="unCompressModal"
    preset="card"
    :title="$gettext('Uncompress - %{ file }', { file: unCompressModel.file })"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('Uncompress to')">
          <n-input v-model:value="unCompressModel.path" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="handleUnCompress">{{ $gettext('Uncompress') }}</n-button>
    </n-flex>
  </n-modal>
  <!-- 属性弹窗 -->
  <property-modal v-model:show="propertyModal" v-model:file-info="propertyFileInfo" />
  <!-- 终端弹窗 -->
  <pty-terminal-modal
    v-model:show="terminalModal"
    :title="$gettext('Terminal - %{ path }', { path: terminalPath })"
    :command="`cd '${terminalPath}' && exec bash`"
  />
</template>

<style scoped lang="scss">
.grid-view-wrapper {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.grid-container {
  position: relative;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
  gap: 16px;
  padding: 16px;
  min-height: 400px;
  max-height: 60vh;
  overflow: auto;
  background: var(--n-color);
  border-radius: 8px;
  user-select: none;
}

.selection-box {
  position: absolute;
  border: 2px dashed var(--n-primary-color);
  background: rgba(var(--n-primary-color-rgb), 0.1);
  pointer-events: none;
  z-index: 100;
}

.grid-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;

  &:hover {
    background: var(--n-color-hover);
  }

  &.selected {
    background: var(--n-color-pressed);
    box-shadow: 0 0 0 2px var(--n-primary-color);
  }
}

.icon-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin-bottom: 8px;
}

.lock-icon {
  position: absolute;
  bottom: 0;
  right: 0;
  color: #f0a020;
}

.item-name {
  text-align: center;
  font-size: 12px;
  line-height: 1.4;
  word-break: break-all;
  max-width: 100%;
}

.empty-state {
  grid-column: 1 / -1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  color: var(--n-text-color-3);
}

.pagination-wrapper {
  padding: 8px 0;
}
</style>

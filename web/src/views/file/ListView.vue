<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import type { DropdownOption, SelectOption } from 'naive-ui'
import {
  NButton,
  NCheckbox,
  NEllipsis,
  NFlex,
  NPopselect,
  NSpin,
  NTag,
  useThemeVars,
} from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import PtyTerminalModal from '@/components/common/PtyTerminalModal.vue'
import TheIcon from '@/components/custom/TheIcon.vue'
import { useFileStore } from '@/stores'
import {
  checkName,
  checkPath,
  getExt,
  getFilename,
  getIconByExt,
  isCompress,
  isImage,
} from '@/utils/file'
import { usePaste } from '@/views/file/composables/usePaste'
import EditModal from '@/views/file/EditModal.vue'
import PreviewModal from '@/views/file/PreviewModal.vue'
import PropertyModal from '@/views/file/PropertyModal.vue'
import type { FileInfo } from '@/views/file/types'

const { $gettext } = useGettext()
const themeVars = useThemeVars()
const fileStore = useFileStore()
const { handlePaste: doPaste } = usePaste()

const props = defineProps<{
  tabId: string
}>()

// 排序状态
const sort = computed(() => fileStore.sort)

const tab = computed(() => fileStore.tabs.find((t) => t.id === props.tabId)!)
const path = computed({
  get: () => tab.value.path,
  set: (v: string) => fileStore.updateTabPath(props.tabId, v),
})
const keyword = computed(() => tab.value.keyword)
const sub = computed(() => tab.value.sub)
const marked = computed(() => fileStore.clipboard.marked)
const markedType = computed(() => fileStore.clipboard.markedType)

const selected = defineModel<any[]>('selected', { type: Array, default: () => [] })
const compress = defineModel<boolean>('compress', { type: Boolean, required: true })
const permission = defineModel<boolean>('permission', { type: Boolean, required: true })
const permissionFileInfoList = defineModel<FileInfo[]>('permissionFileInfoList', {
  type: Array,
  default: () => [],
})

const editorModal = ref(false)
const editorMinimized = ref(false)
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

// 内联重命名状态
const inlineRenameItem = ref<any>(null)
const inlineRenameName = ref('')
const inlineRenameInputRef = ref<HTMLInputElement | null>(null)

// 内联新建状态
const inlineCreateActive = ref(false)
const inlineCreateIsDir = ref(false)
const inlineCreateName = ref('')
const inlineCreateInputRef = ref<HTMLInputElement | null>(null)

// 设置内联重命名输入框 ref 的函数
const setInlineRenameRef = (el: any) => {
  if (el) {
    inlineRenameInputRef.value = el
  }
}

// 设置内联新建输入框 ref 的函数
const setInlineCreateRef = (el: any) => {
  if (el) {
    inlineCreateInputRef.value = el
  }
}

// 启动内联新建
const startInlineCreate = (isDir: boolean) => {
  // 取消正在进行的重命名
  cancelInlineRename()

  inlineCreateActive.value = true
  inlineCreateIsDir.value = isDir
  inlineCreateName.value = ''

  // 等待 DOM 更新后聚焦输入框
  nextTick(() => {
    if (inlineCreateInputRef.value) {
      inlineCreateInputRef.value.focus()
    }
  })
}

// 取消内联新建
const cancelInlineCreate = () => {
  inlineCreateActive.value = false
  inlineCreateIsDir.value = false
  inlineCreateName.value = ''
}

// 提交内联新建
const submitInlineCreate = () => {
  if (!inlineCreateActive.value) return

  const name = inlineCreateName.value.trim()

  // 如果名称为空，直接取消
  if (!name) {
    cancelInlineCreate()
    return
  }

  // 验证名称
  if (!checkName(name)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  const fullPath = path.value + '/' + name

  useRequest(file.create(fullPath, inlineCreateIsDir.value))
    .onSuccess(() => {
      window.$bus.emit('file:refresh')
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      cancelInlineCreate()
    })
}

// 处理内联新建键盘事件
const handleInlineCreateKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Enter') {
    event.preventDefault()
    submitInlineCreate()
  } else if (event.key === 'Escape') {
    event.preventDefault()
    cancelInlineCreate()
  }
}

const unCompressModal = ref(false)
const unCompressModel = ref({
  path: '',
  file: '',
})

// 目录大小计算状态（列表视图用）
const sizeLoading = ref<Map<string, boolean>>(new Map())
const sizeCache = ref<Map<string, string>>(new Map())

// 框选相关状态
const gridContainerRef = ref<HTMLElement | null>(null)
const isSelecting = ref(false)
const selectionBoxEl = ref<HTMLElement | null>(null)

// 非响应式坐标 — 避免每次 mousemove 触发 Vue 重渲染
let selStartX = 0
let selStartY = 0
let selEndX = 0
let selEndY = 0

// 缓存 item 位置，框选期间不再每帧 querySelectorAll + getBoundingClientRect
let cachedItemRects: { path: string; left: number; top: number; right: number; bottom: number }[] =
  []
let cachedContainerRect: DOMRect | null = null

// 将 hex 颜色转换为 RGB
const hexToRgb = (hex: string) => {
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex)
  return result
    ? `${parseInt(result[1] ?? '0', 16)}, ${parseInt(result[2] ?? '0', 16)}, ${parseInt(result[3] ?? '0', 16)}`
    : '24, 160, 88'
}

// 主题 CSS 变量
const themeStyles = computed(() => {
  const primaryRgb = hexToRgb(themeVars.value.primaryColor)
  return {
    '--primary-color': themeVars.value.primaryColor,
    '--primary-color-hover': `rgba(${primaryRgb}, 0.12)`,
    '--primary-color-hover-deep': `rgba(${primaryRgb}, 0.16)`,
    '--primary-color-border': `rgba(${primaryRgb}, 0.3)`,
    '--primary-color-border-deep': `rgba(${primaryRgb}, 0.4)`,
    '--selection-border': `rgba(${primaryRgb}, 0.5)`,
    '--selection-bg': `rgba(${primaryRgb}, 0.05)`,
    '--warning-color': themeVars.value.warningColor,
    '--hover-bg': themeVars.value.hoverColor,
    '--hover-border': themeVars.value.borderColor,
    '--card-color': themeVars.value.cardColor,
    '--border-color': themeVars.value.borderColor,
    '--text-color': themeVars.value.textColor1,
    '--text-color-3': themeVars.value.textColor3,
  }
})

// 处理排序
const handleSort = (key: string) => {
  fileStore.setSort(key)
}

// 获取排序图标
const getSortIcon = (key: string) => {
  if (fileStore.sortKey !== key) return 'mdi:unfold-more-horizontal'
  return fileStore.sortOrder === 'asc' ? 'mdi:chevron-up' : 'mdi:chevron-down'
}

// 检查是否有 immutable 属性
const confirmImmutableOperation = (row: any, callback: () => void) => {
  if (row.immutable) {
    window.$dialog.warning({
      title: $gettext('Warning'),
      content: $gettext(
        '%{ name } has immutable attribute. The panel will temporarily remove the immutable attribute, perform the operation, and then restore the immutable attribute. Do you want to continue?',
        { name: row.name },
      ),
      positiveText: $gettext('Continue'),
      negativeText: $gettext('Cancel'),
      onPositiveClick: callback,
    })
  } else {
    callback()
  }
}

// 判断是否多选
const isMultiSelect = computed(() => selected.value.length > 1)

// 全选状态
const isAllSelected = computed(() => {
  return data.value.length > 0 && selected.value.length === data.value.length
})

// 部分选中状态（用于全选复选框的 indeterminate 状态）
const isIndeterminate = computed(() => {
  return selected.value.length > 0 && selected.value.length < data.value.length
})

// 切换全选
const toggleSelectAll = () => {
  if (isAllSelected.value) {
    selected.value = []
  } else {
    selected.value = data.value.map((item: any) => item.full)
  }
}

// 切换单个项目的选中状态（用于复选框点击）
const toggleCheckbox = (item: any) => {
  const index = selected.value.indexOf(item.full)
  if (index > -1) {
    selected.value.splice(index, 1)
  } else {
    selected.value.push(item.full)
  }
}

// 空白区域右键菜单是否激活
const isEmptyContextMenu = ref(false)

const options = computed<DropdownOption[]>(() => {
  // 空白区域右键菜单
  if (isEmptyContextMenu.value) {
    const options: DropdownOption[] = [
      { label: $gettext('New File'), key: 'new-file' },
      { label: $gettext('New Folder'), key: 'new-folder' },
    ]
    if (marked.value.length) {
      options.unshift({
        label: $gettext('Paste'),
        key: 'paste',
      })
    }
    return options
  }

  // 多选情况下显示简化菜单
  if (isMultiSelect.value) {
    const options: DropdownOption[] = [
      { label: $gettext('Copy'), key: 'copy' },
      { label: $gettext('Move'), key: 'move' },
      { label: $gettext('Compress'), key: 'compress' },
      { label: $gettext('Permission'), key: 'permission' },
      { label: () => h('span', { style: { color: 'red' } }, $gettext('Delete')), key: 'delete' },
    ]
    if (marked.value.length) {
      options.unshift({
        label: $gettext('Paste'),
        key: 'paste',
      })
    }
    return options
  }

  // 单选情况下显示完整菜单
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
            : 'edit',
    },
    { label: $gettext('Copy'), key: 'copy' },
    { label: $gettext('Move'), key: 'move' },
    { label: $gettext('Permission'), key: 'permission' },
    {
      label: selectedRow.value.dir ? $gettext('Compress') : $gettext('Download'),
      key: selectedRow.value.dir ? 'compress' : 'download',
    },
    { label: $gettext('Rename'), key: 'rename' },
    {
      label: $gettext('Terminal'),
      key: 'terminal',
      show: selectedRow.value.dir,
    },
    { label: $gettext('Properties'), key: 'properties' },
    { label: () => h('span', { style: { color: 'red' } }, $gettext('Delete')), key: 'delete' },
  ]
  if (marked.value.length) {
    options.unshift({
      label: $gettext('Paste'),
      key: 'paste',
    })
  }
  return options
})

const openPermissionModal = (row: any) => {
  selected.value = [row.full]
  permissionFileInfoList.value = [row as FileInfo]
  permission.value = true
}

// 计算目录大小（列表视图用）
const calculateDirSize = (dirPath: string) => {
  sizeLoading.value.set(dirPath, true)
  useRequest(file.size(dirPath))
    .onSuccess(({ data }) => {
      sizeCache.value.set(dirPath, data)
    })
    .onComplete(() => {
      sizeLoading.value.set(dirPath, false)
    })
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
    editorMinimized.value = false
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
    return themeVars.value.warningColor
  }
  return themeVars.value.textColor3
}

// 检查项目是否被选中
// 使用 Set 做 O(1) 查找
const selectedSet = computed(() => new Set(selected.value))

const isSelected = (item: any) => {
  return selectedSet.value.has(item.full)
}

// 使用 Set 做 O(1) 查找剪切状态
const markedSet = computed(() => new Set(marked.value.map((m) => m.source)))

// 检查项目是否被标记为剪切（移动）
const isCut = (item: any) => {
  return markedType.value === 'move' && markedSet.value.has(item.full)
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

// 点击计数处理
let clickCount = 0
let clickTimer: ReturnType<typeof setTimeout> | null = null
let lastClickItem: any = null

// 处理项目点击
const handleItemClick = (item: any, event: MouseEvent) => {
  // 如果点击的是不同项目，重置计数
  if (lastClickItem !== item) {
    clickCount = 0
    if (clickTimer) {
      clearTimeout(clickTimer)
      clickTimer = null
    }
  }
  lastClickItem = item
  clickCount++

  if (clickCount >= 2) {
    // 双击：打开
    clickCount = 0
    openFile(item)
  } else {
    // 单击：选择
    toggleSelect(item, event)
    // 重置计数的定时器
    if (clickTimer) clearTimeout(clickTimer)
    clickTimer = setTimeout(() => {
      clickCount = 0
    }, 300)
  }
}

// 处理右键菜单
const handleContextMenu = (item: any, event: MouseEvent) => {
  event.preventDefault()
  showDropdown.value = false

  // 如果右键点击的项目不在已选中列表中，则只选中该项目
  if (!selectedSet.value.has(item.full)) {
    selected.value = [item.full]
  }

  nextTick().then(() => {
    isEmptyContextMenu.value = false
    showDropdown.value = true
    selectedRow.value = item
    dropdownX.value = event.clientX
    dropdownY.value = event.clientY
  })
}

// 处理空白区域右键菜单
const handleEmptyContextMenu = (event: MouseEvent) => {
  // 如果点击的是文件项，不处理
  const target = event.target as HTMLElement
  if (target.closest('.file-item')) return
  // 列表视图表头不触发
  if (target.closest('.list-header')) return

  event.preventDefault()
  showDropdown.value = false

  // 清除选中
  selected.value = []

  nextTick().then(() => {
    isEmptyContextMenu.value = true
    showDropdown.value = true
    selectedRow.value = null
    dropdownX.value = event.clientX
    dropdownY.value = event.clientY
  })
}

// 直接操作 DOM 更新选择框位置，避免 Vue 响应式开销
const updateSelectionBoxDOM = () => {
  const el = selectionBoxEl.value
  if (!el) return
  const left = Math.min(selStartX, selEndX)
  const top = Math.min(selStartY, selEndY)
  const width = Math.abs(selEndX - selStartX)
  const height = Math.abs(selEndY - selStartY)
  el.style.left = left + 'px'
  el.style.top = top + 'px'
  el.style.width = width + 'px'
  el.style.height = height + 'px'
}

// 框选开始
const onSelectionStart = (event: MouseEvent) => {
  // 只响应左键，且不在项目上
  if (event.button !== 0) return
  const target = event.target as HTMLElement
  if (target.closest('.file-item')) return
  // 列表视图表头不触发框选
  if (target.closest('.list-header')) return

  const container = gridContainerRef.value
  if (!container) return

  isSelecting.value = true
  const rect = container.getBoundingClientRect()
  cachedContainerRect = rect
  selStartX = event.clientX - rect.left + container.scrollLeft
  selStartY = event.clientY - rect.top + container.scrollTop
  selEndX = selStartX
  selEndY = selStartY

  // 缓存所有 item 的位置，框选期间不再重复查询 DOM
  const items = container.querySelectorAll('.file-item')
  const scrollLeft = container.scrollLeft
  const scrollTop = container.scrollTop
  cachedItemRects = []
  items.forEach((item) => {
    const fullPath = item.getAttribute('data-path')
    if (fullPath) {
      const itemRect = item.getBoundingClientRect()
      cachedItemRects.push({
        path: fullPath,
        left: itemRect.left - rect.left + scrollLeft,
        top: itemRect.top - rect.top + scrollTop,
        right: itemRect.right - rect.left + scrollLeft,
        bottom: itemRect.bottom - rect.top + scrollTop,
      })
    }
  })

  updateSelectionBoxDOM()

  // 如果没有按住 Ctrl/Cmd，清除已选
  if (!event.ctrlKey && !event.metaKey) {
    selected.value = []
  }
}

// 框选移动
let selectionRafId: number | null = null
const onSelectionMove = (event: MouseEvent) => {
  if (!isSelecting.value) return

  const container = gridContainerRef.value
  if (!container || !cachedContainerRect) return

  selEndX = event.clientX - cachedContainerRect.left + container.scrollLeft
  selEndY = event.clientY - cachedContainerRect.top + container.scrollTop

  // 直接 DOM 更新选择框
  updateSelectionBoxDOM()

  // rAF 节流碰撞检测
  if (selectionRafId === null) {
    selectionRafId = requestAnimationFrame(() => {
      updateSelectionFromBox()
      selectionRafId = null
    })
  }
}

// 框选结束
const onSelectionEnd = () => {
  if (!isSelecting.value) return
  // 最后一次碰撞检测确保结果准确
  updateSelectionFromBox()
  isSelecting.value = false
  cachedItemRects = []
  cachedContainerRect = null
  if (selectionRafId !== null) {
    cancelAnimationFrame(selectionRafId)
    selectionRafId = null
  }
}

// 根据选择框更新选中的项目
const updateSelectionFromBox = () => {
  if (cachedItemRects.length === 0) return

  const left = Math.min(selStartX, selEndX)
  const top = Math.min(selStartY, selEndY)
  const right = Math.max(selStartX, selEndX)
  const bottom = Math.max(selStartY, selEndY)

  const newSelected: string[] = []
  for (const item of cachedItemRects) {
    if (item.right >= left && item.left <= right && item.bottom >= top && item.top <= bottom) {
      newSelected.push(item.path)
    }
  }

  // 只在选中集合真正变化时才更新
  const currentSet = selectedSet.value
  if (newSelected.length !== currentSet.size || newSelected.some((v) => !currentSet.has(v))) {
    selected.value = newSelected
  }
}

// ==================== 公共操作函数 ====================

// 获取选中的文件列表
const getSelectedItems = () => {
  const set = selectedSet.value
  return data.value.filter((item: any) => set.has(item.full))
}

// 标记文件（复制/移动）
const markFiles = (items: any[], type: 'copy' | 'move') => {
  fileStore.setClipboard(
    items.map((item: any) => ({
      name: item.name,
      source: item.full,
      force: false,
    })),
    type,
  )
  window.$message.success(
    $gettext('Marked successfully, please navigate to the destination path to paste'),
  )
}

// 打开权限弹窗
const openPermission = (items: any[]) => {
  selected.value = items.map((item: any) => item.full)
  permissionFileInfoList.value = items as FileInfo[]
  permission.value = true
}

// 打开压缩弹窗
const openCompress = (items: any[]) => {
  selected.value = items.map((item: any) => item.full)
  compress.value = true
}

// 打开解压弹窗
const openUncompress = (item: any) => {
  unCompressModel.value.file = item.full
  unCompressModel.value.path = path.value
  unCompressModal.value = true
}

// 打开终端
const openTerminal = (item: any) => {
  terminalPath.value = item.full
  terminalModal.value = true
}

// 打开属性弹窗
const openProperty = (item: any) => {
  propertyFileInfo.value = item as FileInfo
  propertyModal.value = true
}

// 启动内联重命名
const openRename = (item: any) => {
  confirmImmutableOperation(item, () => {
    inlineRenameItem.value = item
    inlineRenameName.value = getFilename(item.name)
    // 等待 DOM 更新后聚焦输入框
    nextTick(() => {
      if (inlineRenameInputRef.value) {
        inlineRenameInputRef.value.focus()
        // 选中文件名（不包括扩展名）
        const dotIndex = inlineRenameName.value.lastIndexOf('.')
        if (dotIndex > 0 && !item.dir) {
          inlineRenameInputRef.value.setSelectionRange(0, dotIndex)
        } else {
          inlineRenameInputRef.value.select()
        }
      }
    })
  })
}

// 取消内联重命名
const cancelInlineRename = () => {
  inlineRenameItem.value = null
  inlineRenameName.value = ''
}

// 提交内联重命名
const submitInlineRename = () => {
  if (!inlineRenameItem.value) return

  const item = inlineRenameItem.value
  const sourceName = getFilename(item.name)
  const targetName = inlineRenameName.value.trim()

  // 如果名称没有变化，直接取消
  if (sourceName === targetName) {
    cancelInlineRename()
    return
  }

  // 验证名称
  if (!checkName(targetName)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  const source = path.value + '/' + sourceName
  const target = path.value + '/' + targetName

  useRequest(file.exist([target])).onSuccess(({ data: existData }) => {
    if (existData[0]) {
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
                  source: sourceName,
                  target: targetName,
                }),
              )
            })
            .onComplete(() => {
              cancelInlineRename()
            })
        },
        onNegativeClick: () => {
          // 保持编辑状态，让用户修改名称
        },
      })
    } else {
      useRequest(file.move([{ source, target, force: false }]))
        .onSuccess(() => {
          window.$bus.emit('file:refresh')
          window.$message.success(
            $gettext('Renamed %{ source } to %{ target } successfully', {
              source: sourceName,
              target: targetName,
            }),
          )
        })
        .onComplete(() => {
          cancelInlineRename()
        })
    }
  })
}

// 处理内联重命名键盘事件
const handleInlineRenameKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Enter') {
    event.preventDefault()
    submitInlineRename()
  } else if (event.key === 'Escape') {
    event.preventDefault()
    cancelInlineRename()
  }
}

// 删除文件
const deleteFiles = (items: any[]) => {
  if (items.length === 1) {
    confirmImmutableOperation(items[0], () => {
      useRequest(file.delete(items[0].full)).onSuccess(() => {
        window.$bus.emit('file:refresh')
        window.$message.success($gettext('Deleted successfully'))
      })
    })
  } else {
    const hasImmutable = items.some((item: any) => item.immutable)
    if (hasImmutable) {
      window.$message.warning($gettext('Some files are immutable and cannot be deleted'))
      return
    }
    window.$dialog.warning({
      title: $gettext('Warning'),
      content: $gettext('Are you sure you want to delete %{count} items?', {
        count: items.length,
      }),
      positiveText: $gettext('Yes'),
      negativeText: $gettext('No'),
      onPositiveClick: () => {
        const deletePromises = items.map((item: any) => file.delete(item.full))
        Promise.all(deletePromises).then(() => {
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Deleted successfully'))
        })
      },
    })
  }
}

// 复制路径到剪贴板
const copyPath = (item: any) => {
  copy2clipboard(item.full).then(() => {
    window.$message.success($gettext('Path copied to clipboard'))
  })
}

// ==================== 处理粘贴 ====================
const handlePaste = () => {
  doPaste(path.value)
}

const handleSelect = (key: string) => {
  // 空白区域菜单选项
  if (isEmptyContextMenu.value) {
    switch (key) {
      case 'paste':
        handlePaste()
        break
      case 'new-file':
        startInlineCreate(false)
        break
      case 'new-folder':
        startInlineCreate(true)
        break
    }
    onCloseDropdown()
    return
  }

  const items = isMultiSelect.value ? getSelectedItems() : [selectedRow.value]

  switch (key) {
    case 'paste':
      handlePaste()
      break
    case 'open':
    case 'edit':
    case 'preview':
      openFile(selectedRow.value)
      break
    case 'uncompress':
      openUncompress(selectedRow.value)
      break
    case 'copy':
      markFiles(items, 'copy')
      break
    case 'move':
      markFiles(items, 'move')
      break
    case 'permission':
      openPermission(items)
      break
    case 'compress':
      openCompress(items)
      break
    case 'download':
      window.open('/api/file/download?path=' + encodeURIComponent(selectedRow.value.full))
      break
    case 'rename':
      openRename(selectedRow.value)
      break
    case 'terminal':
      openTerminal(selectedRow.value)
      break
    case 'properties':
      openProperty(selectedRow.value)
      break
    case 'delete':
      deleteFiles(items)
      break
  }
  onCloseDropdown()
}

const onCloseDropdown = () => {
  selectedRow.value = null
  showDropdown.value = false
}

// 操作列点击处理
const handleRenameClick = (item: any) => {
  openRename(item)
}

const handleDeleteClick = (item: any) => {
  deleteFiles([item])
}

// 更多操作选项
const getMoreOptions = (item: any): SelectOption[] => {
  const options: SelectOption[] = [
    { label: $gettext('Copy'), value: 'copy' },
    { label: $gettext('Move'), value: 'move' },
    { label: $gettext('Permission'), value: 'permission' },
    { label: $gettext('Compress'), value: 'compress' },
  ]

  // 如果是压缩文件，添加解压选项
  if (isCompress(item.full)) {
    options.push({ label: $gettext('Uncompress'), value: 'uncompress' })
  }

  options.push({ label: $gettext('Copy Path'), value: 'copy-path' })

  // 如果是文件夹，添加终端选项
  if (item.dir) {
    options.push({ label: $gettext('Terminal'), value: 'terminal' })
  }

  options.push({ label: $gettext('Properties'), value: 'properties' })

  return options
}

// 处理更多操作选择
// 下载文件
const handleDownload = (item: any) => {
  window.open('/api/file/download?path=' + encodeURIComponent(item.full))
}

const handleMoreSelect = (key: string, item: any) => {
  switch (key) {
    case 'copy':
      markFiles([item], 'copy')
      break
    case 'move':
      markFiles([item], 'move')
      break
    case 'permission':
      openPermission([item])
      break
    case 'compress':
      openCompress([item])
      break
    case 'uncompress':
      openUncompress(item)
      break
    case 'copy-path':
      copyPath(item)
      break
    case 'terminal':
      openTerminal(item)
      break
    case 'properties':
      openProperty(item)
      break
  }
}

// 获取当前选中项的索引
const getSelectedIndex = () => {
  if (selected.value.length === 0) return -1
  const lastSelected = selected.value[selected.value.length - 1]
  return data.value.findIndex((item: any) => item.full === lastSelected)
}

// 计算网格模式下每行的列数
const getGridColumns = () => {
  const container = gridContainerRef.value
  if (!container || fileStore.viewType !== 'grid') return 1
  const containerWidth = container.clientWidth - 32 // 减去 padding
  const itemMinWidth = 100 + 16 // minmax(100px, 1fr) + gap
  return Math.floor(containerWidth / itemMinWidth) || 1
}

// 导航到指定索引的项目
const navigateToIndex = (index: number, addToSelection = false) => {
  if (index < 0 || index >= data.value.length) return
  const item = data.value[index]
  if (addToSelection) {
    if (!selectedSet.value.has(item.full)) {
      selected.value.push(item.full)
    }
  } else {
    selected.value = [item.full]
  }
  // 滚动到可见区域
  const container = gridContainerRef.value
  if (container) {
    const itemEl = container.querySelector(`[data-path="${CSS.escape(item.full)}"]`)
    if (itemEl) {
      itemEl.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  }
}

// 返回上级目录
const goToParentDir = () => {
  if (path.value === '/') return
  path.value = path.value.substring(0, path.value.lastIndexOf('/')) || '/'
}

// 键盘快捷键暂停状态
const keyboardPaused = ref(false)

const pauseKeyboard = () => {
  keyboardPaused.value = true
}

const resumeKeyboard = () => {
  keyboardPaused.value = false
}

// 键盘快捷键处理
const handleKeyDown = (event: KeyboardEvent) => {
  // 如果键盘监听被暂停（如编辑器打开时），不处理快捷键
  if (keyboardPaused.value) {
    return
  }

  // 如果焦点在输入框中，不处理快捷键
  const target = event.target as HTMLElement
  if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
    return
  }

  // 检测 Ctrl (Windows) 或 Command (macOS)
  const isCtrlOrCmd = event.ctrlKey || event.metaKey

  if (isCtrlOrCmd) {
    switch (event.key.toLowerCase()) {
      case 'a':
        // Ctrl/Cmd + A: 全选
        event.preventDefault()
        selected.value = data.value.map((item: any) => item.full)
        break
      case 'c':
        // Ctrl/Cmd + C: 复制
        if (selected.value.length > 0) {
          event.preventDefault()
          markFiles(getSelectedItems(), 'copy')
        }
        break
      case 'x':
        // Ctrl/Cmd + X: 剪切（移动）
        if (selected.value.length > 0) {
          event.preventDefault()
          markFiles(getSelectedItems(), 'move')
        }
        break
      case 'v':
        // Ctrl/Cmd + V: 粘贴
        if (marked.value.length > 0) {
          event.preventDefault()
          handlePaste()
        }
        break
      case 't':
        // Ctrl/Cmd + T: 新建标签页
        event.preventDefault()
        fileStore.createTab()
        break
      case 'w':
        // Ctrl/Cmd + W: 关闭当前标签页
        event.preventDefault()
        fileStore.closeTab(props.tabId)
        break
    }
  } else {
    const currentIndex = getSelectedIndex()
    const columns = getGridColumns()

    switch (event.key) {
      case 'ArrowUp':
        event.preventDefault()
        if (fileStore.viewType === 'grid') {
          // 网格模式：上移一行
          navigateToIndex(currentIndex - columns, event.shiftKey)
        } else {
          // 列表模式：上移一项
          navigateToIndex(currentIndex - 1, event.shiftKey)
        }
        break
      case 'ArrowDown':
        event.preventDefault()
        if (fileStore.viewType === 'grid') {
          // 网格模式：下移一行
          navigateToIndex(currentIndex + columns, event.shiftKey)
        } else {
          // 列表模式：下移一项
          navigateToIndex(currentIndex + 1, event.shiftKey)
        }
        break
      case 'ArrowLeft':
        event.preventDefault()
        if (fileStore.viewType === 'grid') {
          // 网格模式：左移一项
          navigateToIndex(currentIndex - 1, event.shiftKey)
        }
        break
      case 'ArrowRight':
        event.preventDefault()
        if (fileStore.viewType === 'grid') {
          // 网格模式：右移一项
          navigateToIndex(currentIndex + 1, event.shiftKey)
        }
        break
      case 'Home':
        event.preventDefault()
        navigateToIndex(0, event.shiftKey)
        break
      case 'End':
        event.preventDefault()
        navigateToIndex(data.value.length - 1, event.shiftKey)
        break
      case 'F2':
        // F2: 重命名选中项（单选时）
        if (selected.value.length === 1) {
          event.preventDefault()
          const item = data.value.find((item: any) => item.full === selected.value[0])
          if (item) {
            openRename(item)
          }
        }
        break
      case 'Backspace':
        // Backspace: 返回上级目录
        event.preventDefault()
        goToParentDir()
        break
      case 'Delete':
        // Delete: 删除选中项
        if (selected.value.length > 0) {
          event.preventDefault()
          deleteFiles(getSelectedItems())
        }
        break
      case 'Escape':
        // Escape: 取消选择
        event.preventDefault()
        selected.value = []
        break
      case 'Enter':
        // Enter: 打开选中项（单选时）
        if (selected.value.length === 1) {
          event.preventDefault()
          const item = data.value.find((item: any) => item.full === selected.value[0])
          if (item) {
            openFile(item)
          }
        }
        break
    }
  }
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
    duration: 0,
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
  refresh,
} = usePagination(
  (page, pageSize) =>
    file.list(encodeURIComponent(path.value), keyword.value, sub.value, sort.value, page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 100,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)

const data = computed(() => {
  if (fileStore.showHidden) {
    return rawData.value
  }
  return rawData.value.filter((item: any) => !item.hidden)
})

// 搜索事件处理函数
const handleFileSearch = () => {
  selected.value = []
  nextTick(() => {
    refresh()
  })
  fileStore.pushHistory(props.tabId, path.value)
}

onMounted(() => {
  watch(
    path,
    () => {
      selected.value = []
      sizeCache.value.clear()
      sizeLoading.value.clear()
      nextTick(() => {
        refresh()
      })
    },
    { immediate: true },
  )

  // 监听排序变化
  watch(sort, () => {
    nextTick(() => {
      refresh()
    })
  })

  window.$bus.on('file:search', handleFileSearch)
  window.$bus.on('file:refresh', refresh)
  window.$bus.on('file:keyboard-pause', pauseKeyboard)
  window.$bus.on('file:keyboard-resume', resumeKeyboard)
  window.$bus.on('file:inline-create', startInlineCreate)

  // 添加全局鼠标事件监听
  document.addEventListener('mousemove', onSelectionMove)
  document.addEventListener('mouseup', onSelectionEnd)

  // 添加键盘快捷键监听
  document.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  // 清理点击计时器
  if (clickTimer) {
    clearTimeout(clickTimer)
    clickTimer = null
  }

  // 移除事件监听
  window.$bus.off('file:search', handleFileSearch)
  window.$bus.off('file:refresh', refresh)
  window.$bus.off('file:keyboard-pause', pauseKeyboard)
  window.$bus.off('file:keyboard-resume', resumeKeyboard)
  window.$bus.off('file:inline-create', startInlineCreate)
  document.removeEventListener('mousemove', onSelectionMove)
  document.removeEventListener('mouseup', onSelectionEnd)
  document.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <div class="flex flex-col flex-1 gap-1 min-h-0" :style="themeStyles">
    <n-spin :show="loading" class="flex flex-col flex-1 min-h-0">
      <div
        ref="gridContainerRef"
        class="file-container"
        :class="{
          'grid-mode': fileStore.viewType === 'grid',
          'list-mode': fileStore.viewType === 'list',
        }"
        @mousedown="onSelectionStart"
        @contextmenu="handleEmptyContextMenu"
      >
        <!-- 框选框 -->
        <div v-show="isSelecting" ref="selectionBoxEl" class="selection-box" />

        <!-- 列表视图表头 -->
        <div v-if="fileStore.viewType === 'list'" class="list-header">
          <div class="list-col col-checkbox">
            <n-checkbox
              :checked="isAllSelected"
              :indeterminate="isIndeterminate"
              @update:checked="toggleSelectAll"
              @click.stop
            />
          </div>
          <div
            class="list-col col-name hover:text-primary cursor-pointer select-none"
            @click="handleSort('name')"
          >
            {{ $gettext('Name') }}
            <the-icon :icon="getSortIcon('name')" :size="16" class="align-middle opacity-50" />
          </div>
          <div
            class="list-col col-size hover:text-primary cursor-pointer select-none"
            @click="handleSort('size')"
          >
            {{ $gettext('Size') }}
            <the-icon :icon="getSortIcon('size')" :size="16" class="align-middle opacity-50" />
          </div>
          <div class="list-col col-mode">{{ $gettext('Permission') }}</div>
          <div class="list-col col-owner">{{ $gettext('Owner / Group') }}</div>
          <div
            class="list-col col-modify hover:text-primary cursor-pointer select-none"
            @click="handleSort('modify')"
          >
            {{ $gettext('Modification Time') }}
            <the-icon :icon="getSortIcon('modify')" :size="16" class="align-middle opacity-50" />
          </div>
          <div class="list-col col-actions">{{ $gettext('Actions') }}</div>
        </div>

        <!-- 内联新建项 -->
        <div v-if="inlineCreateActive" class="file-item inline-create-item">
          <!-- 图标视图 -->
          <template v-if="fileStore.viewType === 'grid'">
            <div class="icon-wrapper">
              <the-icon
                :icon="inlineCreateIsDir ? 'mdi:folder' : 'mdi:file-document-outline'"
                :size="48"
                :color="inlineCreateIsDir ? themeVars.warningColor : themeVars.textColor3"
              />
            </div>
            <input
              :ref="setInlineCreateRef"
              v-model="inlineCreateName"
              class="inline-rename-input"
              :placeholder="inlineCreateIsDir ? $gettext('Folder name') : $gettext('File name')"
              @blur="submitInlineCreate"
              @keydown="handleInlineCreateKeydown"
              @click.stop
            />
          </template>
          <!-- 列表视图 -->
          <template v-else>
            <div class="list-col col-checkbox">
              <n-checkbox disabled />
            </div>
            <div class="list-col col-name">
              <the-icon
                :icon="inlineCreateIsDir ? 'mdi:folder' : 'mdi:file-document-outline'"
                :size="20"
                :color="inlineCreateIsDir ? themeVars.warningColor : themeVars.textColor3"
              />
              <input
                :ref="setInlineCreateRef"
                v-model="inlineCreateName"
                class="inline-rename-input list-mode"
                :placeholder="inlineCreateIsDir ? $gettext('Folder name') : $gettext('File name')"
                @blur="submitInlineCreate"
                @keydown="handleInlineCreateKeydown"
                @click.stop
              />
            </div>
            <div class="list-col col-size">-</div>
            <div class="list-col col-mode">-</div>
            <div class="list-col col-owner">-</div>
            <div class="list-col col-modify">-</div>
            <div class="list-col col-actions">-</div>
          </template>
        </div>

        <!-- 文件/文件夹列表 -->
        <div
          v-for="item in data"
          :key="item.full"
          class="file-item"
          :class="{ selected: isSelected(item), cut: isCut(item) }"
          :data-path="item.full"
          @click="handleItemClick(item, $event)"
          @contextmenu="handleContextMenu(item, $event)"
        >
          <!-- 图标视图 -->
          <template v-if="fileStore.viewType === 'grid'">
            <div class="icon-wrapper">
              <the-icon :icon="getFileIcon(item)" :size="48" :color="getIconColor(item)" />
              <the-icon v-if="item.immutable" icon="mdi:lock" :size="16" class="lock-icon" />
            </div>
            <!-- 内联重命名输入框 -->
            <input
              v-if="inlineRenameItem?.full === item.full"
              :ref="setInlineRenameRef"
              v-model="inlineRenameName"
              class="inline-rename-input"
              @blur="submitInlineRename"
              @keydown="handleInlineRenameKeydown"
              @click.stop
            />
            <span v-else class="text-center max-w-full">
              <n-ellipsis
                :line-clamp="2"
                class="text-xs leading-normal break-all"
                :tooltip="{ width: 300 }"
              >
                {{ item.name }}
              </n-ellipsis>
            </span>
          </template>

          <!-- 列表视图 -->
          <template v-else>
            <div class="list-col col-checkbox">
              <n-checkbox
                :checked="isSelected(item)"
                @update:checked="toggleCheckbox(item)"
                @click.stop
              />
            </div>
            <div class="list-col col-name">
              <the-icon :icon="getFileIcon(item)" :size="20" :color="getIconColor(item)" />
              <!-- 内联重命名输入框 -->
              <input
                v-if="inlineRenameItem?.full === item.full"
                :ref="setInlineRenameRef"
                v-model="inlineRenameName"
                class="inline-rename-input list-mode"
                @blur="submitInlineRename"
                @keydown="handleInlineRenameKeydown"
                @click.stop
              />
              <n-ellipsis v-else class="flex-1 overflow-hidden" :tooltip="{ width: 300 }">
                {{ item.symlink ? item.name + ' -> ' + item.link : item.name }}
              </n-ellipsis>
              <the-icon
                v-if="item.immutable"
                icon="mdi:lock"
                :size="14"
                class="text-warning ml-1 shrink-0"
              />
            </div>
            <div class="list-col col-size">
              <n-tag v-if="!item.dir" type="info" size="small" :bordered="false">{{
                item.size
              }}</n-tag>
              <span
                v-else
                class="text-primary text-xs cursor-pointer hover:opacity-80"
                @click.stop="calculateDirSize(item.full)"
              >
                <n-spin v-if="sizeLoading.get(item.full)" />
                <template v-else>{{ sizeCache.get(item.full) || $gettext('Calculate') }}</template>
              </span>
            </div>
            <div class="list-col col-mode">
              <n-tag
                type="success"
                size="small"
                :bordered="false"
                class="cursor-pointer hover:opacity-80"
                @click.stop="openPermissionModal(item)"
              >
                {{ item.mode }}
              </n-tag>
            </div>
            <div class="list-col col-owner">
              <span class="cursor-pointer hover:opacity-80" @click.stop="openPermissionModal(item)">
                <n-tag type="primary" size="small" :bordered="false">{{ item.owner }}</n-tag>
                /
                <n-tag type="primary" size="small" :bordered="false">{{ item.group }}</n-tag>
              </span>
            </div>
            <div class="list-col col-modify">
              <n-tag type="warning" size="small" :bordered="false">{{ item.modify }}</n-tag>
            </div>
            <div class="list-col col-actions">
              <n-flex :size="4">
                <!-- 文件夹操作 -->
                <template v-if="item.dir">
                  <n-button size="tiny" type="success" tertiary @click.stop="openFile(item)">
                    {{ $gettext('Open') }}
                  </n-button>
                  <n-button
                    size="tiny"
                    type="primary"
                    tertiary
                    @click.stop="
                      () => {
                        selected = [item.full]
                        compress = true
                      }
                    "
                  >
                    {{ $gettext('Compress') }}
                  </n-button>
                </template>
                <!-- 文件操作 -->
                <template v-else>
                  <n-button size="tiny" type="primary" tertiary @click.stop="openFile(item)">
                    {{ isImage(item.name) ? $gettext('Preview') : $gettext('Edit') }}
                  </n-button>
                  <n-button size="tiny" type="info" tertiary @click.stop="handleDownload(item)">
                    {{ $gettext('Download') }}
                  </n-button>
                </template>
                <!-- 通用操作 -->
                <n-button size="tiny" type="warning" tertiary @click.stop="handleRenameClick(item)">
                  {{ $gettext('Rename') }}
                </n-button>
                <ConfirmDialog
                  type="delete"
                  :content="
                    $gettext('Are you sure you want to delete %{ name }?', { name: item.name })
                  "
                  @confirm="handleDeleteClick(item)"
                >
                  <template #trigger>
                    <n-button size="tiny" type="error" tertiary @click.stop>
                      {{ $gettext('Delete') }}
                    </n-button>
                  </template>
                </ConfirmDialog>
                <n-popselect
                  :options="getMoreOptions(item)"
                  trigger="click"
                  @update:value="(key: string) => handleMoreSelect(key, item)"
                >
                  <n-button size="tiny" tertiary @click.stop>
                    {{ $gettext('More') }}
                  </n-button>
                </n-popselect>
              </n-flex>
            </div>
          </template>
        </div>

        <!-- 空状态 -->
        <div v-if="data.length === 0 && !loading" class="empty-state">
          <the-icon icon="mdi:folder-open-outline" :size="64" class="opacity-30" />
          <p>{{ $gettext('No files') }}</p>
        </div>
      </div>
    </n-spin>

    <!-- 底部状态栏 -->
    <n-flex justify="space-between" align="center" class="py-2">
      <div class="flex gap-2 items-center">
        <template v-if="selected.length > 0">
          <n-tag type="primary" size="small">
            {{ $gettext('%{count} item(s) selected', { count: selected.length }) }}
          </n-tag>
          <n-button size="tiny" quaternary @click="selected = []">
            {{ $gettext('Clear') }}
          </n-button>
        </template>
        <template v-else>
          <span class="text-base text-gray-400">{{
            $gettext('%{count} item(s)', { count: data.length })
          }}</span>
        </template>
      </div>
      <n-pagination
        v-model:page="page"
        v-model:page-size="pageSize"
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
  <edit-modal
    v-model:show="editorModal"
    v-model:minimized="editorMinimized"
    v-model:file="currentFile"
  />
  <!-- 预览弹窗 -->
  <preview-modal v-model:show="previewModal" v-model:path="currentFile" />
  <!-- 解压弹窗 -->
  <n-modal
    v-model:show="unCompressModal"
    preset="card"
    :title="$gettext('Uncompress - %{ file }', { file: unCompressModel.file })"
    class="w-60vw"
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
    :command="`cd '${terminalPath}' && exec bash -l`"
  />
</template>

<style scoped lang="scss">
.file-container {
  position: relative;
  flex: 1;
  overflow: auto;
  background: var(--card-color);
  border-radius: 3px;
  border: 1px solid var(--border-color);
  user-select: none;

  // 图标视图模式
  &.grid-mode {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
    align-content: start;
    gap: 16px;
    padding: 16px;
    border: none;

    .file-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 12px 8px;
      border-radius: 4px;
      cursor: pointer;
      transition: all 0.1s ease;
      border: 1px solid transparent;

      &:hover {
        background: var(--hover-bg);
        border-color: var(--hover-border);
      }

      &.selected {
        background: var(--primary-color-hover);
        border-color: var(--primary-color-border);
      }

      &.selected:hover {
        background: var(--primary-color-hover-deep);
        border-color: var(--primary-color-border-deep);
      }

      &.cut {
        opacity: 0.5;
      }
    }
  }

  // 列表视图模式
  &.list-mode {
    display: flex;
    flex-direction: column;

    .list-header {
      display: flex;
      align-items: center;
      padding: 8px 16px 8px 8px;
      background: var(--card-color);
      border-bottom: 1px solid var(--border-color);
      font-weight: 500;
      font-size: 13px;
      position: sticky;
      top: 0;
      z-index: 10;
    }

    .file-item {
      display: flex;
      align-items: center;
      padding: 6px 16px 6px 8px;
      border-bottom: 1px solid var(--border-color);
      cursor: pointer;
      transition: all 0.1s ease;

      &:hover {
        background: var(--hover-bg);
      }

      &.selected {
        background: var(--primary-color-hover);
      }

      &.selected:hover {
        background: var(--primary-color-hover-deep);
      }

      &.cut {
        opacity: 0.5;
      }
    }
  }
}

// 列表视图列宽
.list-col {
  display: flex;
  align-items: center;
  gap: 8px;

  &.col-checkbox {
    width: 24px;
    flex-shrink: 0;
    justify-content: center;
  }

  &.col-name {
    flex: 1;
    min-width: 200px;
    overflow: hidden;
  }

  &.col-size {
    width: 140px;
  }

  &.col-mode {
    width: 120px;
  }

  &.col-owner {
    width: 200px;
  }

  &.col-modify {
    width: 200px;
  }

  &.col-actions {
    width: 240px;
    flex-shrink: 0;
  }
}
.selection-box {
  position: absolute;
  border: 1px solid var(--selection-border);
  background: var(--selection-bg);
  pointer-events: none;
  z-index: 100;
}

// 图标视图专用样式
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
  color: var(--warning-color);
}

// 内联重命名输入框样式
.inline-rename-input {
  width: 100%;
  max-width: 100%;
  padding: 2px 6px;
  font-size: 12px;
  line-height: 1.4;
  border: 1px solid var(--primary-color);
  border-radius: 3px;
  background: var(--card-color);
  color: var(--text-color);
  outline: none;
  text-align: center;
  box-sizing: border-box;

  &:focus {
    box-shadow: 0 0 0 2px var(--primary-color-hover);
  }

  // 列表模式下的样式
  &.list-mode {
    flex: 1;
    width: auto;
    margin-right: 8px;
    text-align: left;
    font-size: 14px;
    padding: 2px 8px;
  }
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  color: var(--text-color-3);

  .grid-mode & {
    grid-column: 1 / -1;
  }

  .list-mode & {
    flex: 1;
  }
}

// n-spin 内部容器填满
:deep(.n-spin-content) {
  height: 100%;
  display: flex;
  flex-direction: column;
}
</style>

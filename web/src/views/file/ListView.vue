<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import type { DropdownOption } from 'naive-ui'
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
import tamper from '@/api/panel/tamper'
import { hasOpenWindow } from '@/components/common/composables/useWindowStack'
import { useBus } from '@/components/system/composables/useBus'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { useFileStore, useTaskQueueStore } from '@/stores'
import { checkPath, getExt, getIconByExt, isCompress, isImage } from '@/utils/file'
import { useBoxSelection } from '@/views/file/composables/useBoxSelection'
import { useFileOps } from '@/views/file/composables/useFileOps'
import { useInlineEdit } from '@/views/file/composables/useInlineEdit'
import { usePaste } from '@/views/file/composables/usePaste'
import PreviewModal from '@/views/file/PreviewModal.vue'
import PropertyModal from '@/views/file/PropertyModal.vue'
import ShareModal from '@/views/file/ShareModal.vue'
import type { FileInfo } from '@/views/file/types'

const { $gettext } = useGettext()
const themeVars = useThemeVars()
const fileStore = useFileStore()
const taskQueue = useTaskQueueStore()
const { handlePaste: doPaste } = usePaste()
const { deletePaths, markClipboard, toggleFavorite } = useFileOps()
const { isMobile } = useResponsive()

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

// 选中状态归属 fileStore（按标签页隔离），此处仅做读写代理
// 读到的可能是兜底的临时空数组，所以类型上只读，修改一律整体赋值
const selected = computed<readonly string[], string[]>({
  get: () => fileStore.selected[props.tabId] ?? [],
  set: (v) => {
    fileStore.selected[props.tabId] = v
  },
})
// 使用 Set 做 O(1) 查找
const selectedSet = computed(() => new Set(selected.value))

const compress = defineModel<boolean>('compress', { type: Boolean, required: true })
const permission = defineModel<boolean>('permission', { type: Boolean, required: true })

const previewModal = ref(false)
const currentFile = ref('')
const propertyModal = ref(false)
const propertyFileInfo = ref<FileInfo | null>(null)
const terminalModal = ref(false)
const terminalPath = ref('')
const shareModal = ref(false)
const shareFile = ref('')

const showDropdown = ref(false)
const selectedRow = ref<any>()
const dropdownX = ref(0)
const dropdownY = ref(0)

const {
  inlineCreateActive,
  inlineCreateIsDir,
  inlineCreateName,
  setInlineCreateRef,
  startInlineCreate,
  submitInlineCreate,
  handleInlineCreateKeydown,
  inlineRenameItem,
  inlineRenameName,
  setInlineRenameRef,
  startInlineRename,
  submitInlineRename,
  handleInlineRenameKeydown,
} = useInlineEdit(path)

const unCompressModal = ref(false)
const unCompressModel = ref({
  path: '',
  file: '',
})

// 目录大小计算状态（列表视图用）
const sizeLoading = ref<Map<string, boolean>>(new Map())
const sizeCache = ref<Map<string, string>>(new Map())

const gridContainerRef = ref<HTMLElement | null>(null)
const { isSelecting, selectionBoxEl, onSelectionStart } = useBoxSelection(
  gridContainerRef,
  selected,
  selectedSet,
)

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

// ==================== 防篡改保护 ====================
const tamperRunning = ref(false)
const protectedSet = ref<Set<string>>(new Set())

const isProtected = (item: any) => protectedSet.value.has(item.full)

// 拉取当前列表各路径的保护状态(防篡改未运行时后端返回全 false)
const refreshProtected = (list: any[]) => {
  const paths = list.map((item: any) => item.full)
  if (!paths.length) {
    protectedSet.value = new Set()
    return
  }
  useRequest(tamper.checkPaths(paths)).onSuccess(({ data }: any) => {
    tamperRunning.value = data.running
    protectedSet.value = new Set(Object.keys(data.items || {}).filter((p) => data.items[p]))
  })
}

// 添加/移除保护(二次确认)
const handleProtect = (item: any, protect: boolean) => {
  window.$dialog.warning({
    title: protect ? $gettext('Add Protection') : $gettext('Remove Protection'),
    content: protect
      ? $gettext('Add tamper protection for %{ name }?', { name: item.name })
      : $gettext(
          'Remove tamper protection for %{ name }? The matching rule or exclusions will be adjusted accordingly.',
          { name: item.name },
        ),
    positiveText: $gettext('Yes'),
    negativeText: $gettext('No'),
    onPositiveClick: () => {
      useRequest(tamper.protect(item.full, protect)).onSuccess(() => {
        window.$message.success($gettext('Operation successful'))
        refresh()
      })
    },
  })
}

// ==================== 文件分享 ====================
const sharedPaths = ref<Set<string>>(new Set())

const isShared = (item: any) => sharedPaths.value.has(item.full)

// 拉取有效分享的路径集合，用于列表分享标识
const refreshShares = () => {
  useRequest(file.shareList()).onSuccess(({ data }: any) => {
    const now = Date.now()
    sharedPaths.value = new Set(
      (data || [])
        .filter((item: any) => new Date(item.expired_at).getTime() > now)
        .map((item: any) => item.path),
    )
  })
}

const openShare = (item: any) => {
  shareFile.value = item.full
  shareModal.value = true
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

// 切换单个项目的选中状态（复选框点击、Ctrl/Cmd + 点击）
const toggleCheckbox = (item: any) => {
  selected.value = selectedSet.value.has(item.full)
    ? selected.value.filter((p) => p !== item.full)
    : [...selected.value, item.full]
}

// 空白区域右键菜单是否激活
const isEmptyContextMenu = ref(false)

// 「打开」对不同类型的叫法，与 openFile 的分支一一对应
const openLabel = (item: any) => {
  if (item.dir) return $gettext('Open')
  if (isImage(item.name)) return $gettext('Preview')
  if (isCompress(item.name)) return $gettext('Uncompress')
  return $gettext('Edit')
}

// 单个文件/目录的全部菜单项，右键菜单、「更多」菜单与手机下拉按 key 从中挑选
const getItemOptions = (item: any): DropdownOption[] => [
  { label: $gettext('Paste'), key: 'paste', show: marked.value.length > 0 },
  { label: openLabel(item), key: 'open' },
  { label: $gettext('Copy'), key: 'copy' },
  { label: $gettext('Move'), key: 'move' },
  { label: $gettext('Permission'), key: 'permission' },
  { label: $gettext('Compress'), key: 'compress' },
  { label: $gettext('Download'), key: 'download' },
  { label: $gettext('Share'), key: 'share', show: !item.dir },
  { label: $gettext('Copy Path'), key: 'copy-path' },
  { label: $gettext('Rename'), key: 'rename' },
  { label: $gettext('Terminal'), key: 'terminal', show: item.dir },
  {
    label: fileStore.isFavorite(item.full)
      ? $gettext('Remove from Favorites')
      : $gettext('Add to Favorites'),
    key: 'favorite',
    show: item.dir,
  },
  {
    label: isProtected(item) ? $gettext('Remove Protection') : $gettext('Add Protection'),
    key: 'protect',
    show: tamperRunning.value,
  },
  { label: $gettext('Properties'), key: 'properties' },
  { label: () => h('span', { style: { color: 'red' } }, $gettext('Delete')), key: 'delete' },
]

// 按 keys 的顺序挑出要显示的菜单项
const pickItemOptions = (item: any, keys: string[]): DropdownOption[] => {
  const all = getItemOptions(item)
  return keys.flatMap((key) => all.filter((o) => o.key === key && o.show !== false))
}

// 桌面「更多」菜单里的项
const moreKeys = [
  'copy',
  'move',
  'permission',
  'compress',
  'share',
  'copy-path',
  'terminal',
  'favorite',
  'protect',
  'properties',
]

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
  const row = selectedRow.value
  if (row == null) return []
  return pickItemOptions(row, [
    'paste',
    'open',
    'copy',
    'move',
    'permission',
    row.dir ? 'compress' : 'download',
    'share',
    'rename',
    'terminal',
    'favorite',
    'protect',
    'properties',
    'delete',
  ])
})

const openPermissionModal = (row: any) => {
  selected.value = [row.full]
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
    // 编辑器由 IndexView 承载，跨文件标签页共享同一实例
    window.$bus.emit('file:edit', row.full)
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
    toggleCheckbox(item)
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

// ==================== 公共操作函数 ====================

// 获取选中的文件列表
const getSelectedItems = () => {
  const set = selectedSet.value
  return data.value.filter((item: any) => set.has(item.full))
}

// 标记文件（复制/移动）
const markFiles = (items: any[], type: 'copy' | 'move') => {
  markClipboard(
    items.map((item: any) => item.full),
    type,
  )
}

// 打开权限弹窗
const openPermission = (items: any[]) => {
  selected.value = items.map((item: any) => item.full)
  permission.value = true
}

// 打开压缩弹窗
const openCompress = (items: any[]) => {
  selected.value = items.map((item: any) => item.full)
  compress.value = true
}

// 打开终端
const openTerminal = (item: any) => {
  terminalPath.value = item.full
  terminalModal.value = true
}

watch(terminalModal, (show) => {
  if (!show) window.$bus.emit('file:refresh')
})

// 打开属性弹窗
const openProperty = (item: any) => {
  propertyFileInfo.value = item as FileInfo
  propertyModal.value = true
}

// 启动内联重命名
const openRename = (item: any) => {
  confirmImmutableOperation(item, () => startInlineRename(item))
}

// 执行删除（不带确认）
const doDelete = (items: any[]) => {
  if (items.length === 1) {
    confirmImmutableOperation(items[0], () => deletePaths([items[0].full]))
  } else {
    deletePaths(items.map((item: any) => item.full))
  }
}

// 删除文件（右键菜单/键盘 Delete 入口，带二次确认）
const deleteFiles = (items: any[]) => {
  if (items.length > 1) {
    const hasImmutable = items.some((item: any) => item.immutable)
    if (hasImmutable) {
      window.$message.warning($gettext('Some files are immutable and cannot be deleted'))
      return
    }
  }
  const content =
    items.length === 1
      ? $gettext('Are you sure you want to delete %{ name }?', { name: items[0].name })
      : $gettext('Are you sure you want to delete %{count} items?', { count: items.length })
  window.$dialog.warning({
    title: $gettext('Warning'),
    content,
    positiveText: $gettext('Yes'),
    negativeText: $gettext('No'),
    onPositiveClick: () => doDelete(items),
  })
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

// 菜单动作统一分发：右键菜单、桌面行内按钮、「更多」菜单与手机下拉共用，单项动作取 items 的第一项
const handleAction = (key: string, items: any[]) => {
  const item = items[0]
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
    case 'open':
      openFile(item)
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
      handleDownload(item)
      break
    case 'share':
      openShare(item)
      break
    case 'copy-path':
      copyPath(item)
      break
    case 'rename':
      openRename(item)
      break
    case 'terminal':
      openTerminal(item)
      break
    case 'favorite':
      toggleFavorite(item.full)
      break
    case 'protect':
      handleProtect(item, !isProtected(item))
      break
    case 'properties':
      openProperty(item)
      break
    case 'delete':
      deleteFiles(items)
      break
  }
}

// 右键菜单：多选时作用于全部选中项，否则作用于右键点中的那一项
const handleSelect = (key: string) => {
  handleAction(key, isMultiSelect.value ? getSelectedItems() : [selectedRow.value])
  onCloseDropdown()
}

const onCloseDropdown = () => {
  selectedRow.value = null
  showDropdown.value = false
}

// 行内删除按钮已经过气泡确认，直接执行
const handleDeleteClick = (item: any) => {
  doDelete([item])
}

// 桌面「更多」菜单，n-popselect 的选项用 value 而不是 key
const getMoreOptions = (item: any) =>
  pickItemOptions(item, moreKeys).map((o) => ({ label: o.label as string, value: o.key as string }))

// 下载文件
const handleDownload = (item: any) => {
  window.open('/api/file/download?path=' + encodeURIComponent(item.full))
}

// 手机操作列折叠成下拉：行内按钮在前，「更多」里的选项接在分隔线后，去掉与行内重复的项
const getRowMenuOptions = (item: any): DropdownOption[] => {
  const primary = ['open', item.dir ? 'compress' : 'download', 'rename', 'delete']
  return [
    ...pickItemOptions(item, primary),
    { type: 'divider', key: 'divider' },
    ...pickItemOptions(
      item,
      moreKeys.filter((key) => !primary.includes(key)),
    ),
  ]
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
      selected.value = [...selected.value, item.full]
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

// 键盘快捷键处理
const handleKeyDown = (event: KeyboardEvent) => {
  // 编辑器、任务队列等窗口展开时不处理快捷键
  if (hasOpenWindow.value) {
    return
  }

  // 弹窗、抽屉打开时不处理快捷键，否则弹窗内选中文本按 Ctrl+C 会被接管成标记复制文件
  if (document.querySelector('.n-modal-mask, .n-drawer-mask')) {
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
  useRequest(file.unCompress(unCompressModel.value.file, unCompressModel.value.path)).onSuccess(
    ({ data }) => {
      unCompressModal.value = false
      taskQueue.add(data)
    },
  )
}

const { loading, data, page, pageSize, refresh, reload, pagination } = usePagedTable(
  (page, pageSize) =>
    file.list(
      path.value,
      keyword.value,
      sub.value,
      sort.value,
      page,
      pageSize,
      !fileStore.showHidden,
    ),
  {
    initialPageSize: 100,
    pageSizes: [100, 200, 500, 1000],
  },
)

// selected 只保留当前列表中存在的项，避免删除/翻页/排序后残留导致表头复选框显示部分勾选
// 目录大小缓存同样只保留仍存在的项，避免删除后重建同名目录显示旧值
watch(data, (list) => {
  refreshProtected(list)
  const dataSet = new Set(list.map((item: any) => item.full))
  if (selected.value.length > 0) {
    const filtered = selected.value.filter((p: any) => dataSet.has(p))
    if (filtered.length !== selected.value.length) {
      selected.value = filtered
    }
  }
  for (const key of sizeCache.value.keys()) {
    if (!dataSet.has(key)) sizeCache.value.delete(key)
  }
  for (const key of sizeLoading.value.keys()) {
    if (!dataSet.has(key)) sizeLoading.value.delete(key)
  }
})

// 搜索事件处理函数
const handleFileSearch = () => {
  selected.value = []
  nextTick(() => {
    reload()
  })
  fileStore.pushHistory(props.tabId, path.value)
}

const addKeyListener = () => document.addEventListener('keydown', handleKeyDown)
const removeKeyListener = () => document.removeEventListener('keydown', handleKeyDown)

onActivated(addKeyListener)
onDeactivated(removeKeyListener)

onMounted(() => {
  refreshShares()

  // 首次加载由 usePagedTable 自己发起，这里只跟随路径变化，否则刷新页面会请求两次
  watch(path, () => {
    selected.value = []
    sizeCache.value.clear()
    sizeLoading.value.clear()
    nextTick(() => {
      reload()
    })
  })

  watch([sort, () => fileStore.showHidden], () => {
    nextTick(() => {
      reload()
    })
  })

  addKeyListener()
})

useBus('file:search', handleFileSearch)
useBus('file:refresh', refresh)
useBus('file:inline-create', startInlineCreate)

onUnmounted(() => {
  // 清理点击计时器
  if (clickTimer) {
    clearTimeout(clickTimer)
    clickTimer = null
  }

  removeKeyListener()
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
          'is-mobile': isMobile,
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
              <the-icon
                v-if="isProtected(item)"
                icon="mdi:shield-lock"
                :size="16"
                class="lock-icon text-success"
              />
              <the-icon v-else-if="item.immutable" icon="mdi:lock" :size="16" class="lock-icon" />
              <the-icon
                v-if="isShared(item)"
                icon="mdi:share-variant"
                :size="16"
                class="share-icon text-info"
              />
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
            <div class="list-col col-checkbox" @click.stop="toggleCheckbox(item)">
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
                v-if="isProtected(item)"
                icon="mdi:shield-lock"
                :size="14"
                class="text-success ml-1 shrink-0"
              />
              <the-icon
                v-else-if="item.immutable"
                icon="mdi:lock"
                :size="14"
                class="text-warning ml-1 shrink-0"
              />
              <the-icon
                v-if="isShared(item)"
                icon="mdi:share-variant"
                :size="14"
                class="text-info ml-1 shrink-0"
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
              <n-dropdown
                v-if="isMobile"
                trigger="click"
                :options="getRowMenuOptions(item)"
                @select="(key: string) => handleAction(key, [item])"
              >
                <n-button size="small" quaternary circle @click.stop>
                  <template #icon>
                    <the-icon icon="mdi:dots-vertical" :size="18" />
                  </template>
                </n-button>
              </n-dropdown>
              <n-flex v-else :size="4">
                <!-- 文件夹操作 -->
                <template v-if="item.dir">
                  <n-button
                    size="tiny"
                    type="success"
                    tertiary
                    @click.stop="handleAction('open', [item])"
                  >
                    {{ $gettext('Open') }}
                  </n-button>
                  <n-button
                    size="tiny"
                    type="primary"
                    tertiary
                    @click.stop="handleAction('compress', [item])"
                  >
                    {{ $gettext('Compress') }}
                  </n-button>
                </template>
                <!-- 文件操作 -->
                <template v-else>
                  <n-button
                    size="tiny"
                    type="primary"
                    tertiary
                    @click.stop="handleAction('open', [item])"
                  >
                    {{ openLabel(item) }}
                  </n-button>
                  <n-button
                    size="tiny"
                    type="info"
                    tertiary
                    @click.stop="handleAction('download', [item])"
                  >
                    {{ $gettext('Download') }}
                  </n-button>
                </template>
                <!-- 通用操作 -->
                <n-button
                  size="tiny"
                  type="warning"
                  tertiary
                  @click.stop="handleAction('rename', [item])"
                >
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
                  @update:value="(key: string) => handleAction(key, [item])"
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
      <n-pagination v-model:page="page" v-model:page-size="pageSize" v-bind="pagination" />
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

  <!-- 预览弹窗 -->
  <preview-modal v-model:show="previewModal" v-model:path="currentFile" />
  <!-- 解压弹窗 -->
  <app-modal
    v-model:show="unCompressModal"
    :title="$gettext('Uncompress - %{ file }', { file: unCompressModel.file })"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('Uncompress to')">
          <n-input v-model:value="unCompressModel.path" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="handleUnCompress">{{ $gettext('Uncompress') }}</n-button>
    </n-flex>
  </app-modal>
  <!-- 属性弹窗 -->
  <property-modal v-model:show="propertyModal" v-model:file-info="propertyFileInfo" />
  <!-- 分享弹窗 -->
  <share-modal v-model:show="shareModal" v-model:path="shareFile" @refresh="refreshShares" />
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

      // 勾选格吃掉行的左侧和上下 padding，点偏了也不会落到整行单选上
      .col-checkbox {
        align-self: stretch;
        box-sizing: content-box;
        margin: -6px 0 -6px -8px;
        padding-left: 8px;
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

// 手机只留勾选、名称、操作三列，名称与操作列放开宽度，避免容器横向溢出
// 不用 hidden 工具类：上面 scoped 的 display: flex 优先级更高
.file-container.is-mobile .list-col {
  &.col-size,
  &.col-mode,
  &.col-owner,
  &.col-modify {
    display: none;
  }

  &.col-name {
    min-width: 0;
  }

  &.col-actions {
    width: auto;
    justify-content: flex-end;
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

.share-icon {
  position: absolute;
  bottom: 0;
  left: 0;
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

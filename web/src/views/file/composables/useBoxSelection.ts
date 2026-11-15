// 文件列表的鼠标框选：在容器空白处按下并拖动，选中与选框相交的 .file-item（按 data-path 识别）
export function useBoxSelection(
  container: Ref<HTMLElement | null>,
  selected: Ref<readonly string[], string[]>,
  selectedSet: Readonly<Ref<Set<string>>>,
) {
  const isSelecting = ref(false)
  const selectionBoxEl = ref<HTMLElement | null>(null)

  // 非响应式坐标 — 避免每次 mousemove 触发 Vue 重渲染
  let selStartX = 0
  let selStartY = 0
  let selEndX = 0
  let selEndY = 0

  // 缓存 item 位置，框选期间不再每帧 querySelectorAll + getBoundingClientRect
  let cachedItemRects: {
    path: string
    left: number
    top: number
    right: number
    bottom: number
  }[] = []
  let cachedContainerRect: DOMRect | null = null

  // 按住 Ctrl/Cmd 开始框选时已有的选中项，框内结果追加在它后面
  let baseSelected: readonly string[] = []

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

  // 根据选择框更新选中的项目
  const updateSelectionFromBox = () => {
    if (cachedItemRects.length === 0) return

    const left = Math.min(selStartX, selEndX)
    const top = Math.min(selStartY, selEndY)
    const right = Math.max(selStartX, selEndX)
    const bottom = Math.max(selStartY, selEndY)

    const baseSet = new Set(baseSelected)
    const newSelected = [...baseSelected]
    for (const item of cachedItemRects) {
      if (
        !baseSet.has(item.path) &&
        item.right >= left &&
        item.left <= right &&
        item.bottom >= top &&
        item.top <= bottom
      ) {
        newSelected.push(item.path)
      }
    }

    // 只在选中集合真正变化时才更新
    const currentSet = selectedSet.value
    if (newSelected.length !== currentSet.size || newSelected.some((v) => !currentSet.has(v))) {
      selected.value = newSelected
    }
  }

  // 框选开始
  const onSelectionStart = (event: MouseEvent) => {
    // 只响应左键，且不在项目上
    if (event.button !== 0) return
    const target = event.target as HTMLElement
    if (target.closest('.file-item')) return
    // 列表视图表头不触发框选
    if (target.closest('.list-header')) return

    const el = container.value
    if (!el) return

    isSelecting.value = true
    const rect = el.getBoundingClientRect()
    cachedContainerRect = rect
    selStartX = event.clientX - rect.left + el.scrollLeft
    selStartY = event.clientY - rect.top + el.scrollTop
    selEndX = selStartX
    selEndY = selStartY

    // 缓存所有 item 的位置，框选期间不再重复查询 DOM
    const items = el.querySelectorAll('.file-item')
    const scrollLeft = el.scrollLeft
    const scrollTop = el.scrollTop
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

    // 没有按住 Ctrl/Cmd 就从空开始
    baseSelected = event.ctrlKey || event.metaKey ? [...selected.value] : []
    if (baseSelected.length === 0) {
      selected.value = []
    }
  }

  // 框选移动
  let selectionRafId: number | null = null
  const onSelectionMove = (event: MouseEvent) => {
    if (!isSelecting.value) return

    const el = container.value
    if (!el || !cachedContainerRect) return

    selEndX = event.clientX - cachedContainerRect.left + el.scrollLeft
    selEndY = event.clientY - cachedContainerRect.top + el.scrollTop

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

  // 拖动可能移出容器，移动与松开监听挂在 document 上
  const addListeners = () => {
    document.addEventListener('mousemove', onSelectionMove)
    document.addEventListener('mouseup', onSelectionEnd)
  }
  const removeListeners = () => {
    document.removeEventListener('mousemove', onSelectionMove)
    document.removeEventListener('mouseup', onSelectionEnd)
  }
  onMounted(addListeners)
  onActivated(addListeners)
  onDeactivated(removeListeners)
  onUnmounted(removeListeners)

  return { isSelecting, selectionBoxEl, onSelectionStart }
}

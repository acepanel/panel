<script setup lang="ts">
import { useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const themeVars = useThemeVars()

const props = withDefaults(
  defineProps<{
    title?: string
    minWidth?: number
    minHeight?: number
    defaultWidth?: number
    defaultHeight?: number
    beforeClose?: () => Promise<boolean> | boolean // 关闭前的确认回调，返回 true 继续关闭，false 取消关闭
    closeOnOverlay?: boolean // 点击遮罩层是否最小化窗口，默认 true
  }>(),
  {
    title: '',
    minWidth: 400,
    minHeight: 300,
    defaultWidth: 800,
    defaultHeight: 600,
    closeOnOverlay: true
  }
)

const show = defineModel<boolean>('show', { default: false })
const minimized = defineModel<boolean>('minimized', { default: false })

// 窗口状态
const isMaximized = ref(false)

// 窗口位置和大小
const position = ref({ x: 0, y: 0 })
const size = ref({ width: props.defaultWidth, height: props.defaultHeight })

// 最大化前的状态（用于恢复）
const beforeMaximize = ref({ x: 0, y: 0, width: 0, height: 0 })

// 拖拽状态
const isDragging = ref(false)
const dragStart = ref({ x: 0, y: 0 })

// 调整大小状态
const isResizing = ref(false)
const resizeDirection = ref('')
const resizeStart = ref({ x: 0, y: 0, width: 0, height: 0, posX: 0, posY: 0 })

// 窗口样式
const windowStyle = computed(() => ({
  left: position.value.x + 'px',
  top: position.value.y + 'px',
  width: size.value.width + 'px',
  height: size.value.height + 'px',
  background: themeVars.value.cardColor,
  '--border-color': themeVars.value.borderColor,
  '--text-color-1': themeVars.value.textColor1,
  '--text-color-2': themeVars.value.textColor2,
  '--text-color-3': themeVars.value.textColor3,
  '--hover-color': themeVars.value.buttonColor2Hover,
  '--primary-color': themeVars.value.primaryColor,
  '--border-radius': themeVars.value.borderRadius
}))

// 初始化窗口位置（居中）
function initPosition() {
  position.value = {
    x: (window.innerWidth - size.value.width) / 2,
    y: (window.innerHeight - size.value.height) / 2
  }
}

// 开始拖拽
function startDrag(e: MouseEvent) {
  if (isMaximized.value) return
  isDragging.value = true
  dragStart.value = {
    x: e.clientX - position.value.x,
    y: e.clientY - position.value.y
  }
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
}

function onDrag(e: MouseEvent) {
  if (!isDragging.value) return
  position.value = {
    x: Math.max(0, Math.min(window.innerWidth - size.value.width, e.clientX - dragStart.value.x)),
    y: Math.max(0, Math.min(window.innerHeight - size.value.height, e.clientY - dragStart.value.y))
  }
}

function stopDrag() {
  isDragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
}

// 开始调整大小
function startResize(e: MouseEvent, direction: string) {
  if (isMaximized.value) return
  e.preventDefault()
  e.stopPropagation()
  isResizing.value = true
  resizeDirection.value = direction
  resizeStart.value = {
    x: e.clientX,
    y: e.clientY,
    width: size.value.width,
    height: size.value.height,
    posX: position.value.x,
    posY: position.value.y
  }
  document.addEventListener('mousemove', onResize)
  document.addEventListener('mouseup', stopResize)
}

function onResize(e: MouseEvent) {
  if (!isResizing.value) return

  const deltaX = e.clientX - resizeStart.value.x
  const deltaY = e.clientY - resizeStart.value.y
  const dir = resizeDirection.value

  let newWidth = resizeStart.value.width
  let newHeight = resizeStart.value.height
  let newX = resizeStart.value.posX
  let newY = resizeStart.value.posY

  // 右边
  if (dir.includes('e')) {
    newWidth = Math.max(props.minWidth, resizeStart.value.width + deltaX)
  }
  // 左边
  if (dir.includes('w')) {
    const maxDelta = resizeStart.value.width - props.minWidth
    const actualDelta = Math.min(deltaX, maxDelta)
    newWidth = resizeStart.value.width - actualDelta
    newX = resizeStart.value.posX + actualDelta
  }
  // 下边
  if (dir.includes('s')) {
    newHeight = Math.max(props.minHeight, resizeStart.value.height + deltaY)
  }
  // 上边
  if (dir.includes('n')) {
    const maxDelta = resizeStart.value.height - props.minHeight
    const actualDelta = Math.min(deltaY, maxDelta)
    newHeight = resizeStart.value.height - actualDelta
    newY = resizeStart.value.posY + actualDelta
  }

  // 限制在窗口内
  newX = Math.max(0, newX)
  newY = Math.max(0, newY)
  newWidth = Math.min(newWidth, window.innerWidth - newX)
  newHeight = Math.min(newHeight, window.innerHeight - newY)

  size.value = { width: newWidth, height: newHeight }
  position.value = { x: newX, y: newY }
}

function stopResize() {
  isResizing.value = false
  document.removeEventListener('mousemove', onResize)
  document.removeEventListener('mouseup', stopResize)
}

// 最大化/还原
function toggleMaximize() {
  if (minimized.value) {
    minimized.value = false
    return
  }

  if (isMaximized.value) {
    // 还原
    position.value = { x: beforeMaximize.value.x, y: beforeMaximize.value.y }
    size.value = { width: beforeMaximize.value.width, height: beforeMaximize.value.height }
    isMaximized.value = false
  } else {
    // 最大化
    beforeMaximize.value = {
      x: position.value.x,
      y: position.value.y,
      width: size.value.width,
      height: size.value.height
    }
    position.value = { x: 0, y: 0 }
    size.value = { width: window.innerWidth, height: window.innerHeight }
    isMaximized.value = true
  }
}

// 最小化
function minimize() {
  minimized.value = true
}

// 从最小化恢复
function restore() {
  minimized.value = false
}

// 处理遮罩层点击
function handleOverlayClick() {
  if (props.closeOnOverlay) {
    minimize()
  }
}

// 关闭
async function close() {
  // 如果提供了 beforeClose 回调，先执行它
  if (props.beforeClose) {
    const result = await props.beforeClose()
    if (!result) return // 如果返回 false，取消关闭
  }
  show.value = false
}

// 双击标题栏最大化/还原
function onTitleDoubleClick() {
  toggleMaximize()
}

// 监听显示状态
watch(show, (newShow) => {
  if (newShow) {
    minimized.value = false
    isMaximized.value = false
    size.value = { width: props.defaultWidth, height: props.defaultHeight }
    initPosition()
  }
})

// 监听窗口大小变化
function handleWindowResize() {
  if (isMaximized.value) {
    size.value = { width: window.innerWidth, height: window.innerHeight }
  }
}

onMounted(() => {
  window.addEventListener('resize', handleWindowResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleWindowResize)
})
</script>

<template>
  <Teleport to="body">
    <!-- 遮罩层 -->
    <Transition name="fade">
      <div v-if="show && !minimized" class="draggable-window-overlay" @click="handleOverlayClick" />
    </Transition>

    <!-- 主窗口 -->
    <Transition name="window">
      <div
        v-if="show && !minimized"
        ref="windowRef"
        class="draggable-window"
        :class="{ maximized: isMaximized, dragging: isDragging, resizing: isResizing }"
        :style="windowStyle"
      >
        <!-- 标题栏 -->
        <div class="draggable-window-header" @mousedown="startDrag" @dblclick="onTitleDoubleClick">
          <span class="draggable-window-title">{{ title }}</span>
          <div class="draggable-window-controls">
            <button class="control-btn minimize" @click.stop="minimize" :title="$gettext('Minimize')">
              <i-mdi-window-minimize />
            </button>
            <button
              class="control-btn maximize"
              @click.stop="toggleMaximize"
              :title="isMaximized ? $gettext('Restore') : $gettext('Maximize')"
            >
              <i-mdi-window-restore v-if="isMaximized" />
              <i-mdi-window-maximize v-else />
            </button>
            <button class="control-btn close" @click.stop="close" :title="$gettext('Close')">
              <i-mdi-close />
            </button>
          </div>
        </div>

        <!-- 内容区域 -->
        <div class="draggable-window-content">
          <slot />
        </div>

        <!-- 调整大小的边框 -->
        <template v-if="!isMaximized">
          <div class="resize-handle n" @mousedown="startResize($event, 'n')" />
          <div class="resize-handle s" @mousedown="startResize($event, 's')" />
          <div class="resize-handle e" @mousedown="startResize($event, 'e')" />
          <div class="resize-handle w" @mousedown="startResize($event, 'w')" />
          <div class="resize-handle ne" @mousedown="startResize($event, 'ne')" />
          <div class="resize-handle nw" @mousedown="startResize($event, 'nw')" />
          <div class="resize-handle se" @mousedown="startResize($event, 'se')" />
          <div class="resize-handle sw" @mousedown="startResize($event, 'sw')" />
        </template>
      </div>
    </Transition>

    <!-- 最小化后的图标 -->
    <Transition name="minimize">
      <div
        v-if="show && minimized"
        class="draggable-window-minimized"
        :style="{
          background: themeVars.cardColor,
          color: themeVars.textColor1,
          '--border-radius': themeVars.borderRadius
        }"
        @click="restore"
      >
        <i-mdi-file-document-outline />
        <span>{{ title }}</span>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped lang="scss">
.draggable-window-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 1999;
}

.draggable-window {
  position: fixed;
  z-index: 2000;
  display: flex;
  flex-direction: column;
  border-radius: var(--border-radius);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
  overflow: hidden;

  &.maximized {
    border-radius: 0;
  }

  &.dragging,
  &.resizing {
    user-select: none;
  }
}

.draggable-window-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 40px;
  padding: 0 8px 0 16px;
  border-bottom: 1px solid var(--border-color);
  cursor: move;
  flex-shrink: 0;
  user-select: none;

  .maximized & {
    cursor: default;
  }
}

.draggable-window-title {
  font-weight: 500;
  font-size: 14px;
  color: var(--text-color-1);
}

.draggable-window-controls {
  display: flex;
  gap: 4px;
}

.control-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 28px;
  border: none;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-color-2);
  transition: all 0.2s;

  &:hover {
    background: var(--hover-color);
  }

  &.close:hover {
    background: #e81123;
    color: white;
  }
}

.draggable-window-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

// 调整大小的手柄
.resize-handle {
  position: absolute;

  &.n,
  &.s {
    left: 8px;
    right: 8px;
    height: 6px;
    cursor: ns-resize;
  }

  &.e,
  &.w {
    top: 8px;
    bottom: 8px;
    width: 6px;
    cursor: ew-resize;
  }

  &.n {
    top: -3px;
  }
  &.s {
    bottom: -3px;
  }
  &.e {
    right: -3px;
  }
  &.w {
    left: -3px;
  }

  &.ne,
  &.nw,
  &.se,
  &.sw {
    width: 12px;
    height: 12px;
  }

  &.ne {
    top: -3px;
    right: -3px;
    cursor: nesw-resize;
  }
  &.nw {
    top: -3px;
    left: -3px;
    cursor: nwse-resize;
  }
  &.se {
    bottom: -3px;
    right: -3px;
    cursor: nwse-resize;
  }
  &.sw {
    bottom: -3px;
    left: -3px;
    cursor: nesw-resize;
  }
}

// 最小化后的图标
.draggable-window-minimized {
  position: fixed;
  bottom: 16px;
  right: 16px;
  z-index: 2000;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: var(--border-radius);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
  cursor: pointer;
  transition: all 0.2s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.25);
  }

  span {
    font-size: 13px;
  }
}

// 动画
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.window-enter-active,
.window-leave-active {
  transition: all 0.2s;
}

.window-enter-from,
.window-leave-to {
  opacity: 0;
  transform: scale(0.95);
}

.minimize-enter-active,
.minimize-leave-active {
  transition: all 0.2s;
}

.minimize-enter-from,
.minimize-leave-to {
  opacity: 0;
  transform: translateY(20px);
}
</style>

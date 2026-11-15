<script setup lang="ts">
import type * as Monaco from 'monaco-editor'
import { NButton, NFlex, useThemeVars } from 'naive-ui'
import { h } from 'vue'
import { useGettext } from 'vue3-gettext'

import { useEditorOps } from '@/components/file-editor/composables/useEditorOps'
import type { EditorSettings, EditorTab } from '@/stores'
import { useEditorStore, useThemeStore } from '@/stores'
import { getMonaco } from '@/utils/monaco'
import { getEditorTheme } from '@/utils/monaco/theme'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
const { saveTab } = useEditorOps()
const themeStore = useThemeStore()
const themeVars = useThemeVars()

const props = defineProps<{
  readOnly?: boolean
}>()

const containerRef = ref<HTMLDivElement>()
const editorRef = shallowRef<Monaco.editor.IStandaloneCodeEditor>()
const monacoRef = shallowRef<typeof Monaco>()
const editorReady = ref(false)
const tabsContainerRef = ref<HTMLDivElement>()

// 每个 tab 拥有独立的 Monaco model，切换 tab 通过 setModel 而非 setValue
// 消除单一 model 时因异步事件与 activeTabPath 错位导致的内容/回写污染
const models = new Map<string, Monaco.editor.ITextModel>()
const viewStates = new Map<string, Monaco.editor.ICodeEditorViewState | null>()
let currentPath: string | null = null

// 标签页滚轮横向滚动
function handleTabsWheel(e: WheelEvent) {
  if (tabsContainerRef.value) {
    tabsContainerRef.value.scrollLeft += e.deltaY
  }
}

// 编辑器设置到 Monaco 选项的映射
function toMonacoOptions(
  settings: EditorSettings,
): Monaco.editor.IEditorOptions & Monaco.editor.IGlobalEditorOptions {
  return {
    // Basic settings
    tabSize: settings.tabSize,
    insertSpaces: settings.insertSpaces,
    wordWrap: settings.wordWrap,
    fontSize: settings.fontSize,
    minimap: { enabled: settings.minimap },
    // Display settings
    lineNumbers: settings.lineNumbers,
    renderWhitespace: settings.renderWhitespace,
    bracketPairColorization: { enabled: settings.bracketPairColorization },
    guides: {
      indentation: settings.guides,
      bracketPairs: settings.guides,
    },
    folding: settings.folding,
    // Cursor settings
    cursorStyle: settings.cursorStyle,
    cursorBlinking: settings.cursorBlinking,
    smoothScrolling: settings.smoothScrolling,
    // Behavior settings
    mouseWheelZoom: settings.mouseWheelZoom,
    formatOnPaste: settings.formatOnPaste,
    formatOnType: settings.formatOnType,
  }
}

// 获取或创建 tab 对应的 model
function ensureModel(tab: EditorTab): Monaco.editor.ITextModel {
  const monaco = monacoRef.value!
  const existing = models.get(tab.path)
  if (existing && !existing.isDisposed()) return existing

  const model = monaco.editor.createModel(tab.content, tab.language)
  const eol =
    tab.lineEnding === 'CRLF'
      ? monaco.editor.EndOfLineSequence.CRLF
      : monaco.editor.EndOfLineSequence.LF
  model.setEOL(eol)

  const boundPath = tab.path
  model.onDidChangeContent(() => {
    if (model.isDisposed()) return
    editorStore.updateContent(boundPath, model.getValue())
  })

  models.set(tab.path, model)
  return model
}

// 释放 model 及其视图状态
function disposeModel(path: string) {
  const model = models.get(path)
  if (model && !model.isDisposed()) model.dispose()
  models.delete(path)
  viewStates.delete(path)
  if (currentPath === path) currentPath = null
}

// 切换 editor 显示的 model，同时保存/恢复视图状态
function applyActiveTab() {
  if (!editorRef.value || !monacoRef.value) return

  if (currentPath) {
    viewStates.set(currentPath, editorRef.value.saveViewState())
  }

  const tab = editorStore.activeTab
  if (!tab) {
    editorRef.value.setModel(null)
    currentPath = null
    return
  }

  const model = ensureModel(tab)
  if (editorRef.value.getModel() !== model) {
    editorRef.value.setModel(model)
  }
  currentPath = tab.path

  const state = viewStates.get(tab.path)
  if (state) editorRef.value.restoreViewState(state)

  monacoRef.value.editor.setTheme(getEditorTheme(tab.language, themeStore.darkMode))
}

// 初始化编辑器
async function initEditor() {
  const monaco = await getMonaco(themeStore.locale)
  // 加载期间组件可能已卸载
  if (!containerRef.value) return
  monacoRef.value = monaco

  editorRef.value = monaco.editor.create(containerRef.value, {
    theme: getEditorTheme(editorStore.activeTab?.language ?? '', themeStore.darkMode),
    readOnly: props.readOnly,
    automaticLayout: true,
    ...toMonacoOptions(editorStore.settings),
  })

  // 监听光标位置变化
  editorRef.value.onDidChangeCursorPosition((e) => {
    if (!editorStore.activeTab) return
    editorStore.updateCursor(editorStore.activeTab.path, e.position.lineNumber, e.position.column)
  })

  editorReady.value = true
  applyActiveTab()
}

// 关闭标签页
function handleCloseTab(path: string, e: MouseEvent) {
  e.stopPropagation()
  const tab = editorStore.tabs.find((t) => t.path === path)
  if (tab?.modified) {
    const d = window.$dialog.warning({
      title: $gettext('Unsaved Changes'),
      content: $gettext('This file has unsaved changes. Are you sure you want to close it?'),
      closable: false,
      maskClosable: false,
      action: () =>
        h(NFlex, { justify: 'end' }, () => [
          h(
            NButton,
            {
              onClick: () => {
                d.destroy() // 返回，不关闭标签页
              },
            },
            () => $gettext('Go Back'),
          ),
          h(
            NButton,
            {
              type: 'warning',
              onClick: () => {
                d.destroy()
                editorStore.closeTab(path) // 放弃更改，关闭标签页
              },
            },
            () => $gettext('Discard'),
          ),
          h(
            NButton,
            {
              type: 'primary',
              onClick: async () => {
                if (await saveTab(tab.path)) {
                  window.$message.success($gettext('Saved successfully'))
                  d.destroy()
                  editorStore.closeTab(path) // 保存成功，关闭标签页
                } else {
                  d.destroy() // 保存失败，不关闭标签页
                }
              },
            },
            () => $gettext('Save'),
          ),
        ]),
    })
  } else {
    editorStore.closeTab(path)
  }
}

// 切换标签页
function handleSwitchTab(path: string) {
  editorStore.switchTab(path)
}

// 拖拽排序
const dragIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)

function handleDragStart(e: DragEvent, index: number) {
  dragIndex.value = index
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
  }
}

function handleDragOver(e: DragEvent, index: number) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
  // 只在值变化时更新，避免高频触发导致闪烁
  if (dragOverIndex.value !== index) {
    dragOverIndex.value = index
  }
}

function handleDragLeave(e: DragEvent) {
  // 检查是否离开了整个 tabs 容器，而不是在标签页之间移动
  const relatedTarget = e.relatedTarget as HTMLElement | null
  if (!relatedTarget || !tabsContainerRef.value?.contains(relatedTarget)) {
    dragOverIndex.value = null
  }
}

function handleDrop(e: DragEvent, toIndex: number) {
  e.preventDefault()
  if (dragIndex.value !== null && dragIndex.value !== toIndex) {
    editorStore.reorderTabs(dragIndex.value, toIndex)
  }
  dragIndex.value = null
  dragOverIndex.value = null
}

function handleDragEnd() {
  dragIndex.value = null
  dragOverIndex.value = null
}

// 尾部放置区域的拖拽处理
function handleDragOverEnd(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
  const endIndex = editorStore.tabs.length
  if (dragOverIndex.value !== endIndex) {
    dragOverIndex.value = endIndex
  }
}

function handleDropEnd(e: DragEvent) {
  e.preventDefault()
  if (dragIndex.value !== null && dragIndex.value !== editorStore.tabs.length - 1) {
    // 移动到最后
    editorStore.reorderTabs(dragIndex.value, editorStore.tabs.length - 1)
  }
  dragIndex.value = null
  dragOverIndex.value = null
}

// 右键菜单
const contextMenuOptions = computed(() => [
  {
    label: $gettext('Close'),
    key: 'close',
  },
  {
    label: $gettext('Close Others'),
    key: 'closeOthers',
  },
  {
    label: $gettext('Close All'),
    key: 'closeAll',
  },
  {
    label: $gettext('Close Saved'),
    key: 'closeSaved',
  },
])

const contextMenuX = ref(0)
const contextMenuY = ref(0)
const showContextMenu = ref(false)
const contextMenuPath = ref('')

function handleContextMenu(e: MouseEvent, path: string) {
  e.preventDefault()
  contextMenuPath.value = path
  contextMenuX.value = e.clientX
  contextMenuY.value = e.clientY
  showContextMenu.value = true
}

function handleContextMenuSelect(key: string) {
  showContextMenu.value = false
  switch (key) {
    case 'close':
      handleCloseTab(contextMenuPath.value, new MouseEvent('click'))
      break
    case 'closeOthers':
      editorStore.closeOtherTabs(contextMenuPath.value)
      break
    case 'closeAll':
      editorStore.closeAllTabs()
      break
    case 'closeSaved':
      editorStore.closeSavedTabs()
      break
  }
}

function handleClickOutside() {
  showContextMenu.value = false
}

// 监听当前标签页变化
watch(
  () => editorStore.activeTabPath,
  () => {
    if (editorReady.value) applyActiveTab()
  },
)

// 监听语言变化（用户手动切换语言时更新对应 model）
watch(
  () => editorStore.activeTab?.language,
  (newLanguage) => {
    if (!monacoRef.value || !newLanguage) return
    const tab = editorStore.activeTab
    if (!tab) return
    const model = models.get(tab.path)
    if (!model) return
    monacoRef.value.editor.setModelLanguage(model, newLanguage)
    if (editorRef.value?.getModel() === model) {
      monacoRef.value.editor.setTheme(getEditorTheme(newLanguage, themeStore.darkMode))
    }
  },
)

// 监听行分隔符变化（用户手动切换行分隔符时更新对应 model）
watch(
  () => editorStore.activeTab?.lineEnding,
  (newLineEnding) => {
    if (!monacoRef.value || !newLineEnding) return
    const tab = editorStore.activeTab
    if (!tab) return
    const model = models.get(tab.path)
    if (!model) return
    const eol =
      newLineEnding === 'CRLF'
        ? monacoRef.value.editor.EndOfLineSequence.CRLF
        : monacoRef.value.editor.EndOfLineSequence.LF
    model.setEOL(eol)
  },
)

// 监听当前标签页内容变化（reloadFile 等外部更新时同步到 model）
watch(
  () => editorStore.activeTab?.content,
  (newContent) => {
    if (newContent === undefined) return
    const tab = editorStore.activeTab
    if (!tab) return
    const model = models.get(tab.path)
    if (model && model.getValue() !== newContent) {
      model.setValue(newContent)
    }
  },
)

// 监听 tabs 列表变化
watch(
  () => editorStore.tabs.map((t) => t.path).join('|'),
  () => {
    const active = new Set(editorStore.tabs.map((t) => t.path))
    for (const path of Array.from(models.keys())) {
      if (!active.has(path)) disposeModel(path)
    }
  },
)

// 监听主题变化
watch(
  () => themeStore.darkMode,
  (dark) => {
    if (!monacoRef.value || !editorStore.activeTab) return
    monacoRef.value.editor.setTheme(getEditorTheme(editorStore.activeTab.language, dark))
  },
)

// 监听编辑器设置变化
watch(
  () => editorStore.settings,
  (settings) => {
    editorRef.value?.updateOptions(toMonacoOptions(settings))
  },
  { deep: true },
)

onMounted(() => {
  initEditor()
})

onBeforeUnmount(() => {
  editorRef.value?.dispose()
  for (const model of models.values()) {
    if (!model.isDisposed()) model.dispose()
  }
  models.clear()
  viewStates.clear()
  currentPath = null
})

// 暴露方法
defineExpose({
  getEditor: () => editorRef.value,
  focus: () => editorRef.value?.focus(),
})
</script>

<template>
  <div class="editor-pane">
    <!-- 标签页栏 -->
    <div class="tabs-bar" v-if="editorStore.tabs.length > 0">
      <div ref="tabsContainerRef" class="tabs-container" @wheel.prevent="handleTabsWheel">
        <div
          v-for="(tab, index) in editorStore.tabs"
          :key="tab.path"
          class="tab-item"
          :class="{
            active: tab.path === editorStore.activeTabPath,
            dragging: dragIndex === index,
            'drag-over': dragOverIndex === index && dragIndex !== index,
          }"
          draggable="true"
          @click="handleSwitchTab(tab.path)"
          @contextmenu="handleContextMenu($event, tab.path)"
          @dragstart="handleDragStart($event, index)"
          @dragover="handleDragOver($event, index)"
          @dragleave="handleDragLeave($event)"
          @drop="handleDrop($event, index)"
          @dragend="handleDragEnd"
        >
          <span class="tab-name" :class="{ modified: tab.modified }">
            {{ tab.name }}
            <span v-if="tab.modified" class="modified-dot">●</span>
          </span>
          <n-button
            quaternary
            size="tiny"
            class="close-btn"
            @click="handleCloseTab(tab.path, $event)"
          >
            <template #icon>
              <i-mdi-close />
            </template>
          </n-button>
        </div>
        <!-- 尾部放置区域，用于拖拽到最后 -->
        <div
          v-if="dragIndex !== null"
          class="tab-drop-end"
          :class="{ 'drag-over': dragOverIndex === editorStore.tabs.length }"
          @dragover="handleDragOverEnd($event)"
          @dragleave="handleDragLeave($event)"
          @drop="handleDropEnd($event)"
        />
      </div>
    </div>

    <!-- 编辑器容器 -->
    <div class="editor-container">
      <div v-if="editorStore.tabs.length === 0" class="empty-state">
        <i-mdi-file-document-outline class="empty-icon" />
        <p>{{ $gettext('Select a file to edit') }}</p>
      </div>
      <div v-show="editorStore.tabs.length > 0" ref="containerRef" class="monaco-container" />
      <div v-if="editorStore.activeTab?.loading" class="loading-overlay">
        <n-spin size="medium" />
      </div>
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
      @clickoutside="handleClickOutside"
    />
  </div>
</template>

<style scoped lang="scss">
.editor-pane {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  min-width: 0; /* 允许在 flex 布局中收缩 */
}

.tabs-bar {
  flex-shrink: 0;
  border-bottom: 1px solid v-bind('themeVars.borderColor');
  background: v-bind('themeVars.cardColor');
  overflow: hidden;
}

.tabs-container {
  display: flex;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none; /* Firefox */

  &::-webkit-scrollbar {
    display: none; /* Chrome, Safari, Edge */
  }
}

.tab-item {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
  cursor: pointer;
  border-right: 1px solid v-bind('themeVars.borderColor');
  white-space: nowrap;
  transition:
    background-color 0.2s,
    opacity 0.2s;
  position: relative;
  user-select: none;

  &:hover {
    background: v-bind('themeVars.buttonColor2Hover');
  }

  &.active {
    background: v-bind('themeVars.buttonColor2Hover');
    font-weight: 500;

    &::after {
      content: '';
      position: absolute;
      bottom: 0;
      left: 0;
      right: 0;
      height: 2px;
      background: v-bind('themeVars.primaryColor');
    }
  }

  &.dragging {
    opacity: 0.5;
  }

  &.drag-over {
    border-left: 2px solid v-bind('themeVars.primaryColor');
  }
}

.tab-drop-end {
  width: 20px;
  flex-shrink: 0;

  &.drag-over {
    border-left: 2px solid v-bind('themeVars.primaryColor');
  }
}

.tab-name {
  font-size: 13px;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;

  &.modified {
    font-style: italic;
  }
}

.modified-dot {
  color: v-bind('themeVars.warningColor');
  margin-left: 4px;
}

.close-btn {
  opacity: 0.6;
  padding: 2px;

  &:hover {
    opacity: 1;
  }
}

.editor-container {
  flex: 1;
  position: relative;
  overflow: visible; /* 允许 tooltip 溢出显示 */
}

.monaco-container {
  width: 100%;
  height: 100%;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: v-bind('themeVars.textColor3');

  .empty-icon {
    font-size: 64px;
    margin-bottom: 16px;
    opacity: 0.5;
  }

  p {
    font-size: 14px;
  }
}

.loading-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.3);
  z-index: 10;
}
</style>

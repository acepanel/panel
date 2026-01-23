<script setup lang="ts">
import { useEditorStore, useThemeStore } from '@/store'
import { languageByPath } from '@/utils/file'
import { getMonaco } from '@/utils/monaco'
import type * as Monaco from 'monaco-editor'
import { useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
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

// 标签页滚轮横向滚动
function handleTabsWheel(e: WheelEvent) {
  if (tabsContainerRef.value) {
    tabsContainerRef.value.scrollLeft += e.deltaY
  }
}

// 获取编辑器主题（nginx 特殊处理）
function getEditorTheme(language: string) {
  return (language === 'nginx' ? 'nginx-theme' : 'vs') + (themeStore.darkMode ? '-dark' : '')
}

// 初始化编辑器
async function initEditor() {
  if (!containerRef.value) return

  const monaco = await getMonaco(themeStore.locale)
  monacoRef.value = monaco

  const settings = editorStore.settings
  editorRef.value = monaco.editor.create(containerRef.value, {
    value: '',
    language: 'plaintext',
    theme: 'vs' + (themeStore.darkMode ? '-dark' : ''),
    readOnly: props.readOnly,
    automaticLayout: true,
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
      bracketPairs: settings.guides
    },
    folding: settings.folding,
    // Cursor settings
    cursorStyle: settings.cursorStyle,
    cursorBlinking: settings.cursorBlinking,
    smoothScrolling: settings.smoothScrolling,
    // Behavior settings
    mouseWheelZoom: settings.mouseWheelZoom,
    formatOnPaste: settings.formatOnPaste,
    formatOnType: settings.formatOnType
  })

  // 监听内容变化
  editorRef.value.onDidChangeModelContent(() => {
    if (!editorStore.activeTab) return
    const newValue = editorRef.value?.getValue() ?? ''
    editorStore.updateContent(editorStore.activeTab.path, newValue)
  })

  // 监听光标位置变化
  editorRef.value.onDidChangeCursorPosition((e) => {
    if (!editorStore.activeTab) return
    editorStore.updateCursor(editorStore.activeTab.path, e.position.lineNumber, e.position.column)
  })

  editorReady.value = true
  updateEditorContent()
}

// 更新编辑器内容
function updateEditorContent() {
  if (!editorRef.value || !monacoRef.value) return

  const tab = editorStore.activeTab
  if (!tab) {
    editorRef.value.setValue('')
    return
  }

  // 更新内容
  const currentValue = editorRef.value.getValue()
  if (currentValue !== tab.content) {
    editorRef.value.setValue(tab.content)
  }

  // 更新语言
  const model = editorRef.value.getModel()
  if (model) {
    const language = languageByPath(tab.path)
    monacoRef.value.editor.setModelLanguage(model, language)
    monacoRef.value.editor.setTheme(getEditorTheme(language))
  }
}

// 关闭标签页
function handleCloseTab(path: string, e: MouseEvent) {
  e.stopPropagation()
  const tab = editorStore.tabs.find((t) => t.path === path)
  if (tab?.modified) {
    window.$dialog.warning({
      title: $gettext('Unsaved Changes'),
      content: $gettext('This file has unsaved changes. Are you sure you want to close it?'),
      positiveText: $gettext('Close'),
      negativeText: $gettext('Cancel'),
      onPositiveClick: () => {
        editorStore.closeTab(path)
      }
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
    key: 'close'
  },
  {
    label: $gettext('Close Others'),
    key: 'closeOthers'
  },
  {
    label: $gettext('Close All'),
    key: 'closeAll'
  },
  {
    label: $gettext('Close Saved'),
    key: 'closeSaved'
  }
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
    if (editorReady.value) {
      updateEditorContent()
    }
  }
)

// 监听语言变化（用户手动切换语言时更新 Monaco 高亮）
watch(
  () => editorStore.activeTab?.language,
  (newLanguage) => {
    if (!editorRef.value || !monacoRef.value || !newLanguage) return
    const model = editorRef.value.getModel()
    if (model) {
      monacoRef.value.editor.setModelLanguage(model, newLanguage)
      monacoRef.value.editor.setTheme(getEditorTheme(newLanguage))
    }
  }
)

// 监听当前标签页内容变化（外部更新）
watch(
  () => editorStore.activeTab?.content,
  (newContent) => {
    if (!editorRef.value || !editorStore.activeTab) return
    const currentValue = editorRef.value.getValue()
    if (newContent !== undefined && currentValue !== newContent) {
      editorRef.value.setValue(newContent)
    }
  }
)

// 监听主题变化
watch(
  () => themeStore.darkMode,
  () => {
    if (!monacoRef.value || !editorStore.activeTab) return
    const language = languageByPath(editorStore.activeTab.path)
    monacoRef.value.editor.setTheme(getEditorTheme(language))
  }
)

// 监听编辑器设置变化
watch(
  () => editorStore.settings,
  (settings) => {
    if (!editorRef.value) return
    editorRef.value.updateOptions({
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
        bracketPairs: settings.guides
      },
      folding: settings.folding,
      // Cursor settings
      cursorStyle: settings.cursorStyle,
      cursorBlinking: settings.cursorBlinking,
      smoothScrolling: settings.smoothScrolling,
      // Behavior settings
      mouseWheelZoom: settings.mouseWheelZoom,
      formatOnPaste: settings.formatOnPaste,
      formatOnType: settings.formatOnType
    })
  },
  { deep: true }
)

onMounted(() => {
  initEditor()
})

onBeforeUnmount(() => {
  editorRef.value?.dispose()
})

// 暴露方法
defineExpose({
  getEditor: () => editorRef.value,
  focus: () => editorRef.value?.focus()
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
            'drag-over': dragOverIndex === index && dragIndex !== index
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

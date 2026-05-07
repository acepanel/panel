<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import { useEditorStore } from '@/stores'

import EditorPane from './EditorPane.vue'
import EditorStatusBar from './EditorStatusBar.vue'
import EditorToolbar from './EditorToolbar.vue'
import FileTree from './FileTree.vue'

const { $gettext } = useGettext()
const editorStore = useEditorStore()

const props = defineProps<{
  initialPath?: string
  readOnly?: boolean
}>()

// 侧边栏折叠状态
const siderCollapsed = ref(false)
const siderWidth = ref(250)

// 文件树根目录
const rootPath = ref(props.initialPath || editorStore.rootPath || '/')

// 编辑器面板引用
const editorPaneRef = ref<InstanceType<typeof EditorPane>>()
const fileTreeRef = ref<InstanceType<typeof FileTree>>()
const toolbarRef = ref<InstanceType<typeof EditorToolbar>>()

// 设置弹窗
const showSettings = ref(false)

// 处理工具栏事件
function handleSearch() {
  const editor = editorPaneRef.value?.getEditor()
  if (editor) {
    editor.getAction('actions.find')?.run()
  }
}

function handleReplace() {
  const editor = editorPaneRef.value?.getEditor()
  if (editor) {
    editor.getAction('editor.action.startFindReplaceAction')?.run()
  }
}

function handleGoto() {
  const editor = editorPaneRef.value?.getEditor()
  if (editor) {
    // 需要先聚焦编辑器，gotoLine 才能正常工作
    editor.focus()
    editor.getAction('editor.action.gotoLine')?.run()
  }
}

function handleSettings() {
  showSettings.value = true
}

// 监听根目录变化
watch(rootPath, (newPath) => {
  editorStore.setRootPath(newPath)
})

// 键盘快捷键
function handleKeydown(e: KeyboardEvent) {
  const isMac = /Mac|iPod|iPhone|iPad/.test(navigator.userAgent)
  const modKey = isMac ? e.metaKey : e.ctrlKey

  // Ctrl/Cmd+S 保存
  if (modKey && e.key === 's' && !e.shiftKey) {
    e.preventDefault()
    toolbarRef.value?.save()
  }
  // Ctrl/Cmd+Shift+S 全部保存
  if (modKey && e.shiftKey && e.key.toLowerCase() === 's') {
    e.preventDefault()
    toolbarRef.value?.saveAll()
  }
  // F5 或 Ctrl/Cmd+R 刷新当前文件
  if (e.key === 'F5' || (modKey && e.key === 'r')) {
    e.preventDefault()
    toolbarRef.value?.refresh()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
})

// 暴露方法
defineExpose({
  refresh: () => fileTreeRef.value?.refresh(),
  focus: () => editorPaneRef.value?.focus(),
})
</script>

<template>
  <div class="file-editor-view">
    <!-- 顶部工具栏 -->
    <EditorToolbar
      ref="toolbarRef"
      @search="handleSearch"
      @replace="handleReplace"
      @goto="handleGoto"
      @settings="handleSettings"
    />

    <!-- 主体区域 -->
    <div class="flex-1 overflow-hidden">
      <n-layout has-sider class="h-full">
        <!-- 左侧文件树 -->
        <n-layout-sider
          bordered
          :collapsed="siderCollapsed"
          :collapsed-width="0"
          :width="siderWidth"
          show-trigger="bar"
          collapse-mode="width"
          @update:collapsed="siderCollapsed = $event"
          class="file-tree-sider"
        >
          <FileTree ref="fileTreeRef" v-model:root-path="rootPath" />
        </n-layout-sider>

        <!-- 右侧编辑器区域（包含编辑器和状态栏） -->
        <n-layout class="h-full overflow-hidden">
          <div class="flex flex-col h-full min-w-0 overflow-hidden">
            <EditorPane ref="editorPaneRef" :read-only="readOnly" />
            <EditorStatusBar />
          </div>
        </n-layout>
      </n-layout>
    </div>

    <!-- 设置弹窗 -->
    <n-modal
      v-model:show="showSettings"
      preset="card"
      :title="$gettext('Editor Settings')"
      class="w-150"
    >
      <n-scrollbar style="max-height: 70vh">
        <n-form label-placement="left" label-width="140" class="settings-form">
          <!-- 基础设置 -->
          <n-divider title-placement="left">{{ $gettext('Basic') }}</n-divider>

          <n-form-item :label="$gettext('Tab Size')">
            <n-input-number
              v-model:value="editorStore.settings.tabSize"
              :min="1"
              :max="8"
              @update:value="(v: number | null) => editorStore.updateSettings({ tabSize: v || 4 })"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Use Spaces')">
            <n-switch
              :value="editorStore.settings.insertSpaces"
              @update:value="(v: boolean) => editorStore.updateSettings({ insertSpaces: v })"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Font Size')">
            <n-input-number
              v-model:value="editorStore.settings.fontSize"
              :min="10"
              :max="24"
              @update:value="
                (v: number | null) => editorStore.updateSettings({ fontSize: v || 14 })
              "
            />
          </n-form-item>
          <n-form-item :label="$gettext('Word Wrap')">
            <n-select
              :value="editorStore.settings.wordWrap"
              :options="[
                { label: $gettext('Off'), value: 'off' },
                { label: $gettext('On'), value: 'on' },
                { label: $gettext('Word Wrap Column'), value: 'wordWrapColumn' },
                { label: $gettext('Bounded'), value: 'bounded' },
              ]"
              @update:value="
                (v: 'on' | 'off' | 'wordWrapColumn' | 'bounded') =>
                  editorStore.updateSettings({ wordWrap: v })
              "
            />
          </n-form-item>
          <n-form-item :label="$gettext('Show Minimap')">
            <n-switch
              :value="editorStore.settings.minimap"
              @update:value="(v: boolean) => editorStore.updateSettings({ minimap: v })"
            />
          </n-form-item>

          <!-- 显示设置 -->
          <n-divider title-placement="left">{{ $gettext('Display') }}</n-divider>

          <n-form-item :label="$gettext('Line Numbers')">
            <n-select
              :value="editorStore.settings.lineNumbers"
              :options="[
                { label: $gettext('On'), value: 'on' },
                { label: $gettext('Off'), value: 'off' },
                { label: $gettext('Relative'), value: 'relative' },
                { label: $gettext('Interval'), value: 'interval' },
              ]"
              @update:value="
                (v: 'on' | 'off' | 'relative' | 'interval') =>
                  editorStore.updateSettings({ lineNumbers: v })
              "
            />
          </n-form-item>
          <n-form-item :label="$gettext('Render Whitespace')">
            <n-select
              :value="editorStore.settings.renderWhitespace"
              :options="[
                { label: $gettext('None'), value: 'none' },
                { label: $gettext('Boundary'), value: 'boundary' },
                { label: $gettext('Selection'), value: 'selection' },
                { label: $gettext('Trailing'), value: 'trailing' },
                { label: $gettext('All'), value: 'all' },
              ]"
              @update:value="
                (v: 'none' | 'boundary' | 'selection' | 'trailing' | 'all') =>
                  editorStore.updateSettings({ renderWhitespace: v })
              "
            />
          </n-form-item>
          <n-form-item :label="$gettext('Bracket Colorization')">
            <n-switch
              :value="editorStore.settings.bracketPairColorization"
              @update:value="
                (v: boolean) => editorStore.updateSettings({ bracketPairColorization: v })
              "
            />
          </n-form-item>
          <n-form-item :label="$gettext('Indent Guides')">
            <n-switch
              :value="editorStore.settings.guides"
              @update:value="(v: boolean) => editorStore.updateSettings({ guides: v })"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Code Folding')">
            <n-switch
              :value="editorStore.settings.folding"
              @update:value="(v: boolean) => editorStore.updateSettings({ folding: v })"
            />
          </n-form-item>

          <!-- 光标设置 -->
          <n-divider title-placement="left">{{ $gettext('Cursor') }}</n-divider>

          <n-form-item :label="$gettext('Cursor Style')">
            <n-select
              :value="editorStore.settings.cursorStyle"
              :options="[
                { label: $gettext('Line'), value: 'line' },
                { label: $gettext('Block'), value: 'block' },
                { label: $gettext('Underline'), value: 'underline' },
                { label: $gettext('Line Thin'), value: 'line-thin' },
                { label: $gettext('Block Outline'), value: 'block-outline' },
                { label: $gettext('Underline Thin'), value: 'underline-thin' },
              ]"
              @update:value="
                (
                  v:
                    | 'line'
                    | 'block'
                    | 'underline'
                    | 'line-thin'
                    | 'block-outline'
                    | 'underline-thin',
                ) => editorStore.updateSettings({ cursorStyle: v })
              "
            />
          </n-form-item>
          <n-form-item :label="$gettext('Cursor Blinking')">
            <n-select
              :value="editorStore.settings.cursorBlinking"
              :options="[
                { label: $gettext('Blink'), value: 'blink' },
                { label: $gettext('Smooth'), value: 'smooth' },
                { label: $gettext('Phase'), value: 'phase' },
                { label: $gettext('Expand'), value: 'expand' },
                { label: $gettext('Solid'), value: 'solid' },
              ]"
              @update:value="
                (v: 'blink' | 'smooth' | 'phase' | 'expand' | 'solid') =>
                  editorStore.updateSettings({ cursorBlinking: v })
              "
            />
          </n-form-item>
          <n-form-item :label="$gettext('Smooth Scrolling')">
            <n-switch
              :value="editorStore.settings.smoothScrolling"
              @update:value="(v: boolean) => editorStore.updateSettings({ smoothScrolling: v })"
            />
          </n-form-item>

          <!-- 行为设置 -->
          <n-divider title-placement="left">{{ $gettext('Behavior') }}</n-divider>

          <n-form-item :label="$gettext('Mouse Wheel Zoom')">
            <n-switch
              :value="editorStore.settings.mouseWheelZoom"
              @update:value="(v: boolean) => editorStore.updateSettings({ mouseWheelZoom: v })"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Format On Paste')">
            <n-switch
              :value="editorStore.settings.formatOnPaste"
              @update:value="(v: boolean) => editorStore.updateSettings({ formatOnPaste: v })"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Format On Type')">
            <n-switch
              :value="editorStore.settings.formatOnType"
              @update:value="(v: boolean) => editorStore.updateSettings({ formatOnType: v })"
            />
          </n-form-item>
        </n-form>
      </n-scrollbar>
    </n-modal>
  </div>
</template>

<style scoped lang="scss">
.file-editor-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: var(--n-card-color);
}

.file-tree-sider {
  height: 100%;

  :deep(.n-layout-sider-scroll-container) {
    height: 100%;
  }
}

.settings-form {
  :deep(.n-input-number) {
    width: 180px;
  }

  :deep(.n-select) {
    width: 180px;
  }
}
</style>

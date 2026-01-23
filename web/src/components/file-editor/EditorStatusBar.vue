<script setup lang="ts">
import { useEditorStore } from '@/store'
import { useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
const themeVars = useThemeVars()

// 支持的语言列表
const languages = [
  'plaintext',
  'javascript',
  'typescript',
  'html',
  'css',
  'scss',
  'less',
  'json',
  'xml',
  'yaml',
  'markdown',
  'python',
  'go',
  'java',
  'php',
  'ruby',
  'rust',
  'c',
  'cpp',
  'csharp',
  'shell',
  'sql',
  'nginx',
  'dockerfile'
]

// 缩进选项
const indentOptions = computed(() => [
  { label: `${$gettext('Spaces')}: 2`, value: '2-spaces' },
  { label: `${$gettext('Spaces')}: 4`, value: '4-spaces' },
  { label: `${$gettext('Tabs')}: 2`, value: '2-tabs' },
  { label: `${$gettext('Tabs')}: 4`, value: '4-tabs' }
])

// 当前缩进显示
const currentIndent = computed(() => {
  const { tabSize, insertSpaces } = editorStore.settings
  return insertSpaces ? `${$gettext('Spaces')}: ${tabSize}` : `${$gettext('Tabs')}: ${tabSize}`
})

// 更新行分隔符
function handleLineEndingChange(value: 'LF' | 'CRLF') {
  if (editorStore.activeTab) {
    editorStore.updateLineEnding(editorStore.activeTab.path, value)
  }
}

// 更新语言
function handleLanguageChange(value: string) {
  if (editorStore.activeTab) {
    editorStore.updateLanguage(editorStore.activeTab.path, value)
  }
}

// 更新缩进
function handleIndentChange(value: string) {
  const [size, type] = value.split('-')
  editorStore.updateSettings({
    tabSize: parseInt(size),
    insertSpaces: type === 'spaces'
  })
}
</script>

<template>
  <div class="editor-status-bar" v-if="editorStore.activeTab">
    <!-- 文件路径 -->
    <div class="status-item path">
      <n-ellipsis style="max-width: 400px">
        {{ editorStore.activeTab.path }}
      </n-ellipsis>
    </div>

    <div class="status-spacer" />

    <!-- 行分隔符 -->
    <n-popselect
      :value="editorStore.activeTab.lineEnding"
      :options="[
        { label: 'LF', value: 'LF' },
        { label: 'CRLF', value: 'CRLF' }
      ]"
      @update:value="handleLineEndingChange"
    >
      <div class="status-item clickable">
        {{ editorStore.activeTab.lineEnding }}
      </div>
    </n-popselect>

    <!-- 光标位置 -->
    <div class="status-item">
      {{ $gettext('Ln') }} {{ editorStore.activeTab.cursorLine }}, {{ $gettext('Col') }}
      {{ editorStore.activeTab.cursorColumn }}
    </div>

    <!-- 缩进 -->
    <n-popselect :options="indentOptions" @update:value="handleIndentChange">
      <div class="status-item clickable">
        {{ currentIndent }}
      </div>
    </n-popselect>

    <!-- 语言 -->
    <n-popselect
      :value="editorStore.activeTab.language"
      :options="languages.map((l) => ({ label: l, value: l }))"
      @update:value="handleLanguageChange"
      scrollable
    >
      <div class="status-item clickable">
        {{ $gettext('Language') }}: {{ editorStore.activeTab.language }}
      </div>
    </n-popselect>
  </div>
  <div class="editor-status-bar empty" v-else>
    <span class="status-item">{{ $gettext('No file open') }}</span>
  </div>
</template>

<style scoped lang="scss">
.editor-status-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 12px;
  font-size: 12px;
  background: v-bind('themeVars.cardColor');
  border-top: 1px solid v-bind('themeVars.borderColor');
  flex-shrink: 0;
  height: 26px;
  line-height: 26px;

  &.empty {
    color: v-bind('themeVars.textColor3');
  }
}

.status-item {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  white-space: nowrap;

  &.clickable {
    cursor: pointer;

    &:hover {
      background: v-bind('themeVars.buttonColor2Hover');
    }
  }

  &.path {
    min-width: 0;
    flex-shrink: 1;
  }
}

.status-spacer {
  flex: 1;
}
</style>

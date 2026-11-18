<script setup lang="ts">
import { useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import { useEditorOps } from '@/components/file-editor/composables/useEditorOps'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useEditorStore } from '@/stores'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
const { loadTab } = useEditorOps()
const { confirmAction } = useConfirm()
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
  'dockerfile',
]

// 支持的编码列表
const encodings = computed(() => [
  { name: 'UTF-8', value: 'utf-8' },
  { name: 'UTF-8 BOM', value: 'utf-8-bom' },
  { name: 'UTF-16 LE', value: 'utf-16le' },
  { name: 'UTF-16 BE', value: 'utf-16be' },
  { name: 'GB18030', lang: $gettext('Simplified Chinese'), value: 'gb18030' },
  { name: 'GBK', lang: $gettext('Simplified Chinese'), value: 'gbk' },
  { name: 'Big5', lang: $gettext('Traditional Chinese'), value: 'big5' },
  { name: 'Shift_JIS', lang: $gettext('Japanese'), value: 'shift_jis' },
  { name: 'EUC-JP', lang: $gettext('Japanese'), value: 'euc-jp' },
  { name: 'ISO-2022-JP', lang: $gettext('Japanese'), value: 'iso-2022-jp' },
  { name: 'EUC-KR', lang: $gettext('Korean'), value: 'euc-kr' },
  { name: 'Windows-1252', lang: $gettext('Western European'), value: 'windows-1252' },
  { name: 'Windows-1251', lang: $gettext('Cyrillic'), value: 'windows-1251' },
])

// 编码菜单，两级区分重新解码与转换后保存
const encodingOptions = computed(() => {
  const items = encodings.value.map((e) => ({
    label: e.lang ? `${e.name} (${e.lang})` : e.name,
    value: e.value,
  }))
  return [
    {
      label: $gettext('Reopen with Encoding'),
      key: 'reopen',
      children: items.map((e) => ({ label: e.label, key: `reopen:${e.value}` })),
    },
    {
      label: $gettext('Convert to Encoding'),
      key: 'save',
      children: items.map((e) => ({ label: e.label, key: `save:${e.value}` })),
    },
  ]
})

// 状态栏只显示编码名，不带语言说明
const currentEncoding = computed(() => {
  const encoding = editorStore.activeTab?.encoding ?? ''
  return encodings.value.find((e) => e.value === encoding)?.name ?? encoding.toUpperCase()
})

// 缩进选项
const indentOptions = computed(() => [
  { label: `${$gettext('Spaces')}: 2`, value: '2-spaces' },
  { label: `${$gettext('Spaces')}: 4`, value: '4-spaces' },
  { label: `${$gettext('Tabs')}: 2`, value: '2-tabs' },
  { label: `${$gettext('Tabs')}: 4`, value: '4-tabs' },
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

// 更新编码
function handleEncodingSelect(key: string) {
  const tab = editorStore.activeTab
  if (!tab) return

  const [action, encoding] = key.split(':') as [string, string]
  if (action === 'save') {
    editorStore.updateEncoding(tab.path, encoding)
    return
  }

  // 重新解码需从磁盘取原始字节，未保存的修改会丢失
  if (tab.modified) {
    confirmAction({
      title: $gettext('Unsaved Changes'),
      content: $gettext('This file has unsaved changes. Reopening will discard them. Continue?'),
      positiveText: $gettext('Reopen'),
      onConfirm: () => loadTab(tab.path, encoding),
    })
  } else {
    loadTab(tab.path, encoding)
  }
}

// 更新缩进
function handleIndentChange(value: string) {
  const [size, type] = value.split('-') as [string, string]
  editorStore.updateSettings({
    tabSize: parseInt(size),
    insertSpaces: type === 'spaces',
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
        { label: 'CRLF', value: 'CRLF' },
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

    <!-- 编码 -->
    <n-dropdown
      trigger="click"
      placement="top-end"
      :options="encodingOptions"
      :disabled="editorStore.activeTab.loading"
      @select="handleEncodingSelect"
    >
      <div class="status-item clickable">
        {{ currentEncoding }}
      </div>
    </n-dropdown>
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
  // 窄屏放不下全部状态项时横向滑动，不挤压换行
  overflow-x: auto;
  scrollbar-width: none;

  &::-webkit-scrollbar {
    display: none;
  }

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

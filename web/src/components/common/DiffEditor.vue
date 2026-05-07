<script setup lang="ts">
import type * as Monaco from 'monaco-editor'
import { useThemeVars } from 'naive-ui'

import { useThemeStore } from '@/stores'
import { getMonaco } from '@/utils/monaco'

const props = defineProps({
  original: {
    type: String,
    required: true,
  },
  lang: {
    type: String,
    required: false,
    default: 'yaml',
  },
  height: {
    type: String,
    required: false,
    default: '60vh',
  },
  readOnly: {
    type: Boolean,
    required: false,
    default: false,
  },
})

const modified = defineModel<string>('modified', { type: String, required: true })

const containerRef = ref<HTMLDivElement>()
const editorRef = shallowRef<Monaco.editor.IStandaloneDiffEditor>()
const monacoRef = shallowRef<typeof Monaco>()
const loading = ref(true)

const themeStore = useThemeStore()
const themeVars = useThemeVars()

async function initEditor() {
  if (!containerRef.value) return

  const monaco = await getMonaco(themeStore.locale)
  monacoRef.value = monaco

  const originalModel = monaco.editor.createModel(props.original, props.lang)
  const modifiedModel = monaco.editor.createModel(modified.value, props.lang)

  editorRef.value = monaco.editor.createDiffEditor(containerRef.value, {
    theme: 'vs' + (themeStore.darkMode ? '-dark' : ''),
    automaticLayout: true,
    smoothScrolling: true,
    readOnly: props.readOnly,
    renderSideBySide: true,
    enableSplitViewResizing: true,
    originalEditable: false,
  })

  editorRef.value.setModel({
    original: originalModel,
    modified: modifiedModel,
  })

  // 监听修改后的内容变化
  modifiedModel.onDidChangeContent(() => {
    const newValue = modifiedModel.getValue()
    if (newValue !== modified.value) {
      modified.value = newValue
    }
  })

  loading.value = false
}

watch(
  () => props.original,
  (newOriginal) => {
    if (editorRef.value && monacoRef.value) {
      const model = editorRef.value.getModel()
      if (model?.original) {
        model.original.setValue(newOriginal)
      }
    }
  },
)

watch(modified, (newModified) => {
  if (editorRef.value) {
    const model = editorRef.value.getModel()
    if (model?.modified && model.modified.getValue() !== newModified) {
      model.modified.setValue(newModified)
    }
  }
})

watch(
  () => props.lang,
  (newLang) => {
    if (editorRef.value && monacoRef.value) {
      const model = editorRef.value.getModel()
      if (model?.original) {
        monacoRef.value.editor.setModelLanguage(model.original, newLang)
      }
      if (model?.modified) {
        monacoRef.value.editor.setModelLanguage(model.modified, newLang)
      }
    }
  },
)

watch(
  () => props.readOnly,
  (newReadOnly) => {
    if (editorRef.value) {
      editorRef.value.updateOptions({ readOnly: newReadOnly })
    }
  },
)

onMounted(() => {
  initEditor()
})

onBeforeUnmount(() => {
  const model = editorRef.value?.getModel()
  editorRef.value?.dispose()
  model?.original?.dispose()
  model?.modified?.dispose()
})
</script>

<template>
  <div class="diff-editor" :style="{ height: props.height, borderColor: themeVars.borderColor }">
    <div v-if="loading" class="editor-loading">
      <n-spin size="medium" />
    </div>
    <div ref="containerRef" class="editor-container" :style="{ height: props.height }" />
  </div>
</template>

<style scoped lang="scss">
.diff-editor {
  position: relative;
  width: 100%;
  border: 1px solid;
  border-radius: 3px;
  overflow: hidden;
}

.editor-loading {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1;
}

.editor-container {
  width: 100%;
}
</style>

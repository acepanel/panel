<script setup lang="ts">
import { useThemeVars } from 'naive-ui'

import { useThemeStore } from '@/store'
import { getMonaco } from '@/utils/monaco'
import type * as Monaco from 'monaco-editor'

const value = defineModel<string>('value', { type: String, required: true })
const props = defineProps({
  lang: {
    type: String,
    required: false,
    default: 'ini'
  },
  height: {
    type: String,
    required: false,
    default: '60vh'
  },
  readOnly: {
    type: Boolean,
    required: false
  }
})

const containerRef = ref<HTMLDivElement>()
const editorRef = shallowRef<Monaco.editor.IStandaloneCodeEditor>()
const monacoRef = shallowRef<typeof Monaco>()
const loading = ref(true)

const themeStore = useThemeStore()
const themeVars = useThemeVars()

async function initEditor() {
  if (!containerRef.value) return

  const monaco = await getMonaco(themeStore.locale)
  monacoRef.value = monaco

  editorRef.value = monaco.editor.create(containerRef.value, {
    value: value.value,
    language: props.lang,
    theme: 'vs' + (themeStore.darkMode ? '-dark' : ''),
    readOnly: props.readOnly,
    automaticLayout: true,
    smoothScrolling: true,
    formatOnPaste: true,
    formatOnType: true
  })

  editorRef.value.onDidChangeModelContent(() => {
    const newValue = editorRef.value?.getValue() ?? ''
    if (newValue !== value.value) {
      value.value = newValue
    }
  })

  loading.value = false
}

watch(value, (newValue) => {
  if (editorRef.value && editorRef.value.getValue() !== newValue) {
    editorRef.value.setValue(newValue)
  }
})

watch(
  () => props.lang,
  (newLang) => {
    if (editorRef.value && monacoRef.value) {
      const model = editorRef.value.getModel()
      if (model) {
        monacoRef.value.editor.setModelLanguage(model, newLang)
      }
    }
  }
)

watch(
  () => props.readOnly,
  (newReadOnly) => {
    if (editorRef.value) {
      editorRef.value.updateOptions({ readOnly: newReadOnly })
    }
  }
)

onMounted(() => {
  initEditor()
})

onBeforeUnmount(() => {
  editorRef.value?.dispose()
})
</script>

<template>
  <div
    class="common-editor"
    :style="{ height: props.height, borderColor: themeVars.borderColor }"
  >
    <div v-if="loading" class="editor-loading">
      <n-spin size="medium" />
    </div>
    <div ref="containerRef" class="editor-container" :style="{ height: props.height }" />
  </div>
</template>

<style scoped lang="scss">
.common-editor {
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

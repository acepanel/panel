<script setup lang="ts">
import type * as Monaco from 'monaco-editor'
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useThemeStore } from '@/stores'
import { decodeBase64 } from '@/utils'
import { languageByPath } from '@/utils/file'
import { getMonaco } from '@/utils/monaco'

const { $gettext } = useGettext()
const props = defineProps({
  path: {
    type: String,
    required: true,
  },
  readOnly: {
    type: Boolean,
    required: true,
  },
})

const themeStore = useThemeStore()
const containerRef = ref<HTMLDivElement>()
const editorRef = shallowRef<Monaco.editor.IStandaloneCodeEditor>()
const monacoRef = shallowRef<typeof Monaco>()
const loading = ref(true)

const disabled = ref(false) // 在出现错误的情况下禁用保存
const content = ref('')

async function initEditor() {
  if (!containerRef.value) return

  const monaco = await getMonaco(themeStore.locale)
  monacoRef.value = monaco

  editorRef.value = monaco.editor.create(containerRef.value, {
    value: content.value,
    language: languageByPath(props.path),
    theme:
      (languageByPath(props.path) == 'nginx' ? 'nginx-theme' : 'vs') +
      (themeStore.darkMode ? '-dark' : ''),
    readOnly: props.readOnly,
    automaticLayout: true,
    smoothScrolling: true,
    formatOnPaste: true,
    formatOnType: true,
  })

  editorRef.value.onDidChangeModelContent(() => {
    const newValue = editorRef.value?.getValue() ?? ''
    if (newValue !== content.value) {
      content.value = newValue
    }
  })

  loading.value = false
}

watch(content, (newValue) => {
  if (editorRef.value && editorRef.value.getValue() !== newValue) {
    editorRef.value.setValue(newValue)
  }
})

watch(
  () => props.readOnly,
  (newReadOnly) => {
    if (editorRef.value) {
      editorRef.value.updateOptions({ readOnly: newReadOnly })
    }
  },
)

const get = () => {
  useRequest(file.content(encodeURIComponent(props.path)))
    .onSuccess(({ data }) => {
      content.value = decodeBase64(data.content)
      window.$message.success($gettext('Retrieved successfully'))
    })
    .onError(() => {
      disabled.value = true
    })
}

const save = () => {
  if (disabled.value) {
    window.$message.error($gettext('Cannot save in current state'))
    return
  }
  useRequest(file.save(props.path, content.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

onMounted(() => {
  get()
  initEditor()
})

onBeforeUnmount(() => {
  editorRef.value?.dispose()
})

defineExpose({
  get,
  save,
})
</script>

<template>
  <div class="file-editor" style="height: 60vh">
    <div v-if="loading" class="editor-loading">
      <n-spin size="medium" />
    </div>
    <div ref="containerRef" class="editor-container" style="height: 60vh" />
  </div>
</template>

<style scoped lang="scss">
.file-editor {
  position: relative;
  width: 100%;
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

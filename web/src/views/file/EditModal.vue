<script setup lang="ts">
import file from '@/api/panel/file'
import DraggableWindow from '@/components/common/DraggableWindow.vue'
import { FileEditorView } from '@/components/file-editor'
import { useEditorStore } from '@/store'
import { decodeBase64 } from '@/utils'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const editorStore = useEditorStore()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const minimized = defineModel<boolean>('minimized', { type: Boolean, default: false })
const filePath = defineModel<string>('file', { type: String, required: true })

const editorRef = ref<InstanceType<typeof FileEditorView>>()

// 窗口默认尺寸
const defaultWidth = Math.min(1400, window.innerWidth * 0.9)
const defaultHeight = Math.min(900, window.innerHeight * 0.85)

// 获取文件所在目录作为初始路径
const initialPath = computed(() => {
  if (!filePath.value) return '/'
  const parts = filePath.value.split('/')
  parts.pop()
  return parts.join('/') || '/'
})

// 加载文件
function loadFile(path: string) {
  if (!path) return

  // 如果文件已经打开，直接切换到该标签页
  if (editorStore.tabs.some(f => f.path === path)) {
    editorStore.switchTab(path)
    return
  }

  // 打开新文件
  editorStore.openFile(path, '')
  editorStore.setLoading(path, true)

  useRequest(file.content(encodeURIComponent(path)))
    .onSuccess(({ data }) => {
      const content = decodeBase64(data.content)
      editorStore.reloadFile(path, content)
    })
    .onError(() => {
      window.$message.error($gettext('Failed to load file'))
      editorStore.closeTab(path)
    })
    .onComplete(() => {
      editorStore.setLoading(path, false)
    })
}

// 打开时自动加载文件
watch(show, (newShow) => {
  if (newShow && filePath.value) {
    // 暂停文件管理的键盘快捷键
    window.$bus.emit('file:keyboard-pause')

    // 清空之前的标签页
    editorStore.closeAllTabs()
    // 设置根目录
    editorStore.setRootPath(initialPath.value)
    // 加载文件
    loadFile(filePath.value)
  } else if (!newShow) {
    // 恢复文件管理的键盘快捷键
    window.$bus.emit('file:keyboard-resume')
  }
})

// 监听文件路径变化（编辑器已打开时双击新文件）
watch(filePath, (newPath) => {
  if (show.value && newPath) {
    loadFile(newPath)
  }
})

// 监听最小化状态
watch(minimized, (isMinimized) => {
  if (isMinimized) {
    window.$bus.emit('file:keyboard-resume')
  } else {
    window.$bus.emit('file:keyboard-pause')
  }
})
</script>

<template>
  <DraggableWindow
    v-model:show="show"
    v-model:minimized="minimized"
    :title="$gettext('File Editor')"
    :default-width="defaultWidth"
    :default-height="defaultHeight"
    :min-width="600"
    :min-height="400"
  >
    <FileEditorView ref="editorRef" :initial-path="initialPath" />
  </DraggableWindow>
</template>

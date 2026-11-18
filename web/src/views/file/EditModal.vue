<script setup lang="ts">
import { NButton, NFlex } from 'naive-ui'
import { h } from 'vue'
import { useGettext } from 'vue3-gettext'

import { useEditorOps } from '@/components/file-editor/composables/useEditorOps'
import AppButton from '@/components/system/AppButton.vue'
import { useEditorStore } from '@/stores'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
const { openInEditor, saveTabs } = useEditorOps()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const minimized = defineModel<boolean>('minimized', { type: Boolean, default: false })
const filePath = defineModel<string>('file', { type: String, required: true })

// 窗口尺寸随视口重新钳位，旋转屏幕或缩放后再打开不会超出视口
const { width: viewportWidth, height: viewportHeight } = useWindowSize()
const defaultWidth = computed(() => Math.min(1400, viewportWidth.value * 0.9))
const defaultHeight = computed(() => Math.min(900, viewportHeight.value * 0.85))
const minWidth = computed(() => Math.min(600, defaultWidth.value))
const minHeight = computed(() => Math.min(400, defaultHeight.value))

// 获取文件所在目录作为初始路径
const initialPath = computed(() => {
  if (!filePath.value) return '/'
  const parts = filePath.value.split('/')
  parts.pop()
  return parts.join('/') || '/'
})

// 关闭前确认
async function handleBeforeClose(): Promise<boolean> {
  // 检查是否有未保存的文件
  if (!editorStore.hasUnsavedFiles) {
    return true // 没有未保存的文件，直接关闭
  }

  // 显示确认对话框
  return new Promise((resolve) => {
    // 保存期间不能返回或放弃，也不能按 Esc 关掉
    const saving = ref(false)
    const d = window.$dialog.warning({
      title: $gettext('Unsaved Changes'),
      content: $gettext('You have unsaved changes. Do you want to save them before closing?'),
      closable: false,
      maskClosable: false,
      action: () =>
        h(NFlex, { justify: 'end' }, () => [
          h(
            NButton,
            {
              disabled: saving.value,
              onClick: () => {
                d.destroy()
                resolve(false) // 返回编辑器
              },
            },
            () => $gettext('Go Back'),
          ),
          h(
            NButton,
            {
              type: 'warning',
              disabled: saving.value,
              onClick: () => {
                d.destroy()
                resolve(true) // 放弃更改，关闭窗口
              },
            },
            () => $gettext('Discard'),
          ),
          h(
            AppButton,
            {
              type: 'primary',
              onClick: async () => {
                saving.value = true
                d.closeOnEsc = false
                // 保存所有未保存的文件，saveTabs 不会抛错，结束后对话框总会关闭
                const failed = await saveTabs(editorStore.unsavedTabs.map((t) => t.path))

                d.destroy()
                if (failed.length === 0) {
                  window.$message.success($gettext('All files saved successfully'))
                  resolve(true) // 保存成功，关闭窗口
                } else {
                  const fileList = failed.map((f) => f.split('/').pop()).join(', ')
                  window.$message.error(
                    $gettext('Failed to save files: %{ files }', { files: fileList }),
                  )
                  resolve(false) // 保存失败，不关闭窗口
                }
              },
            },
            () => $gettext('Save'),
          ),
        ]),
    })
  })
}

// 加载文件（已打开则切换，加载失败自动关闭标签页）
function loadFile(path: string) {
  if (!path) return
  openInEditor(path)
}

// 打开时自动加载文件
watch(show, (newShow) => {
  if (newShow && filePath.value) {
    // 清空之前的标签页
    editorStore.closeAllTabs()
    // 设置根目录
    editorStore.setRootPath(initialPath.value)
    // 加载文件
    loadFile(filePath.value)
  }
})

// 监听文件路径变化（编辑器已打开时双击新文件）
watch(filePath, (newPath) => {
  if (show.value && newPath) {
    loadFile(newPath)
  }
})

// 关闭或最小化后列表重新可见，刷新以同步保存后的大小和修改时间
watch(
  () => show.value && !minimized.value,
  (covering) => {
    if (!covering) window.$bus.emit('file:refresh')
  },
)
</script>

<template>
  <DraggableWindow
    v-model:show="show"
    v-model:minimized="minimized"
    :title="$gettext('File Editor')"
    :default-width="defaultWidth"
    :default-height="defaultHeight"
    :min-width="minWidth"
    :min-height="minHeight"
    :before-close="handleBeforeClose"
    :close-on-overlay="false"
  >
    <FileEditorView ref="editorRef" :initial-path="initialPath" :active="!minimized" />
  </DraggableWindow>
</template>

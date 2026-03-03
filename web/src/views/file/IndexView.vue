<script setup lang="ts">
defineOptions({
  name: 'file-index'
})

import { useFileStore } from '@/store'
import CompressModal from '@/views/file/CompressModal.vue'
import ListView from '@/views/file/ListView.vue'
import PathInput from '@/views/file/PathInput.vue'
import PermissionModal from '@/views/file/PermissionModal.vue'
import ToolBar from '@/views/file/ToolBar.vue'
import UploadModal from '@/views/file/UploadModal.vue'
import type { FileInfo } from '@/views/file/types'

const fileStore = useFileStore()

const selected = ref<string[]>([])
// 权限编辑时的文件信息列表
const permissionFileInfoList = ref<FileInfo[]>([])

const compress = ref(false)
const permission = ref(false)

// 上传相关
const upload = ref(false)
const droppedFiles = ref<File[]>([])
const isDragging = ref(false)

// 切换标签页时清空选中
watch(
  () => fileStore.activeTabId,
  () => {
    selected.value = []
  }
)

// n-tabs 事件
const handleTabSwitch = (tabId: string | number) => {
  fileStore.switchTab(tabId as string)
}
const handleTabClose = (tabId: string | number) => {
  fileStore.closeTab(tabId as string)
}
const handleTabAdd = () => {
  fileStore.createTab()
}

// 中键关闭标签页
const handleTabMiddleClick = (tabId: string) => {
  if (fileStore.tabs.length > 1) {
    fileStore.closeTab(tabId)
  }
}

// ==================== 文件拖拽上传 ====================
const handleDragEnter = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  if (e.dataTransfer?.types.includes('Files')) {
    isDragging.value = true
  }
}

const handleDragLeave = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  if (
    e.clientX <= rect.left ||
    e.clientX >= rect.right ||
    e.clientY <= rect.top ||
    e.clientY >= rect.bottom
  ) {
    isDragging.value = false
  }
}

const handleDragOver = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
}

// 递归读取目录中的所有文件
const readDirectoryRecursively = async (
  entry: FileSystemDirectoryEntry,
  basePath: string = ''
): Promise<File[]> => {
  const files: File[] = []
  const reader = entry.createReader()

  const readEntries = (): Promise<FileSystemEntry[]> => {
    return new Promise((resolve, reject) => {
      reader.readEntries(resolve, reject)
    })
  }

  let entries: FileSystemEntry[] = []
  let batch: FileSystemEntry[]
  do {
    batch = await readEntries()
    entries = entries.concat(batch)
  } while (batch.length > 0)

  for (const childEntry of entries) {
    const childPath = basePath ? `${basePath}/${childEntry.name}` : childEntry.name
    if (childEntry.isFile) {
      const fileEntry = childEntry as FileSystemFileEntry
      const file = await new Promise<File>((resolve, reject) => {
        fileEntry.file((f) => {
          const newFile = new File([f], childPath, { type: f.type, lastModified: f.lastModified })
          resolve(newFile)
        }, reject)
      })
      files.push(file)
    } else if (childEntry.isDirectory) {
      const subFiles = await readDirectoryRecursively(
        childEntry as FileSystemDirectoryEntry,
        childPath
      )
      files.push(...subFiles)
    }
  }

  return files
}

const handleDrop = async (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  isDragging.value = false

  const items = e.dataTransfer?.items
  if (!items || items.length === 0) return

  const files: File[] = []

  for (let i = 0; i < items.length; i++) {
    const item = items[i]
    if (item?.kind === 'file') {
      const entry = item.webkitGetAsEntry()
      if (entry) {
        if (entry.isFile) {
          const file = item.getAsFile()
          if (file) files.push(file)
        } else if (entry.isDirectory) {
          const dirFiles = await readDirectoryRecursively(
            entry as FileSystemDirectoryEntry,
            entry.name
          )
          files.push(...dirFiles)
        }
      }
    }
  }

  if (files.length > 0) {
    droppedFiles.value = files
    upload.value = true
  }
}

// 监听上传弹窗关闭，清空预拖入的文件
watch(upload, (val) => {
  if (!val) {
    droppedFiles.value = []
  }
})
</script>

<template>
  <common-page
    show-footer
    flex
    @dragenter="handleDragEnter"
    @dragleave="handleDragLeave"
    @dragover="handleDragOver"
    @drop="handleDrop"
  >
    <n-flex vertical :size="20" class="flex-1 min-h-0">
      <!-- 标签页栏 -->
      <n-tabs
        type="card"
        size="small"
        :value="fileStore.activeTabId"
        :closable="fileStore.tabs.length > 1"
        addable
        class="file-tabs"
        @update:value="handleTabSwitch"
        @close="handleTabClose"
        @add="handleTabAdd"
      >
        <n-tab-pane v-for="tab in fileStore.tabs" :key="tab.id" :name="tab.id">
          <template #tab>
            <span
              :title="tab.path"
              @mousedown.middle.prevent="handleTabMiddleClick(tab.id)"
            >
              {{ tab.label }}
            </span>
          </template>
        </n-tab-pane>
      </n-tabs>

      <!-- 每个标签页内容（v-if 只渲染活跃的） -->
      <template v-for="tab in fileStore.tabs" :key="tab.id">
        <template v-if="tab.id === fileStore.activeTabId">
          <path-input :tab-id="tab.id" />
          <tool-bar
            :tab-id="tab.id"
            v-model:selected="selected"
            v-model:compress="compress"
            v-model:permission="permission"
            v-model:upload="upload"
          />
          <list-view
            :tab-id="tab.id"
            v-model:selected="selected"
            v-model:compress="compress"
            v-model:permission="permission"
            v-model:permission-file-info-list="permissionFileInfoList"
          />
        </template>
      </template>

      <template v-if="fileStore.activeTab">
        <compress-modal
          v-model:show="compress"
          v-model:path="fileStore.activeTab.path"
          v-model:selected="selected"
        />
        <permission-modal
          v-model:show="permission"
          v-model:selected="selected"
          v-model:file-info-list="permissionFileInfoList"
        />
      </template>
    </n-flex>

    <!-- 拖拽上传遮罩 -->
    <div v-if="isDragging" class="drag-overlay">
      <div class="drag-content">
        <the-icon icon="mdi:cloud-upload" :size="64" />
        <p>释放文件以上传</p>
      </div>
    </div>

    <!-- 上传弹窗 -->
    <upload-modal
      v-if="fileStore.activeTab"
      v-model:show="upload"
      v-model:path="fileStore.activeTab.path"
      :initial-files="droppedFiles"
    />
  </common-page>
</template>

<style scoped lang="scss">
.drag-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  pointer-events: none;
}

.drag-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  color: white;
  font-size: 18px;
}

// n-tabs 只用作导航栏，隐藏空的 pane 区域
.file-tabs {
  flex-shrink: 0;
  margin-bottom: -8px;

  :deep(.n-tabs-pane-wrapper) {
    display: none;
  }

  :deep(.n-tabs-tab) {
    padding: 4px 12px !important;
  }
}
</style>

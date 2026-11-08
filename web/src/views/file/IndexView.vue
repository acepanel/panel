<script setup lang="ts">
defineOptions({
  name: 'file-index',
})

import { useGettext } from 'vue3-gettext'

import { useFileStore, useUploadStore } from '@/stores'
import { readDroppedFiles } from '@/utils/file'
import CompressModal from '@/views/file/CompressModal.vue'
import EditModal from '@/views/file/EditModal.vue'
import ListView from '@/views/file/ListView.vue'
import PathInput from '@/views/file/PathInput.vue'
import PermissionModal from '@/views/file/PermissionModal.vue'
import TaskQueueWindow from '@/views/file/TaskQueueWindow.vue'
import ToolBar from '@/views/file/ToolBar.vue'
import UploadModal from '@/views/file/UploadModal.vue'

const { $gettext } = useGettext()
const fileStore = useFileStore()
const uploadStore = useUploadStore()

const compress = ref(false)
const permission = ref(false)

// 编辑器承载在此层，跨文件标签页共享同一实例，避免切换标签页销毁未保存内容
const editorModal = ref(false)
const editorMinimized = ref(false)
const editorFile = ref('')

const handleEditFile = (path: string) => {
  editorFile.value = path
  editorMinimized.value = false
  editorModal.value = true
}

// 上传相关
const upload = ref(false)
const isDragging = ref(false)

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

// 拖入的文件加入上传队列并打开队列弹窗
const handleDrop = async (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  isDragging.value = false

  const path = fileStore.activeTab?.path
  if (!path) return
  const files = await readDroppedFiles(e.dataTransfer)
  if (files.length > 0) {
    uploadStore.add(files, path)
    upload.value = true
  }
}

onMounted(() => {
  window.$bus.on('file:edit', handleEditFile)
  fileStore.loadFavorites()
})

onUnmounted(() => {
  window.$bus.off('file:edit', handleEditFile)
})
</script>

<template>
  <PageContainer
    :show-footer="true"
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
            <span :title="tab.path" @mousedown.middle.prevent="handleTabMiddleClick(tab.id)">
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
            v-model:compress="compress"
            v-model:permission="permission"
            v-model:upload="upload"
          />
          <list-view :tab-id="tab.id" v-model:compress="compress" v-model:permission="permission" />
        </template>
      </template>

      <template v-if="fileStore.activeTab">
        <compress-modal v-model:show="compress" v-model:path="fileStore.activeTab.path" />
        <permission-modal v-model:show="permission" />
      </template>

      <!-- 编辑弹窗（跨标签页共享） -->
      <edit-modal
        v-model:show="editorModal"
        v-model:minimized="editorMinimized"
        v-model:file="editorFile"
      />
    </n-flex>

    <!-- 拖拽上传遮罩 -->
    <div v-if="isDragging" class="drag-overlay">
      <div class="drag-content">
        <the-icon icon="mdi:cloud-upload" :size="64" />
        <p>{{ $gettext('Drop files to upload') }}</p>
      </div>
    </div>

    <task-queue-window />

    <!-- 上传弹窗 -->
    <upload-modal
      v-if="fileStore.activeTab"
      v-model:show="upload"
      :path="fileStore.activeTab.path"
    />
  </PageContainer>
</template>

<style scoped lang="scss">
.drag-overlay {
  position: absolute;
  inset: 0;
  background: var(--color-overlay);
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

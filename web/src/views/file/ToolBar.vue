<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { useFileStore, useTaskQueueStore, useUploadStore } from '@/stores'
import { checkName, joinPath } from '@/utils/file'
import { useFileOps } from '@/views/file/composables/useFileOps'
import { usePaste } from '@/views/file/composables/usePaste'

const { $gettext } = useGettext()
const fileStore = useFileStore()
const uploadStore = useUploadStore()
const taskQueue = useTaskQueueStore()
const { pasting, handlePaste: doPaste } = usePaste()
const { deletePaths, markClipboard } = useFileOps()
const { isMobile } = useResponsive()
const { confirmDelete } = useConfirm()

const props = defineProps<{
  tabId: string
}>()

const compress = defineModel<boolean>('compress', { type: Boolean, required: true })
const permission = defineModel<boolean>('permission', { type: Boolean, required: true })
const upload = defineModel<boolean>('upload', { type: Boolean, required: true })

const tab = computed(() => fileStore.tabs.find((t) => t.id === props.tabId)!)
const path = computed(() => tab.value.path)
const marked = computed(() => fileStore.clipboard.marked)
const selected = computed(() => fileStore.selected[props.tabId] ?? [])

// 终端弹窗
const terminalModal = ref(false)
watch(terminalModal, (show) => {
  if (!show) window.$bus.emit('file:refresh')
})

const download = ref(false)
const downloadModel = ref({
  path: '',
  url: '',
})

const showCreate = (value: string) => {
  // 触发内联新建事件
  window.$bus.emit('file:inline-create', value !== 'file')
}

const handleDownload = async () => {
  if (!checkName(downloadModel.value.path)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  const data = await file.remoteDownload(
    joinPath(path.value, downloadModel.value.path),
    downloadModel.value.url,
  )
  download.value = false
  taskQueue.add(data)
}

const handleCopy = () => {
  if (!selected.value.length) {
    window.$message.error($gettext('Please select files/folders to copy'))
    return
  }
  markClipboard(selected.value, 'copy')
}

const handleMove = () => {
  if (!selected.value.length) {
    window.$message.error($gettext('Please select files/folders to move'))
    return
  }
  markClipboard(selected.value, 'move')
}

const handleCancel = () => {
  fileStore.clearClipboard()
}

const handlePaste = () => doPaste(path.value)

const bulkDelete = () => deletePaths(selected.value)
const batchDeleting = ref(false)

// 批量操作：桌面按钮组，手机放不下改下拉，下拉里的删除改用对话框确认
const batchActions = computed(() => [
  { label: $gettext('Copy'), key: 'copy', handler: handleCopy },
  { label: $gettext('Move'), key: 'move', handler: handleMove },
  { label: $gettext('Compress'), key: 'compress', handler: () => (compress.value = true) },
  { label: $gettext('Permission'), key: 'permission', handler: () => (permission.value = true) },
])
const batchMobileActions = computed(() => [
  ...batchActions.value,
  {
    label: $gettext('Delete'),
    key: 'delete',
    handler: async () => {
      if (await confirmDelete({ content: $gettext('Are you sure you want to delete in bulk?') })) {
        batchDeleting.value = true
        try {
          await bulkDelete()
        } finally {
          batchDeleting.value = false
        }
      }
    },
  },
])
const handleBatchSelect = (key: string) =>
  batchMobileActions.value.find((item) => item.key === key)?.handler()

// 自动填充下载文件名
watch(
  () => downloadModel.value.url,
  (newUrl) => {
    if (!newUrl) return
    try {
      const url = new URL(newUrl)
      const path = url.pathname.split('/').pop()
      if (path) {
        downloadModel.value.path = decodeURIComponent(path).replace(
          /[~!@#$%^&*()+=[\]{}|;:'",<>?/]/g,
          '-',
        )
      }
    } catch (error) {
      /* empty */
    }
  },
)

// 打开终端
const openTerminal = () => {
  terminalModal.value = true
}

// 切换视图类型
const toggleViewType = () => {
  fileStore.toggleViewType()
}

// 排序选项
const sortOptions = computed(() => [
  { label: $gettext('Name'), value: 'name' },
  { label: $gettext('Size'), value: 'size' },
  { label: $gettext('Modification Time'), value: 'modify' },
])

// 当前排序显示文本
const currentSortLabel = computed(() => {
  if (!fileStore.sortKey) return $gettext('Sort')
  const option = sortOptions.value.find((o) => o.value === fileStore.sortKey)
  const label = option?.label || fileStore.sortKey
  const arrow = fileStore.sortOrder === 'asc' ? '↑' : '↓'
  return `${label} ${arrow}`
})

// 处理排序选择
const handleSortSelect = (key: string) => {
  fileStore.setSort(key)
}
</script>

<template>
  <n-flex>
    <n-popselect
      :options="[
        { label: $gettext('File'), value: 'file' },
        { label: $gettext('Folder'), value: 'folder' },
      ]"
      @update:value="showCreate"
    >
      <n-button type="primary">{{ $gettext('New') }}</n-button>
    </n-popselect>
    <!-- 后台有上传任务时显示数量角标 -->
    <n-badge :value="uploadStore.activeCount" :max="99">
      <n-button @click="upload = true">{{ $gettext('Upload') }}</n-button>
    </n-badge>
    <n-button @click="download = true">{{ $gettext('Remote Download') }}</n-button>
    <n-badge v-if="taskQueue.tasks.length" :value="taskQueue.tasks.length" :max="99">
      <n-button @click="taskQueue.open()">{{ $gettext('Task Queue') }}</n-button>
    </n-badge>
    <n-button @click="openTerminal">{{ $gettext('Terminal') }}</n-button>
    <n-popselect :options="sortOptions" :value="fileStore.sortKey" @update:value="handleSortSelect">
      <n-button>
        <template #icon>
          <i-mdi-sort class="text-lg" />
        </template>
        {{ currentSortLabel }}
      </n-button>
    </n-popselect>
    <n-tooltip>
      <template #trigger>
        <n-button @click="toggleViewType">
          <i-mdi-view-list v-if="fileStore.viewType === 'list'" class="text-lg" />
          <i-mdi-view-grid v-else class="text-lg" />
        </n-button>
      </template>
      {{
        fileStore.viewType === 'list'
          ? $gettext('Switch to grid view')
          : $gettext('Switch to list view')
      }}
    </n-tooltip>
    <div class="ml-auto">
      <n-flex>
        <n-button v-if="marked.length" secondary type="error" @click="handleCancel">
          {{ $gettext('Cancel') }}
        </n-button>
        <app-button
          v-if="marked.length"
          secondary
          type="primary"
          :loading="pasting"
          @click="handlePaste"
        >
          {{ $gettext('Paste') }}
        </app-button>
        <template v-if="selected.length">
          <n-dropdown
            v-if="isMobile"
            trigger="click"
            :options="batchMobileActions"
            :disabled="batchDeleting"
            @select="handleBatchSelect"
          >
            <n-button :loading="batchDeleting">{{ $gettext('Actions') }}</n-button>
          </n-dropdown>
          <n-button-group v-else>
            <n-button v-for="item in batchActions" :key="item.key" @click="item.handler">
              {{ item.label }}
            </n-button>
            <ConfirmDialog
              type="danger"
              :content="$gettext('Are you sure you want to delete in bulk?')"
              @confirm="bulkDelete"
            >
              <template #trigger>
                <app-button :disabled="selected.length === 0" ghost>
                  {{ $gettext('Delete') }}
                </app-button>
              </template>
            </ConfirmDialog>
          </n-button-group>
        </template>
      </n-flex>
    </div>
  </n-flex>
  <app-modal v-model:show="download" :title="$gettext('Remote Download')">
    <n-space vertical>
      <n-form :model="downloadModel">
        <n-form-item :label="$gettext('Download URL')">
          <n-input :input-props="{ type: 'url' }" v-model:value="downloadModel.url" />
        </n-form-item>
        <n-form-item :label="$gettext('Save as')">
          <n-input v-model:value="downloadModel.path" />
        </n-form-item>
      </n-form>
      <app-button type="info" block @click="handleDownload">{{ $gettext('Submit') }}</app-button>
    </n-space>
  </app-modal>
  <!-- 终端弹窗 -->
  <pty-terminal-modal
    v-model:show="terminalModal"
    :title="$gettext('Terminal - %{ path }', { path })"
    :command="`cd '${path}' && exec bash -l`"
  />
</template>

<style scoped lang="scss"></style>

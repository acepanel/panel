<script setup lang="ts">
import file from '@/api/panel/file'
import PtyTerminalModal from '@/components/common/PtyTerminalModal.vue'
import { checkName, lastDirectory } from '@/utils/file'
import UploadModal from '@/views/file/UploadModal.vue'
import type { FileInfo, Marked } from '@/views/file/types'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

const path = defineModel<string>('path', { type: String, required: true })
const selected = defineModel<string[]>('selected', { type: Array, default: () => [] })
const marked = defineModel<Marked[]>('marked', { type: Array, default: () => [] })
const markedType = defineModel<string>('markedType', { type: String, required: true })
const compress = defineModel<boolean>('compress', { type: Boolean, required: true })
const permission = defineModel<boolean>('permission', { type: Boolean, required: true })
// 权限编辑时的文件信息列表
const permissionFileInfoList = defineModel<FileInfo[]>('permissionFileInfoList', { type: Array, default: () => [] })

// 视图类型：list（列表视图）或 grid（图标视图）
const viewType = ref<'list' | 'grid'>('list')

// 终端弹窗
const terminalModal = ref(false)

const upload = ref(false)
const create = ref(false)
const createModel = ref({
  dir: false,
  path: ''
})
const download = ref(false)
const downloadModel = ref({
  path: '',
  url: ''
})

const showCreate = (value: string) => {
  createModel.value.dir = value !== 'file'
  createModel.value.path = ''
  create.value = true
}

const handleCreate = () => {
  if (!checkName(createModel.value.path)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  const fullPath = path.value + '/' + createModel.value.path
  useRequest(file.create(fullPath, createModel.value.dir)).onSuccess(() => {
    create.value = false
    window.$bus.emit('file:refresh')
    window.$message.success($gettext('Created successfully'))
  })
}

const handleDownload = () => {
  if (!checkName(downloadModel.value.path)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  useRequest(
    file.remoteDownload(path.value + '/' + downloadModel.value.path, downloadModel.value.url)
  ).onSuccess(() => {
    download.value = false
    window.$bus.emit('file:refresh')
    window.$message.success($gettext('Download task created successfully'))
  })
}

const handleCopy = () => {
  if (!selected.value.length) {
    window.$message.error($gettext('Please select files/folders to copy'))
    return
  }
  markedType.value = 'copy'
  marked.value = selected.value.map((path) => ({
    name: lastDirectory(path),
    source: path,
    force: false
  }))
  selected.value = []
  window.$message.success(
    $gettext('Marked successfully, please navigate to the destination path to paste')
  )
}

const handleMove = () => {
  if (!selected.value.length) {
    window.$message.error($gettext('Please select files/folders to move'))
    return
  }
  markedType.value = 'move'
  marked.value = selected.value.map((path) => ({
    name: lastDirectory(path),
    source: path,
    force: false
  }))
  selected.value = []
  window.$message.success(
    $gettext('Marked successfully, please navigate to the destination path to paste')
  )
}

const handleCancel = () => {
  marked.value = []
}

const handlePaste = () => {
  if (!marked.value.length) {
    window.$message.error($gettext('Please mark the files/folders to copy or move first'))
    return
  }

  // 查重
  let flag = false
  const paths = marked.value.map((item) => {
    return {
      name: item.name,
      source: item.source,
      target: path.value + '/' + item.name,
      force: false
    }
  })
  const sources = paths.map((item: any) => item.target)
  useRequest(file.exist(sources)).onSuccess(({ data }) => {
    for (let i = 0; i < data.length; i++) {
      if (data[i]) {
        flag = true
        paths[i].force = true
      }
    }
    if (flag) {
      window.$dialog.warning({
        title: $gettext('Warning'),
        content: $gettext(
          'There are items with the same name %{ items } Do you want to overwrite?',
          {
            items: `${paths
              .filter((item) => item.force)
              .map((item) => item.name)
              .join(', ')}`
          }
        ),
        positiveText: $gettext('Overwrite'),
        negativeText: $gettext('Cancel'),
        onPositiveClick: async () => {
          if (markedType.value == 'copy') {
            useRequest(file.copy(paths)).onSuccess(() => {
              marked.value = []
              window.$bus.emit('file:refresh')
              window.$message.success($gettext('Copied successfully'))
            })
          } else {
            useRequest(file.move(paths)).onSuccess(() => {
              marked.value = []
              window.$bus.emit('file:refresh')
              window.$message.success($gettext('Moved successfully'))
            })
          }
        },
        onNegativeClick: () => {
          marked.value = []
          window.$message.info($gettext('Canceled'))
        }
      })
    } else {
      if (markedType.value == 'copy') {
        useRequest(file.copy(paths)).onSuccess(() => {
          marked.value = []
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Copied successfully'))
        })
      } else {
        useRequest(file.move(paths)).onSuccess(() => {
          marked.value = []
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Moved successfully'))
        })
      }
    }
  })
}

const bulkDelete = async () => {
  const promises = selected.value.map((path) => file.delete(path))
  await Promise.all(promises)

  selected.value = []
  window.$bus.emit('file:refresh')
  window.$message.success($gettext('Deleted successfully'))
}

// 自动填充下载文件名
watch(
  () => downloadModel.value.url,
  (newUrl) => {
    if (!newUrl) return
    try {
      const url = new URL(newUrl)
      const path = url.pathname.split('/').pop()
      if (path) {
        downloadModel.value.path = decodeURIComponent(path)
      }
    } catch (error) {
      /* empty */
    }
  }
)

// 打开终端
const openTerminal = () => {
  terminalModal.value = true
}

// 切换视图类型（预留功能）
const toggleViewType = () => {
  // 目前只支持列表视图，图标视图将在未来实现
  if (viewType.value === 'list') {
    window.$message.info($gettext('Grid view is coming soon'))
    return
  }
  viewType.value = 'list'
}
</script>

<template>
  <n-flex>
    <n-popselect
      :options="[
        { label: $gettext('File'), value: 'file' },
        { label: $gettext('Folder'), value: 'folder' }
      ]"
      @update:value="showCreate"
    >
      <n-button type="primary">{{ $gettext('New') }}</n-button>
    </n-popselect>
    <n-button @click="upload = true">{{ $gettext('Upload') }}</n-button>
    <n-button @click="download = true">{{ $gettext('Remote Download') }}</n-button>
    <n-button @click="openTerminal">{{ $gettext('Terminal') }}</n-button>
    <n-divider vertical />
    <n-tooltip>
      <template #trigger>
        <n-button @click="toggleViewType">
          <i-mdi-view-list v-if="viewType === 'list'" :size="16" />
          <i-mdi-view-grid v-else :size="16" />
        </n-button>
      </template>
      {{ viewType === 'list' ? $gettext('Switch to grid view') : $gettext('Switch to list view') }}
    </n-tooltip>
    <div ml-auto>
      <n-flex>
        <n-button v-if="marked.length" secondary type="error" @click="handleCancel">
          {{ $gettext('Cancel') }}
        </n-button>
        <n-button v-if="marked.length" secondary type="primary" @click="handlePaste">
          {{ $gettext('Paste') }}
        </n-button>
        <n-button-group v-if="selected.length">
          <n-button @click="handleCopy">{{ $gettext('Copy') }}</n-button>
          <n-button @click="handleMove">{{ $gettext('Move') }}</n-button>
          <n-button @click="compress = true">{{ $gettext('Compress') }}</n-button>
          <n-button @click="permission = true">{{ $gettext('Permission') }}</n-button>
          <n-popconfirm @positive-click="bulkDelete">
            <template #trigger>
              <n-button :disabled="selected.length === 0" ghost>
                {{ $gettext('Delete') }}
              </n-button>
            </template>
            {{ $gettext('Are you sure you want to delete in bulk?') }}
          </n-popconfirm>
        </n-button-group>
      </n-flex>
    </div>
  </n-flex>
  <n-modal
    v-model:show="create"
    preset="card"
    :title="$gettext('New')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-form :model="createModel">
        <n-form-item :label="$gettext('Name')">
          <n-input v-model:value="createModel.path" />
        </n-form-item>
      </n-form>
      <n-button type="info" block @click="handleCreate">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
  <n-modal
    v-model:show="download"
    preset="card"
    :title="$gettext('Remote Download')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-form :model="downloadModel">
        <n-form-item :label="$gettext('Download URL')">
          <n-input :input-props="{ type: 'url' }" v-model:value="downloadModel.url" />
        </n-form-item>
        <n-form-item :label="$gettext('Save as')">
          <n-input v-model:value="downloadModel.path" />
        </n-form-item>
      </n-form>
      <n-button type="info" block @click="handleDownload">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
  <upload-modal v-model:show="upload" v-model:path="path" />
  <!-- 终端弹窗 -->
  <pty-terminal-modal
    v-model:show="terminalModal"
    :title="$gettext('Terminal - %{ path }', { path })"
    :command="`cd '${path}' && exec bash`"
  />
</template>

<style scoped lang="scss"></style>

<script setup lang="ts">
import { NButton, NDataTable, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'

import { useImagePull } from './composables/useImagePull'
import ImagePullProgress from './ImagePullProgress.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const pullModel = ref({
  name: '',
  background: false,
  auth: false,
  username: '',
  password: '',
})
const pullModal = ref(false)
const pullSubmitting = ref(false)
const selectedRowKeys = ref<any>([])

const exportModal = ref(false)
const exportModel = ref({ id: '', name: '', path: '' })
const importModal = ref(false)
const importPath = ref('')
// 导出选保存目录，导入选归档文件，共用一个路径选择器
const showPathSelector = ref(false)
const selectorPath = ref('/')
const selectorDir = ref(true)

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: 'ID',
    key: 'id',
    minWidth: 400,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Container Count'),
    key: 'containers',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Image'),
    key: 'repo_tags',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return row.repo_tags.join(', ')
    },
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 200,
    resizable: true,
    render(row: any) {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 8 }, () => [
        h(
          NButton,
          { size: 'small', type: 'primary', onClick: () => openExport(row) },
          { default: () => $gettext('Export') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete?'),
              })
              if (ok) await handleDelete(row)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  container.imageList(page, pageSize),
)

const handleDelete = async (row: any) => {
  await container.imageRemove(row.id)
  refresh()
  window.$message.success($gettext('Delete successful'))
}

const handlePrune = async () => {
  await container.imagePrune()
  refresh()
  window.$message.success($gettext('Cleanup successful'))
}

const handleBulkDelete = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.imageRemove(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const openExport = (row: any) => {
  exportModel.value = { id: row.id, name: row.repo_tags.join(', ') || row.id, path: '' }
  exportModal.value = true
}

const handleExport = async () => {
  const path = await container.imageExport(exportModel.value.id, exportModel.value.path)
  exportModal.value = false
  window.$message.success(
    $gettext('Exporting to %{ path }, please check progress in background tasks', { path }),
  )
}

const openImport = () => {
  importPath.value = ''
  importModal.value = true
}

const handleImport = async () => {
  if (!importPath.value) {
    window.$message.warning($gettext('Please select the image archive'))
    return
  }
  await container.imageImport(importPath.value)
  importModal.value = false
  window.$message.success($gettext('Task submitted, please check progress in background tasks'))
}

const handleSelectPath = (dir: boolean) => {
  const current = dir ? exportModel.value.path : importPath.value
  selectorDir.value = dir
  // 选择器从目录打开，已填的文件路径取其所在目录
  selectorPath.value = (dir ? current : current.slice(0, current.lastIndexOf('/'))) || '/'
  showPathSelector.value = true
}

const handlePathSelected = (path: string) => {
  if (selectorDir.value) {
    exportModel.value.path = path
  } else {
    importPath.value = path
  }
}

const { isPulling, pullProgress, pullStatus, pullError, totalProgress, start, cancel, reset } =
  useImagePull(() => {
    pullModal.value = false
    refresh()
    window.$message.success($gettext('Pull successful'))
  })

// 取消拉取：出错后回到表单以便修改重试，拉取中则直接关窗
const cancelPull = () => {
  if (!pullError.value) {
    pullModal.value = false
  }
  reset()
}

// 拉取镜像
const handlePull = () => {
  if (!pullModel.value.name) {
    window.$message.warning($gettext('Please enter image name'))
    return
  }

  if (pullModel.value.background) {
    pullSubmitting.value = true
    useRequest(container.imagePull({ ...pullModel.value, background: true }))
      .onSuccess(() => {
        pullModal.value = false
        window.$message.success(
          $gettext('Task submitted, please check progress in background tasks'),
        )
      })
      .onComplete(() => {
        pullSubmitting.value = false
      })
    return
  }

  start(
    pullModel.value.name,
    pullModel.value.auth
      ? { username: pullModel.value.username, password: pullModel.value.password }
      : undefined,
  )
}

// 监听弹窗打开，重置状态
watch(pullModal, (val) => {
  if (val) {
    reset()
    pullModel.value.background = false
  } else {
    cancel()
  }
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex>
      <n-button type="primary" @click="pullModal = true">{{ $gettext('Pull Image') }}</n-button>
      <n-button type="primary" ghost @click="openImport">{{ $gettext('Import Image') }}</n-button>
      <app-button type="primary" ghost @click="handlePrune">
        {{ $gettext('Cleanup Images') }}
      </app-button>
      <ConfirmDialog
        type="danger"
        :content="$gettext('Are you sure you want to delete the selected images?')"
        @confirm="handleBulkDelete"
      >
        <template #trigger>
          <app-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Delete') }}
          </app-button>
        </template>
      </ConfirmDialog>
    </n-flex>
    <n-data-table
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :loading="loading"
      :scroll-x="1380"
      :data="data"
      :columns="columns"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
  <app-modal
    v-model:show="pullModal"
    :title="$gettext('Pull Image')"
    :mask-closable="!isPulling && !pullSubmitting"
    :closable="!isPulling && !pullSubmitting"
  >
    <image-pull-progress
      v-if="isPulling || pullError || pullProgress.size > 0"
      :progress="pullProgress"
      :status="pullStatus"
      :error="pullError"
      :percentage="totalProgress"
      @cancel="cancelPull"
      @retry="handlePull"
    />

    <!-- 拉取表单 -->
    <template v-else>
      <n-form :model="pullModel">
        <n-form-item path="name" :label="$gettext('Image Name')">
          <n-input
            v-model:value="pullModel.name"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('docker.io/php:8.3-fpm')"
          />
        </n-form-item>
        <n-form-item path="auth" :label="$gettext('Authentication')">
          <n-switch v-model:value="pullModel.auth" />
        </n-form-item>
        <n-form-item path="background" :label="$gettext('Execution Mode')">
          <n-radio-group v-model:value="pullModel.background">
            <n-radio-button :value="false">{{ $gettext('Foreground') }}</n-radio-button>
            <n-radio-button :value="true">{{ $gettext('Background') }}</n-radio-button>
          </n-radio-group>
        </n-form-item>
        <n-form-item v-if="pullModel.auth" path="username" :label="$gettext('Username')">
          <n-input
            v-model:value="pullModel.username"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter username')"
          />
        </n-form-item>
        <n-form-item v-if="pullModel.auth" path="password" :label="$gettext('Password')">
          <n-input
            v-model:value="pullModel.password"
            type="password"
            show-password-on="click"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
      </n-form>
      <n-button
        type="info"
        block
        :loading="pullSubmitting"
        :disabled="pullSubmitting"
        @click="handlePull"
      >
        {{ $gettext('Submit') }}
      </n-button>
    </template>
  </app-modal>
  <app-modal v-model:show="exportModal" size="sm" :title="$gettext('Export Image')">
    <n-form>
      <n-form-item :label="$gettext('Image')">
        <n-input :value="exportModel.name" readonly />
      </n-form-item>
      <n-form-item :label="$gettext('Save Directory')">
        <n-input-group>
          <n-input
            v-model:value="exportModel.path"
            :placeholder="$gettext('Leave empty to use the backup directory')"
            @keydown.enter.prevent
          />
          <n-button type="primary" ghost @click="handleSelectPath(true)">
            {{ $gettext('Select') }}
          </n-button>
        </n-input-group>
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleExport">{{ $gettext('Submit') }}</app-button>
  </app-modal>
  <app-modal v-model:show="importModal" size="sm" :title="$gettext('Import Image')">
    <n-form>
      <n-form-item :label="$gettext('Image Archive')">
        <n-input-group>
          <n-input
            v-model:value="importPath"
            placeholder="/path/to/image.tar"
            @keydown.enter.prevent
          />
          <n-button type="primary" ghost @click="handleSelectPath(false)">
            {{ $gettext('Select') }}
          </n-button>
        </n-input-group>
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleImport">{{ $gettext('Submit') }}</app-button>
  </app-modal>
  <path-selector
    v-model:show="showPathSelector"
    v-model:path="selectorPath"
    :dir="selectorDir"
    @select="handlePathSelected"
  />
</template>

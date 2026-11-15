<script setup lang="ts">
import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
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
const pruneLoading = ref(false)
const selectedRowKeys = ref<any>([])

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
    width: 120,
    hideInExcel: true,
    render(row: any) {
      return h(
        NButton,
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
      )
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  container.imageList(page, pageSize),
)

const handleDelete = async (row: any) => {
  useRequest(container.imageRemove(row.id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Delete successful'))
  })
}

const handlePrune = () => {
  pruneLoading.value = true
  useRequest(container.imagePrune())
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Cleanup successful'))
    })
    .onComplete(() => {
      pruneLoading.value = false
    })
}

const handleBulkDelete = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.imageRemove(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
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
      <n-button
        type="primary"
        ghost
        :loading="pruneLoading"
        :disabled="pruneLoading"
        @click="handlePrune"
      >
        {{ $gettext('Cleanup Images') }}
      </n-button>
      <ConfirmDialog
        type="danger"
        :content="$gettext('Are you sure you want to delete the selected images?')"
        @confirm="handleBulkDelete"
      >
        <template #trigger>
          <n-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Delete') }}
          </n-button>
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
      :scroll-x="1300"
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
</template>

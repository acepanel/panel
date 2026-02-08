<script setup lang="ts">
import backup from '@/api/panel/backup'
import storage from '@/api/panel/backup-storage'
import type { MessageReactive } from 'naive-ui'
import { NButton, NDataTable, NFlex, NInput, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import website from '@/api/panel/website'
import { formatDateTime } from '@/utils'
import UploadModal from '@/views/backup/UploadModal.vue'

const { $gettext } = useGettext()
const type = defineModel<string>('type', { type: String, required: true })

let messageReactive: MessageReactive | null = null

const uploadModal = ref(false)
const createLoading = ref(false)
const restoreLoading = ref(false)

const createModal = ref(false)
const createModel = ref({
  target: '',
  storage: 0
})

const storages = ref<any[]>([])

const restoreModal = ref(false)
const restoreModel = ref({
  file: '',
  target: ''
})

const websites = ref<any>([])

const columns: any = [
  {
    title: $gettext('Filename'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 160,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Update Date'),
    key: 'time',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.time)
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 260,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            onClick: () => {
              restoreModel.value.file = row.path
              restoreModal.value = true
            }
          },
          {
            default: () => $gettext('Restore')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleDelete(row.name)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete this backup?')
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 15px;'
                },
                {
                  default: () => $gettext('Delete')
                }
              )
            }
          }
        )
      ]
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => backup.list(type.value, page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleCreate = () => {
  createLoading.value = true
  useRequest(
    backup.create(type.value, createModel.value.target, createModel.value.storage)
  )
    .onSuccess(() => {
      createModal.value = false
      window.$bus.emit('backup:refresh')
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      createLoading.value = false
    })
}

const handleRestore = () => {
  restoreLoading.value = true
  messageReactive = window.$message.loading($gettext('Restoring...'), {
    duration: 0
  })

  useRequest(backup.restore(type.value, restoreModel.value.file, restoreModel.value.target))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Restored successfully'))
    })
    .onComplete(() => {
      messageReactive?.destroy()
      restoreLoading.value = false
    })
}

const handleDelete = async (file: string) => {
  useRequest(backup.delete(type.value, file)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

watch(
  type,
  (newType) => {
    if (newType === 'website') {
      createModel.value.target = websites.value[0]?.value || ''
      restoreModel.value.target = websites.value[0]?.value || ''
    } else {
      createModel.value.target = ''
      restoreModel.value.target = ''
    }
    refresh()
  },
  { immediate: true }
)

onMounted(() => {
  useRequest(app.isInstalled('nginx')).onSuccess(({ data }) => {
    if (data) {
      useRequest(website.list('all', 1, 10000)).onSuccess(({ data }: { data: any }) => {
        for (const item of data.items) {
          websites.value.push({
            label: item.name,
            value: item.name
          })
        }
        if (type.value === 'website') {
          createModel.value.target = websites.value[0]?.value
          restoreModel.value.target = websites.value[0]?.value
        }
      })
    }
  })
  useRequest(storage.list(1, 10000)).onSuccess(({ data }: { data: any }) => {
    for (const item of data.items) {
      storages.value.push({
        label: item.name,
        value: item.id
      })
    }
    createModel.value.storage = storages.value[0]?.value || 0
  })
  refresh()
  window.$bus.on('backup:refresh', refresh)
})

onUnmounted(() => {
  window.$bus.off('backup:refresh')
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-alert type="info">
      {{
        $gettext(
          'Only local backups are displayed here. Remote backups are stored in the corresponding backup storage.'
        )
      }}
    </n-alert>
    <n-flex>
      <n-button type="primary" @click="createModal = true">{{
        $gettext('Create Backup')
      }}</n-button>
      <n-button type="primary" @click="uploadModal = true" ghost>{{
        $gettext('Upload Backup')
      }}</n-button>
    </n-flex>
    <n-data-table
      striped
      remote
      :scroll-x="1000"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.name"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200]
      }"
    />
  </n-flex>
  <n-modal
    v-model:show="createModal"
    preset="card"
    :title="$gettext('Create Backup')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="createModal = false"
  >
    <n-form :model="createModel">
      <n-form-item v-if="type == 'website'" path="name" :label="$gettext('Website')">
        <n-select
          v-model:value="createModel.target"
          :options="websites"
          :placeholder="$gettext('Select website')"
        />
      </n-form-item>
      <n-form-item v-if="type != 'website'" path="name" :label="$gettext('Database Name')">
        <n-input
          v-model:value="createModel.target"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database name')"
        />
      </n-form-item>
      <n-form-item path="storage" :label="$gettext('Backup Storage')">
        <n-select
          v-model:value="createModel.storage"
          :options="storages"
          :placeholder="$gettext('Select backup storage')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="createLoading" :disabled="createLoading" @click="handleCreate">{{ $gettext('Submit') }}</n-button>
  </n-modal>
  <n-modal
    v-model:show="restoreModal"
    preset="card"
    :title="$gettext('Restore Backup')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="restoreModal = false"
  >
    <n-form :model="restoreModel">
      <n-form-item v-if="type == 'website'" path="name" :label="$gettext('Website')">
        <n-select
          v-model:value="restoreModel.target"
          :options="websites"
          :placeholder="$gettext('Select website')"
        />
      </n-form-item>
      <n-form-item v-if="type != 'website'" path="name" :label="$gettext('Database')">
        <n-input v-model:value="restoreModel.target" type="text" @keydown.enter.prevent />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="restoreLoading" :disabled="restoreLoading" @click="handleRestore">{{ $gettext('Submit') }}</n-button>
  </n-modal>
  <upload-modal v-model:show="uploadModal" v-model:type="type" />
</template>

<style scoped lang="scss"></style>

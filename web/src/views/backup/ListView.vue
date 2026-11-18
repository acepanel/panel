<script setup lang="ts">
import { NButton, NDataTable, NEllipsis, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import backup from '@/api/panel/backup'
import storage from '@/api/panel/backup-storage'
import database from '@/api/panel/database'
import website from '@/api/panel/website'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime, WEBSERVER_SLUGS } from '@/utils'
import UploadModal from '@/views/backup/UploadModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const type = defineModel<string>('type', { type: String, required: true })

const uploadModal = ref(false)

const createModal = ref(false)
const createModel = ref<{ target: string | null; storage: number }>({
  target: null,
  storage: 0,
})

const storages = ref<any[]>([])

const restoreModal = ref(false)
const restoreModel = ref<{ file: string; target: string | null }>({
  file: '',
  target: null,
})
const restorePoints = ref<any[]>([])

const websites = ref<any>([])
const databases = ref<any[]>([])
const databasesLoading = ref(false)

const selectedRowKeys = ref<any>([])

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      const content = row.children
        ? [
            row.name,
            ' ',
            h(NTag, { size: 'small', round: true, bordered: false }, () => row.children.length),
          ]
        : row.name
      // naive-ui 列自带的省略没扣掉展开按钮（24px）和子行缩进（16px），长名称会被挤到下一行
      return h(
        NEllipsis,
        { style: { maxWidth: `calc(100% - ${row.child ? 40 : 24}px)` } },
        () => content,
      )
    },
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 160,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.file.size
    },
  },
  {
    title: $gettext('Time'),
    key: 'time',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.file.time)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 320,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            onClick: () => {
              window.open(
                `/api/backup/${type.value}/download?file=${encodeURIComponent(row.file.name)}`,
              )
            },
          },
          { default: () => $gettext('Download') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            onClick: () => openRestore(row),
          },
          { default: () => $gettext('Restore') },
        ),
        // 分组行不放删除，免得想删最新一份却删了整组
        row.children
          ? null
          : h(
              AppButton,
              {
                size: 'small',
                type: 'error',
                onClick: async () => {
                  const ok = await confirmDelete({
                    content: $gettext('Are you sure you want to delete this backup?'),
                  })
                  if (ok) await handleDelete(row.file.name)
                },
              },
              { default: () => $gettext('Delete') },
            ),
      ])
    },
  },
]

// 分组行 key 加 / 后缀与文件名区分，文件名里不会有 /
const toRow = (group: any) => {
  const [latest] = group.items
  if (group.items.length === 1) return { key: latest.name, name: group.name, file: latest, group }
  return {
    key: `${group.name}/`,
    name: group.name,
    file: latest,
    group,
    children: group.items.map((file: any) => ({
      key: file.name,
      name: file.name,
      file,
      group,
      child: true,
    })),
  }
}

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable(
  (page, pageSize) => backup.list(type.value, page, pageSize),
  {
    data: (res: any) => res.items.map(toRow),
    watchingStates: [type],
  },
)

const openRestore = (row: any) => {
  restorePoints.value = row.group.items.map((item: any) => ({
    label: formatDateTime(item.time),
    value: item.path,
  }))
  restoreModel.value.file = row.file.path
  const targets = type.value === 'website' ? websites.value : databases.value
  if (targets.some((item: any) => item.value === row.group.name)) {
    restoreModel.value.target = row.group.name
  }
  restoreModal.value = true
}

const handleCreate = async () => {
  await backup.create(type.value, createModel.value.target, createModel.value.storage)
  createModal.value = false
  window.$bus.emit('backup:refresh')
  window.$message.success(
    $gettext('Backup task created, please check the progress in background tasks'),
  )
}

const handleRestore = async () => {
  await backup.restore(type.value, restoreModel.value.file, restoreModel.value.target)
  restoreModal.value = false
  window.$bus.emit('backup:refresh')
  window.$message.success(
    $gettext('Restore task created, please check the progress in background tasks'),
  )
}

const handleDelete = async (file: string) => {
  await backup.delete(type.value, file)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const bulkDelete = async () => {
  // 勾选分组会级联勾选其下文件，去掉分组行自身即可
  const files = selectedRowKeys.value.filter((key: string) => !key.endsWith('/'))
  const promises = files.map((file: string) => backup.delete(type.value, file))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const loadDatabases = (dbType: string) => {
  databases.value = []
  databasesLoading.value = true
  useRequest(database.list(1, 10000, dbType))
    .onSuccess(({ data }: { data: any }) => {
      for (const item of data.items) {
        databases.value.push({
          label: `${item.name} (${item.server_name || 'local'})`,
          value: item.name,
        })
      }
    })
    .onComplete(() => {
      databasesLoading.value = false
    })
}

watch(
  type,
  (newType) => {
    // 切换类型时组件不重建，不清空会把上个类型选中的文件名拿去删
    selectedRowKeys.value = []
    if (newType === 'website') {
      createModel.value.target = websites.value[0]?.value || ''
      restoreModel.value.target = websites.value[0]?.value || ''
    } else if (newType === 'redis' || newType === 'valkey') {
      // Redis/Valkey 整实例备份，无库名，target 固定为实例类型
      createModel.value.target = newType
      restoreModel.value.target = newType
    } else {
      // mysql/postgresql/clickhouse 加载数据库列表供下拉选择
      createModel.value.target = null
      restoreModel.value.target = null
      loadDatabases(newType)
    }
  },
  { immediate: true },
)

onMounted(() => {
  useRequest(app.isInstalled(WEBSERVER_SLUGS)).onSuccess(({ data }) => {
    if (data) {
      useRequest(website.list('all', 1, 10000)).onSuccess(({ data }: { data: any }) => {
        for (const item of data.items) {
          websites.value.push({
            label: item.name,
            value: item.name,
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
        value: item.id,
      })
    }
    createModel.value.storage = storages.value[0]?.value || 0
  })
})

useBus('backup:refresh', refresh)
</script>

<template>
  <n-flex vertical :size="20">
    <n-alert type="info">
      {{
        $gettext(
          'Only local backups are displayed here. Remote backups are stored in the corresponding backup storage.',
        )
      }}
    </n-alert>
    <n-flex>
      <n-button type="primary" @click="createModal = true">{{
        $gettext('Create Backup')
      }}</n-button>
      <n-button type="primary" ghost @click="uploadModal = true">
        {{ $gettext('Upload Backup') }}
      </n-button>
      <ConfirmDialog
        type="delete"
        :content="$gettext('Are you sure you want to delete the selected backups?')"
        @confirm="bulkDelete"
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
      :scroll-x="1000"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.key"
      :pagination="pagination"
    />
  </n-flex>
  <app-modal v-model:show="createModal" :title="$gettext('Create Backup')">
    <n-form :model="createModel">
      <n-form-item v-if="type == 'website'" path="name" :label="$gettext('Website')">
        <n-select
          v-model:value="createModel.target"
          :options="websites"
          :placeholder="$gettext('Select website')"
        />
      </n-form-item>
      <n-form-item
        v-if="['mysql', 'postgresql', 'clickhouse'].includes(type)"
        path="name"
        :label="$gettext('Database Name')"
      >
        <n-select
          v-model:value="createModel.target"
          :options="databases"
          :loading="databasesLoading"
          filterable
          :placeholder="$gettext('Select database')"
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
    <app-button type="info" block @click="handleCreate">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
  <app-modal v-model:show="restoreModal" :title="$gettext('Restore Backup')">
    <n-form :model="restoreModel">
      <n-form-item path="file" :label="$gettext('Time')">
        <n-select v-model:value="restoreModel.file" :options="restorePoints" />
      </n-form-item>
      <n-form-item v-if="type == 'website'" path="name" :label="$gettext('Website')">
        <n-select
          v-model:value="restoreModel.target"
          :options="websites"
          :placeholder="$gettext('Select website')"
        />
      </n-form-item>
      <n-form-item
        v-if="['mysql', 'postgresql', 'clickhouse'].includes(type)"
        path="name"
        :label="$gettext('Database')"
      >
        <n-select
          v-model:value="restoreModel.target"
          :options="databases"
          :loading="databasesLoading"
          filterable
          :placeholder="$gettext('Select database')"
        />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleRestore">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
  <upload-modal v-model:show="uploadModal" v-model:type="type" />
</template>

<style scoped lang="scss"></style>

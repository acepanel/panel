<script setup lang="ts">
import { NButton, NDataTable, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import storage from '@/api/panel/backup-storage'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'
import StorageModal from '@/views/backup/StorageModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const modalShow = ref(false)
const editItem = ref<any>(null)

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render(row: any) {
      const typeMap: Record<string, string> = {
        local: $gettext('Local'),
        s3: 'S3',
        sftp: 'SFTP',
        webdav: 'WebDAV',
      }
      return typeMap[row.type] || row.type
    },
  },
  {
    title: $gettext('Created At'),
    key: 'created_at',
    width: 180,
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
      const isLocal = row.type === 'local'
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            disabled: isLocal,
            onClick: () => handleEdit(row),
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            disabled: isLocal,
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this storage?'),
              })
              if (ok) await handleDelete(row.id)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  storage.list(page, pageSize),
)

const handleCreate = () => {
  editItem.value = null
  modalShow.value = true
}

const handleEdit = (row: any) => {
  editItem.value = row
  modalShow.value = true
}

const handleDelete = async (id: number) => {
  await storage.delete(id)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex>
      <n-button type="primary" @click="handleCreate">{{ $gettext('Add Storage') }}</n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="800"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>

  <storage-modal v-model:show="modalShow" :edit-item="editItem" @saved="refresh" />
</template>

<style scoped lang="scss"></style>

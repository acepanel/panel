<script setup lang="ts">
import { NButton, NDataTable, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import task from '@/api/panel/task'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()
const logModal = ref(false)
const logPath = ref('')
const logModalRef = ref<{ clear: () => void } | null>(null)

const handleClearLog = async () => {
  if (!logPath.value) return
  await file.truncate(logPath.value)
  logModalRef.value?.clear()
  window.$message.success($gettext('Cleared successfully'))
}

const columns: any = [
  {
    title: $gettext('Task Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 150,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.status === 'finished'
        ? $gettext('Completed')
        : row.status === 'waiting'
          ? $gettext('Waiting')
          : row.status === 'failed'
            ? $gettext('Failed')
            : row.status === 'canceled'
              ? $gettext('Canceled')
              : $gettext('Running')
    },
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Completion Time'),
    key: 'updated_at',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatDateTime(row.updated_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      const items: any[] = []
      if (row.log) {
        items.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'warning',
              secondary: true,
              onClick: () => {
                logPath.value = row.log
                logModal.value = true
              },
            },
            { default: () => $gettext('Logs') },
          ),
        )
      }
      if (row.status == 'waiting' || row.status == 'running') {
        items.push(
          h(
            AppButton,
            {
              size: 'small',
              type: 'error',
              secondary: true,
              onClick: async () => {
                const ok = await confirmAction({
                  title: $gettext('Cancel Task'),
                  content: $gettext('Are you sure you want to cancel task %{ name }?', {
                    name: row.name,
                  }),
                })
                if (ok) await handleCancel(row.id)
              },
            },
            { default: () => $gettext('Cancel') },
          ),
        )
      } else {
        items.push(
          h(
            AppButton,
            {
              size: 'small',
              type: 'error',
              onClick: async () => {
                const ok = await confirmDelete({
                  content: $gettext('Are you sure you want to delete?'),
                })
                if (ok) await handleDelete(row.id)
              },
            },
            { default: () => $gettext('Delete') },
          ),
        )
      }
      return h(NFlex, { size: 'small', align: 'center' }, () => items)
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  task.list(page, pageSize),
)

const handleDelete = async (id: number) => {
  await task.delete(id)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const handleCancel = async (id: number) => {
  await task.cancel(id)
  refresh()
  window.$message.success($gettext('Canceled successfully'))
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">{{
      $gettext('If logs cannot be loaded, please disable ad blockers!')
    }}</n-alert>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1000"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
  <realtime-log-modal
    ref="logModalRef"
    v-model:show="logModal"
    :path="logPath"
    clearable
    @clear="handleClearLog"
  />
</template>

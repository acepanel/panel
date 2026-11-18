<script setup lang="ts">
import { NDataTable, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import CreateForwardModal from '@/views/firewall/CreateForwardModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const createModalShow = ref(false)

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Transport Protocol'),
    key: 'protocol',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      return h(NTag, null, {
        default: () => {
          if (row.protocol !== '') {
            return row.protocol
          }
          return $gettext('None')
        },
      })
    },
  },
  {
    title: $gettext('Port'),
    key: 'port',
    width: 150,
    render(row: any): any {
      return h(NTag, null, {
        default: () => {
          return row.port
        },
      })
    },
  },
  {
    title: $gettext('Target IP'),
    key: 'target_ip',
    minWidth: 200,
    render(row: any): any {
      return h(
        NTag,
        {
          type: 'info',
        },
        {
          default: () => {
            return row.target_ip
          },
        },
      )
    },
  },
  {
    title: $gettext('Target Port'),
    key: 'target_port',
    width: 150,
    render(row: any): any {
      return h(
        NTag,
        {
          type: 'info',
        },
        {
          default: () => {
            return row.target_port
          },
        },
      )
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return h(
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
      )
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  firewall.forwards(page, pageSize),
)

const selectedRowKeys = ref<any>([])

const handleDelete = async (row: any) => {
  await firewall.deleteForward(row)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const deleteSelected = async () => {
  const promises = selectedRowKeys.value.map((key: any) => {
    const rule = JSON.parse(key)
    return firewall.deleteForward(rule)
  })
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

// 未勾选时同步返回，避免删除按钮闪一下加载图标
const batchDelete = () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select rules to delete'))
    return
  }
  return deleteSelected()
}

watch(createModalShow, () => {
  refresh()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex items-center>
      <n-button type="primary" @click="createModalShow = true">
        {{ $gettext('Create Forwarding') }}
      </n-button>
      <ConfirmDialog
        type="danger"
        :content="$gettext('Are you sure you want to delete the selected rules?')"
        @confirm="batchDelete"
      >
        <template #trigger>
          <app-button type="error" ghost>
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
      :row-key="(row: any) => JSON.stringify(row)"
      :pagination="pagination"
    />
  </n-flex>
  <create-forward-modal v-model:show="createModalShow" />
</template>

<style scoped lang="scss"></style>

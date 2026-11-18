<script setup lang="ts">
import { NDataTable, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import CreateIpModal from '@/views/firewall/CreateIpModal.vue'

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
    title: $gettext('Network Protocol'),
    key: 'family',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      return h(NTag, null, {
        default: () => {
          if (row.family !== '') {
            return row.family
          }
          return $gettext('None')
        },
      })
    },
  },
  {
    title: $gettext('Strategy'),
    key: 'strategy',
    width: 150,
    render(row: any): any {
      return h(
        NTag,
        {
          type:
            row.strategy === 'accept'
              ? 'success'
              : row.strategy === 'drop'
                ? 'warning'
                : row.strategy === 'reject'
                  ? 'error'
                  : 'default',
        },
        {
          default: () => {
            switch (row.strategy) {
              case 'accept':
                return $gettext('Accept')
              case 'drop':
                return $gettext('Drop')
              case 'reject':
                return $gettext('Reject')
              case 'mark':
                return $gettext('Mark')
              default:
                return $gettext('Unknown')
            }
          },
        },
      )
    },
  },
  {
    title: $gettext('Direction'),
    key: 'direction',
    width: 150,
    render(row: any): any {
      return h(
        NTag,
        {
          type: row.direction === 'in' ? 'info' : 'default',
        },
        {
          default: () => {
            switch (row.direction) {
              case 'in':
                return $gettext('Inbound')
              case 'out':
                return $gettext('Outbound')
              default:
                return $gettext('Unknown')
            }
          },
        },
      )
    },
  },
  {
    title: $gettext('IP Address'),
    key: 'address',
    minWidth: 200,
    render(row: any): any {
      return h(NTag, null, {
        default: () => {
          return row.address
        },
      })
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
  firewall.ipRules(page, pageSize),
)

const selectedRowKeys = ref<any>([])

const handleDelete = async (row: any) => {
  await firewall.deleteIpRule(row)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const deleteSelected = async () => {
  const promises = selectedRowKeys.value.map((key: any) => {
    const rule = JSON.parse(key)
    return firewall.deleteIpRule(rule)
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
        {{ $gettext('Create Rule') }}
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
      :scroll-x="1100"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => JSON.stringify(row)"
      :pagination="pagination"
    />
  </n-flex>
  <create-ip-modal v-model:show="createModalShow" />
</template>

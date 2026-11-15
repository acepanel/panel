<script setup lang="ts">
defineOptions({
  name: 'frp-visitor-view',
})

import { NButton, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

import FrpVisitorModal from './FrpVisitorModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const modalShow = ref(false)
const current = ref<any>(undefined)

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
    width: 100,
    render(row: any) {
      return h(NTag, { type: 'info', size: 'small' }, { default: () => row.type })
    },
  },
  {
    title: $gettext('Status'),
    key: 'enabled',
    width: 100,
    render(row: any) {
      return row.enabled === false
        ? h(NTag, { type: 'warning', size: 'small' }, { default: () => $gettext('Disabled') })
        : h(NTag, { type: 'success', size: 'small' }, { default: () => $gettext('Enabled') })
    },
  },
  {
    title: $gettext('Target Proxy'),
    key: 'server_name',
    minWidth: 150,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Local Listen'),
    key: 'bind',
    minWidth: 180,
    ellipsis: { tooltip: true },
    render(row: any) {
      return `${row.bind_addr || '127.0.0.1'}:${row.bind_port || '-'}`
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => {
              current.value = row
              modalShow.value = true
            },
          },
          { default: () => $gettext('Configure') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete visitor %{ name }?', {
                  name: row.name,
                }),
              })
              if (ok) handleDelete(row.name)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  frp.visitors(page, pageSize),
)

const handleDelete = (name: string) => {
  useRequest(frp.deleteVisitor(name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const handleAdd = () => {
  current.value = undefined
  modalShow.value = true
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">
      {{
        $gettext(
          'Visitors are used to access stcp, sudp and xtcp proxies. They must be configured on the client that initiates the access.',
        )
      }}
    </n-alert>
    <n-flex>
      <n-button type="primary" @click="handleAdd">
        {{ $gettext('Add Visitor') }}
      </n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1000"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.name"
      :pagination="pagination"
    />
  </n-flex>
  <frp-visitor-modal v-model:show="modalShow" :visitor="current" @submitted="refresh" />
</template>

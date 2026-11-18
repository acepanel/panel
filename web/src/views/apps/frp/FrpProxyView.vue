<script setup lang="ts">
defineOptions({
  name: 'frp-proxy-view',
})

import { NButton, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import frp from '@/api/apps/frp'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

import FrpProxyModal from './FrpProxyModal.vue'

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
    title: $gettext('Local'),
    key: 'local',
    minWidth: 180,
    ellipsis: { tooltip: true },
    render(row: any) {
      if (row.plugin?.type) {
        return $gettext('Plugin: %{ plugin }', { plugin: row.plugin.type })
      }
      return `${row.local_ip || '127.0.0.1'}:${row.local_port || '-'}`
    },
  },
  {
    title: $gettext('Remote'),
    key: 'remote',
    minWidth: 180,
    ellipsis: { tooltip: true },
    render(row: any) {
      if (['tcp', 'udp'].includes(row.type)) {
        return row.remote_port ? String(row.remote_port) : $gettext('Random')
      }
      if (row.subdomain) return row.subdomain
      if (row.custom_domains?.length) return row.custom_domains.join(', ')
      return '-'
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
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete proxy %{ name }?', {
                  name: row.name,
                }),
              })
              if (ok) await handleDelete(row.name)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  frp.proxies(page, pageSize),
)

const handleDelete = async (name: string) => {
  await frp.deleteProxy(name)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
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
          'Only proxies created by the panel under the conf.d directory are managed here. Proxies written directly in frpc.toml should be edited in the main configuration.',
        )
      }}
    </n-alert>
    <n-flex>
      <n-button type="primary" @click="handleAdd">
        {{ $gettext('Add Proxy') }}
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
  <frp-proxy-modal v-model:show="modalShow" :proxy="current" @submitted="refresh" />
</template>

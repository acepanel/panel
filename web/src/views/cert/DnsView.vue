<script setup lang="ts">
import { NButton, NDataTable, NFlex, NSpace, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import DnsModal from '@/views/cert/DnsModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const props = defineProps({
  dnsProviders: {
    type: Array<any>,
    required: true,
  },
})

const { dnsProviders } = toRefs(props)

const editModal = ref(false)
const editItem = ref<any>(null)

const columns: any = [
  {
    title: $gettext('Note Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(
        NTag,
        {
          type: 'info',
          bordered: false,
        },
        {
          default: () => {
            const provider = dnsProviders.value.find((provider: any) => provider.value === row.type)
            if (provider) {
              return provider.label
            } else {
              return $gettext('Unknown')
            }
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
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              editItem.value = row
              editModal.value = true
            },
          },
          { default: () => $gettext('Modify') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete the DNS?'),
              })
              if (!ok) return
              await cert.dnsDelete(row.id)
              refresh()
              window.$message.success($gettext('Deletion successful'))
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  cert.dns(page, pageSize),
)

useBus('cert:refresh-dns', refresh)
</script>

<template>
  <n-space vertical size="large">
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
  </n-space>
  <dns-modal v-model:show="editModal" :dns-providers="dnsProviders" :edit-item="editItem" />
</template>

<style scoped lang="scss"></style>

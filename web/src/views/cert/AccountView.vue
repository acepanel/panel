<script setup lang="ts">
import { NButton, NDataTable, NFlex, NSpace, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import AccountModal from '@/views/cert/AccountModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const props = defineProps({
  caProviders: {
    type: Array<any>,
    required: true,
  },
  algorithms: {
    type: Array<any>,
    required: true,
  },
})

const { caProviders, algorithms } = toRefs(props)

const editModal = ref(false)
const editItem = ref<any>(null)

const columns: any = [
  {
    title: $gettext('Email'),
    key: 'email',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: 'CA',
    key: 'ca',
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
            return caProviders.value?.find((item: any) => item.value === row.ca)?.label
          },
        },
      )
    },
  },
  {
    title: $gettext('Key Type'),
    key: 'key_type',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
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
                content: $gettext('Are you sure you want to delete the account?'),
              })
              if (!ok) return
              await cert.accountDelete(row.id)
              window.$message.success($gettext('Deletion successful'))
              refresh()
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  cert.accounts(page, pageSize),
)

useBus('cert:refresh-account', refresh)
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
  <account-modal
    v-model:show="editModal"
    :ca-providers="caProviders"
    :algorithms="algorithms"
    :edit-item="editItem"
  />
</template>

<style scoped lang="scss"></style>

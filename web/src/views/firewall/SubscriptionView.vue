<script setup lang="ts">
import { NButton, NDataTable, NFlex, NSwitch, NTag, NTooltip } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import CronPreview from '@/components/common/CronPreview.vue'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'
import SubscriptionModal from '@/views/firewall/SubscriptionModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const modalShow = ref(false)
const editing = ref<any>()

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 160,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render(row: any) {
      return row.type === 'allow'
        ? h(NTag, { type: 'success' }, { default: () => $gettext('Whitelist') })
        : h(NTag, { type: 'error' }, { default: () => $gettext('Blacklist') })
    },
  },
  {
    title: $gettext('List URL'),
    key: 'url',
    minWidth: 260,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Update Schedule'),
    key: 'schedule',
    width: 280,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(CronPreview, { cron: row.schedule })
    },
  },
  {
    title: $gettext('IP Ranges'),
    key: 'count',
    width: 110,
  },
  {
    title: $gettext('Last Update'),
    key: 'refreshed_at',
    width: 260,
    render(row: any) {
      const time = formatDateTime(row.refreshed_at)
      if (!row.error) return time
      // 更新失败时旧列表继续生效，原因放在提示里
      return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
        time,
        h(NTooltip, null, {
          trigger: () =>
            h(NTag, { type: 'error', size: 'small' }, { default: () => $gettext('Failed') }),
          default: () => row.error,
        }),
      ])
    },
  },
  {
    title: $gettext('Enabled'),
    key: 'status',
    width: 100,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.status,
        loading: switching.has(row.id),
        onUpdateValue: () => handleStatusChange(row),
      })
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 260,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
        h(
          AppButton,
          {
            size: 'small',
            type: 'success',
            secondary: true,
            onClick: () => handleRefresh(row),
          },
          { default: () => $gettext('Update') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              editing.value = row
              modalShow.value = true
            },
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this subscription?'),
              })
              if (ok) await handleDelete(row)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  firewall.subscriptions(page, pageSize),
)

const switching = reactive(new Set<number>())

const handleStatusChange = (row: any) => {
  const { name, type, url, schedule } = row
  switching.add(row.id)
  useRequest(
    firewall.updateSubscription(row.id, { name, type, url, schedule, status: !row.status }),
  )
    .onSuccess(() => {
      row.status = !row.status
      window.$message.success($gettext('Modified successfully'))
    })
    .onComplete(() => switching.delete(row.id))
}

const handleRefresh = async (row: any) => {
  try {
    await firewall.refreshSubscription(row.id)
    window.$message.success($gettext('Updated successfully'))
  } finally {
    // 失败原因记在订阅上，成功与否都要刷新列表
    refresh()
  }
}

const handleDelete = async (row: any) => {
  await firewall.deleteSubscription(row.id)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const handleCreate = () => {
  editing.value = undefined
  modalShow.value = true
}
</script>

<template>
  <n-flex vertical :size="20">
    <n-alert type="info" :bordered="false">
      {{
        $gettext(
          'Blacklists drop new inbound connections from the listed addresses, including those to ports published by containers. Whitelists exempt the listed addresses from blacklists without opening any port. Addresses accepted by your own port or IP rules are never blocked, nor are private and reserved addresses.',
        )
      }}
    </n-alert>
    <n-flex items-center>
      <n-button type="primary" @click="handleCreate">
        {{ $gettext('Create Subscription') }}
      </n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1550"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
  <subscription-modal v-model:show="modalShow" :subscription="editing" @saved="refresh" />
</template>

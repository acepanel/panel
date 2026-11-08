<script setup lang="ts">
defineOptions({
  name: 'stalwart-queue',
})

import { NButton, NDataTable, NFlex, NTag, NText, NTooltip } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import stalwart from '@/api/apps/stalwart'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useTablePagination } from '@/components/system/composables/useTablePagination'
import { formatBytes, formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const keyword = ref('')
const retrying = reactive(new Set<string>())

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) => stalwart.queue(page, pageSize, keyword.value),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)
const pagination = useTablePagination(page, pageSize, total)

const statusType = (status: string) => {
  switch (status) {
    case 'completed':
      return 'success'
    case 'temporary_failure':
      return 'warning'
    case 'permanent_failure':
      return 'error'
    default:
      return 'info'
  }
}

const statusText = (status: string) => {
  switch (status) {
    case 'completed':
      return $gettext('Delivered')
    case 'temporary_failure':
      return $gettext('Temporary Failure')
    case 'permanent_failure':
      return $gettext('Permanent Failure')
    default:
      return $gettext('Scheduled')
  }
}

const columns: any = [
  {
    title: $gettext('Sender'),
    key: 'return_path',
    minWidth: 220,
    resizable: true,
    ellipsis: { tooltip: true },
    render: (row: any) => row.return_path || $gettext('(bounce)'),
  },
  {
    title: $gettext('Recipients'),
    key: 'recipients',
    minWidth: 360,
    resizable: true,
    render(row: any) {
      return h(NFlex, { vertical: true, size: 'small' }, () =>
        row.recipients.map((rcpt: any) => {
          const tag = h(
            NTag,
            { size: 'small', bordered: false, type: statusType(rcpt.status) },
            { default: () => statusText(rcpt.status) },
          )
          return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
            rcpt.error ? h(NTooltip, null, { trigger: () => tag, default: () => rcpt.error }) : tag,
            h(NText, null, { default: () => rcpt.address }),
            rcpt.retry_count
              ? h(
                  NText,
                  { depth: 3 },
                  {
                    default: () =>
                      $gettext('%{ count } retries', { count: String(rcpt.retry_count) }),
                  },
                )
              : null,
          ])
        }),
      )
    },
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 100,
    render: (row: any) => formatBytes(row.size),
  },
  {
    title: $gettext('Queued At'),
    key: 'created_at',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row: any) => formatDateTime(row.created_at),
  },
  {
    title: $gettext('Next Retry'),
    key: 'next_retry',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row: any) => (row.next_retry ? formatDateTime(row.next_retry) : '-'),
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            loading: retrying.has(row.id),
            onClick: () => handleRetry(row.id),
          },
          { default: () => $gettext('Retry Now') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext(
                  'Are you sure you want to delete this message from the queue? The sender will not be notified.',
                ),
              })
              if (ok) handleDelete(row.id)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const handleRetry = (id: string) => {
  retrying.add(id)
  useRequest(stalwart.queueRetry(id))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Retry scheduled'))
    })
    .onComplete(() => retrying.delete(id))
}

const handleDelete = (id: string) => {
  useRequest(stalwart.queueDelete(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical>
    <n-flex align="center">
      <n-input
        v-model:value="keyword"
        class="w-60"
        clearable
        :placeholder="$gettext('Search sender or recipient')"
        @keyup.enter="refresh"
        @clear="refresh"
      />
      <n-button :loading="loading" :disabled="loading" @click="refresh">
        {{ $gettext('Refresh') }}
      </n-button>
    </n-flex>
    <n-alert type="info">
      {{
        $gettext(
          'Only messages waiting for delivery are listed here. Delivered mail leaves the queue immediately; failed recipients are retried automatically until they expire.',
        )
      }}
    </n-alert>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1300"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
</template>

<script setup lang="ts">
import type { DataTableSortState } from 'naive-ui'
import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import type mysql from '@/api/apps/mysql'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useTablePagination } from '@/components/system/composables/useTablePagination'

const props = defineProps<{
  api: typeof mysql
}>()

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()

const { data: processes, send: refreshProcesses } = useRequest(props.api.processes, {
  initialData: [],
})
const { data: transactions, send: refreshTransactions } = useRequest(props.api.transactions, {
  initialData: { transactions: [], lock_waits: [] },
})
const topSQLPage = ref(1)
const topSQLPageSize = ref(50)
const topSQLSort = ref<DataTableSortState | null>(null)
const {
  data: topSQL,
  loading: topSQLLoading,
  send: refreshTopSQL,
} = useRequest(
  () => {
    const order = topSQLSort.value?.order
    return props.api.topSQL({
      page: topSQLPage.value,
      limit: topSQLPageSize.value,
      sort: order ? topSQLSort.value?.columnKey : undefined,
      order: order ? (order === 'descend' ? 'desc' : 'asc') : undefined,
    })
  },
  { initialData: { supported: true, enabled: true, pending_restart: false, total: 0, items: [] } },
)
const topSQLPagination = useTablePagination(topSQLPage, topSQLPageSize, () => topSQL.value.total)
watch([topSQLPage, topSQLPageSize, topSQLSort], () => refreshTopSQL())
const handleTopSQLSort = (sorter: DataTableSortState | DataTableSortState[] | null) => {
  topSQLSort.value = Array.isArray(sorter) ? (sorter[0] ?? null) : sorter
  topSQLPage.value = 1
}

const handleKill = async (id: number, refresh: () => void) => {
  await props.api.killProcess(id)
  window.$message.success($gettext('Terminated successfully'))
  refresh()
}

const killButton = (id: number, refresh: () => void) =>
  h(
    AppButton,
    {
      size: 'small',
      type: 'error',
      onClick: async () => {
        const ok = await confirmDelete({
          title: $gettext('Confirm Terminate'),
          content: $gettext('Are you sure you want to terminate connection %{ id }?', {
            id: String(id),
          }),
          positiveText: $gettext('Terminate'),
        })
        if (ok) await handleKill(id, refresh)
      },
    },
    { default: () => $gettext('Terminate') },
  )

const processColumns: any = [
  { title: 'ID', key: 'id', width: 90, sorter: (a: any, b: any) => a.id - b.id },
  { title: $gettext('User'), key: 'user', width: 110, ellipsis: { tooltip: true } },
  { title: $gettext('Host'), key: 'host', width: 150, ellipsis: { tooltip: true } },
  { title: $gettext('Database'), key: 'db', width: 120, ellipsis: { tooltip: true } },
  { title: $gettext('Command'), key: 'command', width: 120, ellipsis: { tooltip: true } },
  {
    title: $gettext('Duration (s)'),
    key: 'time',
    width: 145,
    sorter: (a: any, b: any) => a.time - b.time,
  },
  { title: $gettext('State'), key: 'state', width: 150, ellipsis: { tooltip: true } },
  { title: 'SQL', key: 'info', minWidth: 250, ellipsis: { tooltip: true } },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 120,
    render: (row: any) => killButton(row.id, refreshProcesses),
  },
]

const transactionColumns: any = [
  { title: $gettext('Transaction ID'), key: 'id', width: 140, ellipsis: { tooltip: true } },
  { title: $gettext('Thread ID'), key: 'thread_id', width: 110 },
  {
    title: $gettext('State'),
    key: 'state',
    width: 130,
    render(row: any) {
      const type = row.state === 'LOCK WAIT' ? 'error' : 'default'
      return h(NTag, { type, size: 'small' }, { default: () => row.state })
    },
  },
  {
    title: $gettext('Duration (s)'),
    key: 'seconds',
    width: 145,
    sorter: (a: any, b: any) => a.seconds - b.seconds,
  },
  {
    title: $gettext('Rows Locked'),
    key: 'rows_locked',
    width: 130,
    sorter: (a: any, b: any) => a.rows_locked - b.rows_locked,
  },
  {
    title: $gettext('Rows Modified'),
    key: 'rows_modified',
    width: 140,
    sorter: (a: any, b: any) => a.rows_modified - b.rows_modified,
  },
  { title: 'SQL', key: 'query', minWidth: 250, ellipsis: { tooltip: true } },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 120,
    render: (row: any) => killButton(row.thread_id, refreshTransactions),
  },
]

const lockWaitColumns: any = [
  { title: $gettext('Waiting Thread'), key: 'waiting_thread_id', width: 150 },
  {
    title: $gettext('Waiting SQL'),
    key: 'waiting_query',
    minWidth: 200,
    ellipsis: { tooltip: true },
  },
  { title: $gettext('Blocking Thread'), key: 'blocking_thread_id', width: 150 },
  {
    title: $gettext('Blocking SQL'),
    key: 'blocking_query',
    minWidth: 200,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 180,
    render: (row: any) =>
      h(
        AppButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              title: $gettext('Confirm Terminate'),
              content: $gettext(
                'Are you sure you want to terminate the blocking connection %{ id }?',
                { id: String(row.blocking_thread_id) },
              ),
              positiveText: $gettext('Terminate'),
            })
            if (ok) await handleKill(row.blocking_thread_id, refreshTransactions)
          },
        },
        { default: () => $gettext('Terminate Blocker') },
      ),
  },
]

const topSQLColumns: any = [
  { title: $gettext('Database'), key: 'database', width: 120, ellipsis: { tooltip: true } },
  { title: $gettext('Calls'), key: 'calls', width: 100, sorter: true },
  {
    title: $gettext('Total Time (ms)'),
    key: 'total_ms',
    width: 150,
    sorter: true,
    defaultSortOrder: 'descend',
  },
  { title: $gettext('Mean Time (ms)'), key: 'mean_ms', width: 150, sorter: true },
  { title: $gettext('Rows Sent'), key: 'rows_sent', width: 120, sorter: true },
  { title: $gettext('Rows Examined'), key: 'rows_examined', width: 150, sorter: true },
  { title: 'SQL', key: 'query', minWidth: 300, ellipsis: { tooltip: true } },
]

const handleEnableTopSQL = async () => {
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Enable'),
    content: $gettext(
      'This will enable performance_schema, which increases memory usage (use with caution on low-memory servers). A restart is required to take effect.',
    ),
  })
  if (!ok) return
  await props.api.enableTopSQL()
  window.$message.success($gettext('Enabled, please restart the service to take effect'))
  refreshTopSQL()
}

const handleResetTopSQL = async () => {
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Reset'),
    content: $gettext('Are you sure you want to reset all SQL statistics?'),
  })
  if (!ok) return
  await props.api.resetTopSQL()
  window.$message.success($gettext('Reset successfully'))
  topSQLPage.value = 1
  refreshTopSQL()
}
</script>

<template>
  <n-tabs type="segment" animated>
    <n-tab-pane name="processes" :tab="$gettext('Processes')">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshProcesses()">
            {{ $gettext('Refresh') }}
          </app-button>
        </n-flex>
        <n-data-table
          striped
          :columns="processColumns"
          :data="processes"
          :scroll-x="1365"
          max-height="60vh"
        />
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="transactions" :tab="$gettext('Transactions & Locks')">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshTransactions()">
            {{ $gettext('Refresh') }}
          </app-button>
        </n-flex>
        <n-alert v-if="transactions.lock_waits.length" type="warning">
          {{ $gettext('Lock waits detected, check the blocking connections below.') }}
        </n-alert>
        <n-data-table
          v-if="transactions.lock_waits.length"
          striped
          :columns="lockWaitColumns"
          :data="transactions.lock_waits"
          :scroll-x="1020"
        />
        <n-data-table
          striped
          :columns="transactionColumns"
          :data="transactions.transactions"
          :scroll-x="1315"
          max-height="50vh"
        />
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="top-sql" :tab="'Top SQL'">
      <n-flex vertical>
        <n-alert v-if="!topSQL.supported" type="warning">
          {{
            $gettext(
              'This instance was built without performance_schema support, SQL statistics are not available.',
            )
          }}
        </n-alert>
        <n-alert v-else-if="!topSQL.enabled && topSQL.pending_restart" type="warning">
          {{ $gettext('performance_schema is configured, restart the service to take effect.') }}
        </n-alert>
        <template v-else-if="!topSQL.enabled">
          <n-alert type="info">
            {{
              $gettext(
                'performance_schema is not enabled. After enabling, SQL performance statistics will be collected, which increases memory usage and requires a restart.',
              )
            }}
          </n-alert>
          <n-flex>
            <app-button type="primary" @click="handleEnableTopSQL">
              {{ $gettext('Enable') }}
            </app-button>
          </n-flex>
        </template>
        <template v-if="topSQL.enabled">
          <n-flex>
            <app-button type="primary" @click="() => refreshTopSQL()">
              {{ $gettext('Refresh') }}
            </app-button>
            <app-button type="warning" @click="handleResetTopSQL">
              {{ $gettext('Reset Statistics') }}
            </app-button>
          </n-flex>
          <n-data-table
            v-model:page="topSQLPage"
            v-model:page-size="topSQLPageSize"
            striped
            remote
            :loading="topSQLLoading"
            :columns="topSQLColumns"
            :data="topSQL.items"
            :scroll-x="1365"
            max-height="60vh"
            :pagination="topSQLPagination"
            @update:sorter="handleTopSQLSort"
          />
        </template>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

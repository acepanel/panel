<script setup lang="ts">
defineOptions({
  name: 'postgresql-performance',
})

import type { DataTableSortState } from 'naive-ui'
import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useTablePagination } from '@/components/system/composables/useTablePagination'

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()

const { data: sessions, send: refreshSessions } = useRequest(postgresql.sessions, {
  initialData: [],
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
    return postgresql.topSQL({
      page: topSQLPage.value,
      limit: topSQLPageSize.value,
      sort: order ? topSQLSort.value?.columnKey : undefined,
      order: order ? (order === 'descend' ? 'desc' : 'asc') : undefined,
    })
  },
  { initialData: { enabled: true, pending_restart: false, total: 0, items: [] } },
)
const topSQLPagination = useTablePagination(topSQLPage, topSQLPageSize, () => topSQL.value.total)
watch([topSQLPage, topSQLPageSize, topSQLSort], () => refreshTopSQL())
const handleTopSQLSort = (sorter: DataTableSortState | DataTableSortState[] | null) => {
  topSQLSort.value = Array.isArray(sorter) ? (sorter[0] ?? null) : sorter
  topSQLPage.value = 1
}

const formatDuration = (seconds: number) => {
  if (!seconds || seconds <= 0) return '-'
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h${Math.floor((seconds % 3600) / 60)}m`
}

const sessionColumns: any = [
  { title: 'PID', key: 'pid', width: 90, sorter: (a: any, b: any) => a.pid - b.pid },
  { title: $gettext('Database'), key: 'database', width: 120, ellipsis: { tooltip: true } },
  { title: $gettext('User'), key: 'user', width: 120, ellipsis: { tooltip: true } },
  { title: $gettext('Client'), key: 'client_addr', width: 140, ellipsis: { tooltip: true } },
  { title: $gettext('State'), key: 'state', width: 100, ellipsis: { tooltip: true } },
  {
    title: $gettext('Wait Event'),
    key: 'wait_event',
    width: 160,
    ellipsis: { tooltip: true },
    render(row: any) {
      if (!row.wait_event) return '-'
      return `${row.wait_event_type}: ${row.wait_event}`
    },
  },
  {
    title: $gettext('Blocked By'),
    key: 'blocked_by',
    width: 110,
    render(row: any) {
      if (!row.blocked_by) return '-'
      return h(NTag, { type: 'error', size: 'small' }, { default: () => row.blocked_by })
    },
  },
  {
    title: $gettext('Transaction Duration'),
    key: 'xact_seconds',
    width: 190,
    sorter: (a: any, b: any) => a.xact_seconds - b.xact_seconds,
    render: (row: any) => formatDuration(row.xact_seconds),
  },
  {
    title: $gettext('Query Duration'),
    key: 'query_seconds',
    width: 150,
    sorter: (a: any, b: any) => a.query_seconds - b.query_seconds,
    render: (row: any) => formatDuration(row.query_seconds),
  },
  { title: 'SQL', key: 'query', minWidth: 250, ellipsis: { tooltip: true } },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 120,
    render(row: any) {
      return h(
        AppButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              title: $gettext('Confirm Terminate'),
              content: $gettext('Are you sure you want to terminate session %{ pid }?', {
                pid: String(row.pid),
              }),
              positiveText: $gettext('Terminate'),
            })
            if (ok) await handleTerminate(row.pid)
          },
        },
        { default: () => $gettext('Terminate') },
      )
    },
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
  { title: $gettext('Rows'), key: 'rows', width: 100, sorter: true },
  {
    title: $gettext('Cache Hit Rate'),
    key: 'hit_rate',
    width: 140,
    sorter: true,
    render: (row: any) => `${row.hit_rate}%`,
  },
  { title: 'SQL', key: 'query', minWidth: 300, ellipsis: { tooltip: true } },
]

const handleTerminate = async (pid: number) => {
  await postgresql.terminateSession(pid)
  window.$message.success($gettext('Terminated successfully'))
  refreshSessions()
}

const handleEnableTopSQL = async () => {
  const ok = await confirmAction({
    type: 'info',
    title: $gettext('Confirm Enable'),
    content: $gettext(
      'This will add pg_stat_statements to shared_preload_libraries, a restart of PostgreSQL is required to take effect.',
    ),
  })
  if (!ok) return
  await postgresql.enableTopSQL()
  window.$message.success($gettext('Enabled, please restart PostgreSQL to take effect'))
  refreshTopSQL()
}

const handleResetTopSQL = async () => {
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Reset'),
    content: $gettext('Are you sure you want to reset all SQL statistics?'),
  })
  if (!ok) return
  await postgresql.resetTopSQL()
  window.$message.success($gettext('Reset successfully'))
  topSQLPage.value = 1
  refreshTopSQL()
}
</script>

<template>
  <n-tabs type="segment" animated>
    <n-tab-pane name="sessions" :tab="$gettext('Sessions')">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshSessions()">
            {{ $gettext('Refresh') }}
          </app-button>
        </n-flex>
        <n-data-table
          striped
          :columns="sessionColumns"
          :data="sessions"
          :scroll-x="1720"
          max-height="60vh"
        />
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="top-sql" :tab="'Top SQL'">
      <n-flex vertical>
        <n-alert v-if="!topSQL.enabled && topSQL.pending_restart" type="warning">
          {{ $gettext('pg_stat_statements is configured, restart PostgreSQL to take effect.') }}
        </n-alert>
        <template v-else-if="!topSQL.enabled">
          <n-alert type="info">
            {{
              $gettext(
                'pg_stat_statements is not enabled. After enabling, SQL performance statistics will be collected, a restart of PostgreSQL is required.',
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
            :scroll-x="1300"
            max-height="60vh"
            :pagination="topSQLPagination"
            @update:sorter="handleTopSQLSort"
          />
        </template>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

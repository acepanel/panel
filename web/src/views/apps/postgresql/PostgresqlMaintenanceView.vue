<script setup lang="ts">
defineOptions({
  name: 'postgresql-maintenance',
})

import type { DataTableSortState } from 'naive-ui'
import { NButton, NSpace, NTag, NTooltip } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useTablePagination } from '@/components/system/composables/useTablePagination'

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()

const selectedDatabase = ref('')

const { data: databases } = useRequest(postgresql.databases, {
  initialData: [],
}).onSuccess(({ data }) => {
  if (data?.length && !selectedDatabase.value) {
    // 默认选中 postgres 库
    selectedDatabase.value = data.includes('postgres') ? 'postgres' : data[0]
  }
})

const databaseOptions = computed(() =>
  (databases.value as string[]).map((name) => ({ label: name, value: name })),
)

const bloatPage = ref(1)
const bloatPageSize = ref(50)
const bloatSort = ref<DataTableSortState | null>(null)
const {
  data: bloat,
  loading: bloatLoading,
  send: sendBloat,
} = useRequest(
  () => {
    const order = bloatSort.value?.order
    return postgresql.bloat({
      database: selectedDatabase.value,
      page: bloatPage.value,
      limit: bloatPageSize.value,
      sort: order ? bloatSort.value?.columnKey : undefined,
      order: order ? (order === 'descend' ? 'desc' : 'asc') : undefined,
    })
  },
  {
    immediate: false,
    initialData: { repack_installed: false, total: 0, items: [] },
  },
)
const bloatPagination = useTablePagination(
  bloatPage,
  bloatPageSize,
  () => bloat.value.total,
  [50, 100, 200, 500],
)
watch([selectedDatabase, bloatPage, bloatPageSize, bloatSort], () => sendBloat())
const handleBloatSort = (sorter: DataTableSortState | DataTableSortState[] | null) => {
  bloatSort.value = Array.isArray(sorter) ? (sorter[0] ?? null) : sorter
  bloatPage.value = 1
}
// 勾选跨页保留，换库或刷新时清空
const checkedTables = ref<string[]>([])
const handleDatabaseChange = () => {
  checkedTables.value = []
  bloatPage.value = 1
}
// 未选数据库时同步返回，避免刷新按钮闪一下加载图标
const refreshBloat = () => {
  checkedTables.value = []
  if (selectedDatabase.value) return sendBloat()
}
const bloatRowKey = (row: any) => `${row.schema}|${row.table}`

const { data: wal, send: refreshWal } = useRequest(postgresql.wal, {
  initialData: {
    wal_size: '-',
    archiver: { archived_count: 0, failed_count: 0, last_archived_wal: '', last_failed_wal: '' },
    slots: [],
    replications: [],
  },
})

const handleMaintenance = async (
  tables: { schema: string; table: string }[],
  operation: string,
) => {
  const target =
    tables.length === 1
      ? `${tables[0]!.schema}.${tables[0]!.table}`
      : $gettext('%{ count } selected tables', { count: String(tables.length) })
  const content =
    operation === 'vacuum_full'
      ? $gettext(
          'VACUUM FULL rewrites the entire table and holds an exclusive lock, blocking all reads and writes until it finishes. It may take a long time for large tables. Are you sure you want to run it on %{ table }?',
          { table: target },
        )
      : $gettext('Are you sure you want to run %{ op } on %{ table }?', {
          op: operation.replace('_', ' ').toUpperCase(),
          table: target,
        })
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Operation'),
    content,
  })
  if (!ok) return
  await postgresql.runMaintenance({
    database: selectedDatabase.value,
    tables,
    operation,
  })
  window.$message.success($gettext('Task submitted, please check progress in background tasks'))
}

const handleBatchMaintenance = (operation: string) => {
  const tables = checkedTables.value.map((key) => {
    const [schema, table] = key.split('|')
    return { schema: schema!, table: table! }
  })
  return handleMaintenance(tables, operation)
}

const bloatColumns: any = [
  { type: 'selection' },
  { title: $gettext('Schema'), key: 'schema', width: 110, ellipsis: { tooltip: true } },
  { title: $gettext('Table'), key: 'table', minWidth: 150, ellipsis: { tooltip: true } },
  { title: $gettext('Size'), key: 'size', width: 100, sorter: true },
  { title: $gettext('Live Tuples'), key: 'live_tuples', width: 120, sorter: true },
  {
    title: $gettext('Dead Tuples'),
    key: 'dead_tuples',
    width: 130,
    sorter: true,
    defaultSortOrder: 'descend',
  },
  {
    title: $gettext('Dead Rate'),
    key: 'dead_rate',
    width: 120,
    sorter: true,
    render(row: any) {
      const type = row.dead_rate >= 20 ? 'error' : row.dead_rate >= 10 ? 'warning' : 'default'
      return h(NTag, { type, size: 'small' }, { default: () => `${row.dead_rate}%` })
    },
  },
  {
    title: $gettext('Last Vacuum'),
    key: 'last_vacuum',
    width: 150,
    sorter: true,
    render: (row: any) => row.last_vacuum || row.last_autovacuum || '-',
  },
  {
    title: $gettext('Last Analyze'),
    key: 'last_analyze',
    width: 150,
    sorter: true,
    render: (row: any) => row.last_analyze || row.last_autoanalyze || '-',
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 430,
    render(row: any) {
      const rowTables = [{ schema: row.schema, table: row.table }]
      const buttons = [
        h(
          AppButton,
          { size: 'small', type: 'info', onClick: () => handleMaintenance(rowTables, 'vacuum') },
          { default: () => 'VACUUM' },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: () => handleMaintenance(rowTables, 'vacuum_full'),
          },
          { default: () => 'VACUUM FULL' },
        ),
        h(
          AppButton,
          { size: 'small', onClick: () => handleMaintenance(rowTables, 'analyze') },
          { default: () => 'ANALYZE' },
        ),
      ]
      if (bloat.value.repack_installed) {
        buttons.push(
          h(
            AppButton,
            {
              size: 'small',
              type: 'warning',
              onClick: () => handleMaintenance(rowTables, 'repack'),
            },
            { default: () => 'REPACK' },
          ),
        )
      } else {
        buttons.push(
          h(
            NTooltip,
            {},
            {
              trigger: () =>
                h(NButton, { size: 'small', disabled: true }, { default: () => 'REPACK' }),
              default: () =>
                $gettext('pg_repack is not installed, please install it in the extensions tab'),
            },
          ),
        )
      }
      return h(NSpace, { size: 'small', wrap: false }, { default: () => buttons })
    },
  },
]

const slotColumns: any = [
  { title: $gettext('Slot Name'), key: 'name', minWidth: 180, ellipsis: { tooltip: true } },
  { title: $gettext('Type'), key: 'type', width: 110 },
  {
    title: $gettext('Active'),
    key: 'active',
    width: 100,
    render(row: any) {
      return row.active
        ? h(NTag, { type: 'success', size: 'small' }, { default: () => $gettext('Yes') })
        : h(NTag, { type: 'warning', size: 'small' }, { default: () => $gettext('No') })
    },
  },
  { title: $gettext('Retained WAL'), key: 'retained_wal', width: 150 },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 100,
    render(row: any) {
      if (row.active) return '-'
      return h(
        AppButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              title: $gettext('Confirm Delete'),
              content: $gettext(
                'After deleting slot %{ name }, the corresponding subscription or standby will not be able to continue syncing. Are you sure?',
                { name: row.name },
              ),
              positiveText: $gettext('Delete'),
              countdown: 5,
            })
            if (ok) await handleDropSlot(row.name)
          },
        },
        { default: () => $gettext('Delete') },
      )
    },
  },
]

const replicationColumns: any = [
  { title: $gettext('Client'), key: 'client_addr', minWidth: 150 },
  { title: $gettext('State'), key: 'state', width: 120 },
  { title: $gettext('Sync State'), key: 'sync_state', width: 120 },
  { title: $gettext('Lag'), key: 'lag', width: 120 },
]

const handleDropSlot = async (name: string) => {
  await postgresql.dropReplicationSlot(name)
  window.$message.success($gettext('Deleted successfully'))
  refreshWal()
}
</script>

<template>
  <n-tabs type="segment" animated>
    <n-tab-pane name="bloat" :tab="$gettext('Table Bloat')">
      <n-flex vertical>
        <n-flex>
          <n-select
            v-model:value="selectedDatabase"
            :options="databaseOptions"
            class="md:!w-60"
            @update:value="handleDatabaseChange"
          />
          <app-button type="primary" @click="refreshBloat">
            {{ $gettext('Refresh') }}
          </app-button>
          <app-button
            type="info"
            :disabled="!checkedTables.length"
            @click="handleBatchMaintenance('vacuum')"
          >
            VACUUM
          </app-button>
          <app-button
            type="error"
            :disabled="!checkedTables.length"
            @click="handleBatchMaintenance('vacuum_full')"
          >
            VACUUM FULL
          </app-button>
          <app-button :disabled="!checkedTables.length" @click="handleBatchMaintenance('analyze')">
            ANALYZE
          </app-button>
          <app-button
            v-if="bloat.repack_installed"
            type="warning"
            :disabled="!checkedTables.length"
            @click="handleBatchMaintenance('repack')"
          >
            REPACK
          </app-button>
        </n-flex>
        <n-data-table
          v-model:checked-row-keys="checkedTables"
          v-model:page="bloatPage"
          v-model:page-size="bloatPageSize"
          striped
          remote
          :loading="bloatLoading"
          :columns="bloatColumns"
          :data="bloat.items"
          :row-key="bloatRowKey"
          :scroll-x="1530"
          max-height="60vh"
          virtual-scroll
          :pagination="bloatPagination"
          @update:sorter="handleBloatSort"
        />
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="wal" tab="WAL">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshWal()">
            {{ $gettext('Refresh') }}
          </app-button>
        </n-flex>
        <n-card :title="$gettext('WAL Status')">
          <n-flex>
            <n-statistic :label="$gettext('WAL Size')" :value="wal.wal_size" />
            <n-statistic
              class="ml-4 md:ml-40"
              :label="$gettext('Archived Count')"
              :value="wal.archiver.archived_count"
            />
            <n-statistic
              class="ml-4 md:ml-40"
              :label="$gettext('Archive Failed Count')"
              :value="wal.archiver.failed_count"
            />
          </n-flex>
        </n-card>
        <n-card :title="$gettext('Replication Slots')">
          <n-data-table striped :columns="slotColumns" :data="wal.slots" :scroll-x="680" />
        </n-card>
        <n-card v-if="wal.replications.length" :title="$gettext('Replication Status')">
          <n-data-table
            striped
            :columns="replicationColumns"
            :data="wal.replications"
            :scroll-x="510"
          />
        </n-card>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

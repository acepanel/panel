<script setup lang="ts">
import type { DataTableSortState } from 'naive-ui'
import { NSpace, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import type mysql from '@/api/apps/mysql'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { useTablePagination } from '@/components/system/composables/useTablePagination'

const props = defineProps<{
  api: typeof mysql
}>()

const { $gettext } = useGettext()
const { isMobile } = useResponsive()
const { confirmDelete, confirmAction } = useConfirm()

const selectedDatabase = ref('')
const { data: databases } = useRequest(props.api.databases, {
  initialData: [],
})
const databaseOptions = computed(() => [
  { label: $gettext('All databases'), value: '' },
  ...(databases.value as string[]).map((name) => ({ label: name, value: name })),
])
const tablesPage = ref(1)
const tablesPageSize = ref(50)
const tablesSort = ref<DataTableSortState | null>(null)
const {
  data: tables,
  loading: tablesLoading,
  send: sendTables,
} = useRequest(
  () => {
    const order = tablesSort.value?.order
    return props.api.tables({
      database: selectedDatabase.value,
      page: tablesPage.value,
      limit: tablesPageSize.value,
      sort: order ? tablesSort.value?.columnKey : undefined,
      order: order ? (order === 'descend' ? 'desc' : 'asc') : undefined,
    })
  },
  { initialData: { total: 0, items: [] } },
)
const tablesPagination = useTablePagination(
  tablesPage,
  tablesPageSize,
  () => tables.value.total,
  [50, 100, 200, 500],
)
watch([selectedDatabase, tablesPage, tablesPageSize, tablesSort], () => sendTables())
const handleTablesSort = (sorter: DataTableSortState | DataTableSortState[] | null) => {
  tablesSort.value = Array.isArray(sorter) ? (sorter[0] ?? null) : sorter
  tablesPage.value = 1
}
// 勾选跨页保留，换库或刷新时清空
const checkedTables = ref<string[]>([])
const handleDatabaseChange = () => {
  checkedTables.value = []
  tablesPage.value = 1
}
const refreshTables = () => {
  checkedTables.value = []
  return sendTables()
}
const tableRowKey = (row: any) => `${row.database}|${row.table}`
const { data: binlog, send: refreshBinlogs } = useRequest(props.api.binlogs, {
  initialData: { enabled: true, total_size: '-', items: [] },
})
const { data: replication, send: refreshReplication } = useRequest(props.api.replication, {
  initialData: { enabled: false },
})

const handleMaintenance = async (
  batch: { database: string; table: string }[],
  operation: string,
) => {
  const target =
    batch.length === 1
      ? `${batch[0]!.database}.${batch[0]!.table}`
      : $gettext('%{ count } selected tables', { count: String(batch.length) })
  const content =
    operation === 'optimize'
      ? $gettext(
          'OPTIMIZE will rebuild the InnoDB table, which may take a long time for large tables. Are you sure you want to run it on %{ table }?',
          { table: target },
        )
      : $gettext('Are you sure you want to run %{ op } on %{ table }?', {
          op: operation.toUpperCase(),
          table: target,
        })
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Operation'),
    content,
  })
  if (!ok) return
  await props.api.runMaintenance({
    tables: batch,
    operation,
  })
  window.$message.success($gettext('Task submitted, please check progress in background tasks'))
}

const handleBatchMaintenance = (operation: string) => {
  const batch = checkedTables.value.map((key) => {
    const [database, table] = key.split('|')
    return { database: database!, table: table! }
  })
  return handleMaintenance(batch, operation)
}

const tableColumns: any = [
  { type: 'selection' },
  { title: $gettext('Database'), key: 'database', width: 130, ellipsis: { tooltip: true } },
  { title: $gettext('Table'), key: 'table', minWidth: 150, ellipsis: { tooltip: true } },
  { title: $gettext('Engine'), key: 'engine', width: 100 },
  { title: $gettext('Rows'), key: 'rows', width: 110, sorter: true },
  { title: $gettext('Size'), key: 'size', width: 100, sorter: true, defaultSortOrder: 'descend' },
  {
    title: $gettext('Fragment Rate'),
    key: 'fragment_rate',
    width: 130,
    sorter: true,
    render(row: any) {
      const rate = Math.round(row.fragment_rate * 10) / 10
      const type = rate >= 30 ? 'error' : rate >= 10 ? 'warning' : 'default'
      return h(NTag, { type, size: 'small' }, { default: () => `${rate}%` })
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 230,
    render(row: any) {
      const rowTables = [{ database: row.database, table: row.table }]
      return h(
        NSpace,
        { size: 'small', wrap: false },
        {
          default: () => [
            h(
              AppButton,
              {
                size: 'small',
                type: 'warning',
                onClick: () => handleMaintenance(rowTables, 'optimize'),
              },
              { default: () => 'OPTIMIZE' },
            ),
            h(
              AppButton,
              { size: 'small', onClick: () => handleMaintenance(rowTables, 'analyze') },
              { default: () => 'ANALYZE' },
            ),
          ],
        },
      )
    },
  },
]

const binlogColumns: any = [
  { title: $gettext('File'), key: 'name', minWidth: 200 },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 120,
    sorter: (a: any, b: any) => a.size_bytes - b.size_bytes,
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 160,
    render(row: any) {
      return h(
        AppButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              title: $gettext('Confirm Purge'),
              content: $gettext(
                'This will delete all binlogs before %{ file }. If a replica has not applied them yet, replication will break. Are you sure?',
                { file: row.name },
              ),
              positiveText: $gettext('Purge'),
              countdown: 5,
            })
            if (ok) await handlePurge(row.name)
          },
        },
        { default: () => $gettext('Purge to Here') },
      )
    },
  },
]

const handlePurge = async (file: string) => {
  await props.api.purgeBinlog(file)
  window.$message.success($gettext('Purged successfully'))
  refreshBinlogs()
}

const replicationRunning = (value: string) => {
  return value === 'Yes'
    ? h(NTag, { type: 'success', size: 'small' }, { default: () => $gettext('Running') })
    : h(NTag, { type: 'error', size: 'small' }, { default: () => value || $gettext('Stopped') })
}
</script>

<template>
  <n-tabs type="segment" animated>
    <n-tab-pane name="tables" :tab="$gettext('Table Maintenance')">
      <n-flex vertical>
        <n-flex>
          <n-select
            v-model:value="selectedDatabase"
            :options="databaseOptions"
            class="md:!w-60"
            @update:value="handleDatabaseChange"
          />
          <app-button type="primary" @click="() => refreshTables()">
            {{ $gettext('Refresh') }}
          </app-button>
          <app-button
            type="warning"
            :disabled="!checkedTables.length"
            @click="handleBatchMaintenance('optimize')"
          >
            OPTIMIZE
          </app-button>
          <app-button :disabled="!checkedTables.length" @click="handleBatchMaintenance('analyze')">
            ANALYZE
          </app-button>
        </n-flex>
        <n-data-table
          v-model:checked-row-keys="checkedTables"
          v-model:page="tablesPage"
          v-model:page-size="tablesPageSize"
          striped
          remote
          :loading="tablesLoading"
          :columns="tableColumns"
          :data="tables.items"
          :row-key="tableRowKey"
          :scroll-x="1030"
          max-height="60vh"
          virtual-scroll
          :pagination="tablesPagination"
          @update:sorter="handleTablesSort"
        />
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="binlog" :tab="'Binlog'">
      <n-flex vertical>
        <n-alert v-if="!binlog.enabled" type="info">
          {{ $gettext('Binary log is not enabled.') }}
        </n-alert>
        <template v-else>
          <n-flex>
            <app-button type="primary" @click="() => refreshBinlogs()">
              {{ $gettext('Refresh') }}
            </app-button>
          </n-flex>
          <n-card>
            <n-flex>
              <n-statistic :label="$gettext('File Count')" :value="binlog.items.length" />
              <n-statistic
                class="md:ml-40"
                :label="$gettext('Total Size')"
                :value="binlog.total_size"
              />
            </n-flex>
          </n-card>
          <n-data-table
            striped
            :columns="binlogColumns"
            :data="binlog.items"
            :scroll-x="520"
            max-height="50vh"
          />
        </template>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="replication" :tab="$gettext('Replication')">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshReplication()">
            {{ $gettext('Refresh') }}
          </app-button>
        </n-flex>
        <n-alert v-if="!replication.enabled" type="info">
          {{ $gettext('This instance is not a replica.') }}
        </n-alert>
        <n-card v-else :title="$gettext('Replication Status')">
          <n-descriptions label-placement="left" :column="isMobile ? 1 : 2">
            <n-descriptions-item :label="$gettext('Source Host')">
              {{ replication.source_host || '-' }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Delay (s)')">
              {{ replication.seconds_behind || '-' }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('IO Thread')">
              <component :is="replicationRunning(replication.io_running)" />
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('SQL Thread')">
              <component :is="replicationRunning(replication.sql_running)" />
            </n-descriptions-item>
            <n-descriptions-item
              v-if="replication.last_error"
              :label="$gettext('Last Error')"
              :span="2"
            >
              {{ replication.last_error }}
            </n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>

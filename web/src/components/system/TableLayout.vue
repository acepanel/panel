<script lang="ts" setup generic="T extends Record<string, any>">
import type { DataTableColumn, DataTableRowKey } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import BatchActions from './BatchActions.vue'
import { useResponsive } from './composables/useResponsive'
import { useTableQuery } from './composables/useTableQuery'
import EmptyState from './EmptyState.vue'
import SearchToolbar from './SearchToolbar.vue'
import type { BatchAction, FetchFn, SearchField, ToolbarAction } from './types'

interface Props {
  columns: DataTableColumn<T>[]
  fetch: FetchFn<T>
  rowKey: (row: T) => DataTableRowKey
  scrollX?: number
  searchFields?: SearchField[]
  pageSizes?: number[]
  initialPageSize?: number
  selectable?: boolean
  batchActions?: BatchAction[]
  primaryAction?: ToolbarAction
  toolbarActions?: ToolbarAction[]
  refreshEvent?: string
  mobileCard?: (row: T) => any
  empty?: { title?: string; description?: string; icon?: string }
  immediate?: boolean
  size?: 'small' | 'medium' | 'large'
  striped?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  scrollX: 1200,
  searchFields: () => [],
  pageSizes: () => [20, 50, 100, 200],
  initialPageSize: 20,
  selectable: false,
  batchActions: () => [],
  primaryAction: undefined,
  toolbarActions: () => [],
  refreshEvent: undefined,
  mobileCard: undefined,
  empty: () => ({}),
  immediate: true,
  size: 'small',
  striped: true,
})

const emit = defineEmits<{
  (e: 'batch', key: string, rows: T[]): void
  (e: 'refresh'): void
  (e: 'row-click', row: T): void
}>()

const { $gettext } = useGettext()
const { isMobile } = useResponsive()
const checkedRowKeys = ref<DataTableRowKey[]>([])

const tableQuery = useTableQuery<T>({
  fetch: props.fetch,
  initialPageSize: props.initialPageSize,
  immediate: props.immediate,
})

const selectedRows = computed<T[]>(() => {
  const keys = new Set(checkedRowKeys.value)
  return tableQuery.items.value.filter((row) => keys.has(props.rowKey(row)))
})

const computedColumns = computed<DataTableColumn<T>[]>(() => {
  const cols: DataTableColumn<T>[] = []
  if (props.selectable) {
    cols.push({ type: 'selection', fixed: 'left' })
  }
  return cols.concat(props.columns)
})

const pagination = computed<any>(() => ({
  page: tableQuery.page.value,
  pageSize: tableQuery.pageSize.value,
  itemCount: tableQuery.total.value,
  pageSizes: props.pageSizes,
  showSizePicker: true,
  showQuickJumper: !isMobile.value,
  prefix: (info: { itemCount?: number }) =>
    $gettext('Total: %{n}', { n: String(info?.itemCount ?? 0) }),
  onUpdatePage: (p: number) => tableQuery.setPage(p),
  onUpdatePageSize: (s: number) => tableQuery.setPageSize(s),
}))

const handleSearch = (params: Record<string, any>) => {
  tableQuery.setQuery(params)
}

const handleRefresh = async () => {
  await tableQuery.refresh()
  emit('refresh')
}

const handleBatch = (key: string) => {
  emit('batch', key, selectedRows.value)
}

const clearSelection = () => {
  checkedRowKeys.value = []
}

if (props.refreshEvent) {
  const evt = props.refreshEvent
  const handler = () => tableQuery.refresh()
  window.$bus?.on?.(evt, handler)
  onUnmounted(() => window.$bus?.off?.(evt, handler))
}

defineExpose({
  refresh: handleRefresh,
  selected: selectedRows,
  clearSelection,
  query: tableQuery,
})
</script>

<template>
  <div class="flex flex-col flex-1 gap-3 min-h-0">
    <SearchToolbar
      v-if="searchFields.length || primaryAction || toolbarActions.length"
      :search-fields="searchFields"
      :primary-action="primaryAction"
      :toolbar-actions="toolbarActions"
      :loading="tableQuery.loading.value"
      @search="handleSearch"
      @refresh="handleRefresh"
    >
      <template v-if="$slots['toolbar-prefix']" #prefix>
        <slot name="toolbar-prefix" />
      </template>
      <template v-if="$slots['toolbar-suffix']" #suffix>
        <slot name="toolbar-suffix" />
      </template>
    </SearchToolbar>

    <BatchActions
      v-if="selectable && batchActions.length"
      :selected="selectedRows"
      :actions="batchActions"
      sticky
      @action="handleBatch"
      @clear="clearSelection"
    />

    <template v-if="isMobile && mobileCard">
      <div class="flex flex-col gap-2">
        <div
          v-for="row in tableQuery.items.value"
          :key="String(rowKey(row))"
          class="section-card cursor-pointer"
          @click="emit('row-click', row)"
        >
          <component :is="mobileCard(row)" />
        </div>
        <EmptyState
          v-if="!tableQuery.loading.value && tableQuery.items.value.length === 0"
          :title="empty.title"
          :description="empty.description"
          :icon="empty.icon"
        />
      </div>
      <n-pagination
        v-if="tableQuery.total.value > 0"
        :page="tableQuery.page.value"
        :page-size="tableQuery.pageSize.value"
        :item-count="tableQuery.total.value"
        :page-sizes="pageSizes"
        show-size-picker
        @update:page="tableQuery.setPage"
        @update:page-size="tableQuery.setPageSize"
      />
    </template>
    <n-data-table
      v-else
      v-model:checked-row-keys="checkedRowKeys"
      :columns="computedColumns"
      :data="tableQuery.items.value"
      :row-key="rowKey"
      :loading="tableQuery.loading.value"
      :pagination="pagination"
      :scroll-x="scrollX"
      :striped="striped"
      :size="size"
      remote
      flex-height
      class="flex-1"
    >
      <template #empty>
        <EmptyState :title="empty.title" :description="empty.description" :icon="empty.icon" />
      </template>
    </n-data-table>
  </div>
</template>

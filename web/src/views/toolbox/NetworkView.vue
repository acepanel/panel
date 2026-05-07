<script setup lang="ts">
import type { DataTableSortState } from 'naive-ui'
import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import toolboxNetwork, { type NetworkListParams } from '@/api/panel/toolbox-network'

const { $gettext } = useGettext()

// 排序状态
const sortState = ref<DataTableSortState | null>(null)
const sortKeyMapOrder = computed(() => {
  if (!sortState.value || !sortState.value.order) return {}
  return { [sortState.value.columnKey]: sortState.value.order }
})

// 筛选状态
const stateFilter = ref<string[]>([])
const pidSearch = ref('')
const processSearch = ref('')
const portSearch = ref('')

// 状态选项
const stateOptions = [
  { label: 'LISTEN', value: 'LISTEN' },
  { label: 'ESTABLISHED', value: 'ESTABLISHED' },
  { label: 'TIME_WAIT', value: 'TIME_WAIT' },
  { label: 'CLOSE_WAIT', value: 'CLOSE_WAIT' },
  { label: 'SYN_SENT', value: 'SYN_SENT' },
  { label: 'SYN_RECV', value: 'SYN_RECV' },
  { label: 'FIN_WAIT1', value: 'FIN_WAIT1' },
  { label: 'FIN_WAIT2', value: 'FIN_WAIT2' },
  { label: 'LAST_ACK', value: 'LAST_ACK' },
  { label: 'CLOSING', value: 'CLOSING' },
  { label: 'NONE', value: 'NONE' },
]

// 渲染状态标签
const renderState = (state: string) => {
  switch (state) {
    case 'LISTEN':
      return h(NTag, { type: 'success', size: 'small' }, { default: () => state })
    case 'ESTABLISHED':
      return h(NTag, { type: 'info', size: 'small' }, { default: () => state })
    case 'TIME_WAIT':
    case 'CLOSE_WAIT':
    case 'FIN_WAIT1':
    case 'FIN_WAIT2':
    case 'LAST_ACK':
    case 'CLOSING':
      return h(NTag, { type: 'warning', size: 'small' }, { default: () => state })
    case 'NONE':
      return h(NTag, { size: 'small' }, { default: () => state })
    default:
      return h(NTag, { type: 'default', size: 'small' }, { default: () => state })
  }
}

const columns = computed<any[]>(() => [
  {
    title: $gettext('Type'),
    key: 'type',
    width: 100,
    sortOrder: sortKeyMapOrder.value.type || false,
    sorter: true,
  },
  {
    title: 'PID',
    key: 'pid',
    width: 120,
    sortOrder: sortKeyMapOrder.value.pid || false,
    sorter: true,
  },
  {
    title: $gettext('Process'),
    key: 'process',
    minWidth: 300,
    resizable: true,
    sortOrder: sortKeyMapOrder.value.process || false,
    sorter: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Local Address'),
    key: 'local',
    width: 280,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Remote Address'),
    key: 'remote',
    width: 280,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Status'),
    key: 'state',
    width: 160,
    render(row: any) {
      return renderState(row.state)
    },
  },
])

// 处理排序变化
const handleSorterChange = (sorter: DataTableSortState | DataTableSortState[] | null) => {
  if (Array.isArray(sorter)) {
    sortState.value = sorter[0] || null
  } else {
    sortState.value = sorter
  }
}

// 分页获取列表
const { loading, data, page, total, pageSize, pageCount, reload } = usePagination(
  (page, pageSize) => {
    const sort = sortState.value?.columnKey as string | undefined
    const order = sortState.value?.order
      ? sortState.value.order === 'descend'
        ? 'desc'
        : 'asc'
      : undefined
    const params: NetworkListParams = {
      page,
      limit: pageSize,
      sort,
      order,
      state: stateFilter.value.length ? stateFilter.value.join(',') : undefined,
      pid: pidSearch.value || undefined,
      process: processSearch.value || undefined,
      port: portSearch.value || undefined,
    }
    return toolboxNetwork.list(params)
  },
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 50,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [sortState, stateFilter, pidSearch, processSearch, portSearch],
  },
)
</script>

<template>
  <n-flex vertical :size="16">
    <!-- 工具栏 -->
    <n-flex :size="12" :wrap="true">
      <n-select
        v-model:value="stateFilter"
        multiple
        :options="stateOptions"
        :placeholder="$gettext('Filter by status')"
        clearable
        class="w-70"
        @update:value="page = 1"
      />
      <n-input
        v-model:value="pidSearch"
        :placeholder="$gettext('Search PID')"
        clearable
        class="!w-35"
      >
        <template #prefix>
          <the-icon :size="16" icon="mdi:magnify" />
        </template>
      </n-input>
      <n-input
        v-model:value="processSearch"
        :placeholder="$gettext('Search process')"
        clearable
        class="!w-45"
      >
        <template #prefix>
          <the-icon :size="16" icon="mdi:magnify" />
        </template>
      </n-input>
      <n-input
        v-model:value="portSearch"
        :placeholder="$gettext('Search port')"
        clearable
        class="!w-35"
      >
        <template #prefix>
          <the-icon :size="16" icon="mdi:magnify" />
        </template>
      </n-input>
      <n-button @click="reload" type="primary" ghost>{{ $gettext('Refresh') }}</n-button>
    </n-flex>

    <!-- 网络连接列表 -->
    <n-data-table
      striped
      remote
      virtual-scroll
      :scroll-x="1300"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => `${row.type}-${row.pid}-${row.local}-${row.remote}`"
      max-height="60vh"
      @update:sorter="handleSorterChange"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [50, 100, 200, 500],
      }"
    />
  </n-flex>
</template>

<script setup lang="ts">
import type { DataTableSortState, DropdownOption } from 'naive-ui'
import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import process, { type ProcessListParams } from '@/api/panel/process'
import { formatBytes, formatDateTime, formatPercent } from '@/utils'

const { $gettext } = useGettext()

// 排序状态
const sortState = ref<DataTableSortState | null>(null)
const sortKeyMapOrder = computed(() => {
  if (!sortState.value || !sortState.value.order) return {}
  return { [sortState.value.columnKey]: sortState.value.order }
})

// 筛选状态
const statusFilter = ref<string>('')
const keyword = ref<string>('')

// 右键菜单相关
const showDropdown = ref(false)
const selectedRow = ref<any>(null)
const dropdownX = ref(0)
const dropdownY = ref(0)

// 进程详情弹窗
const detailModal = ref(false)
const detailLoading = ref(false)
const processDetail = ref<any>(null)

// 信号定义
const SIGNALS = {
  SIGHUP: 1, // 挂起
  SIGINT: 2, // 中断 (Ctrl+C)
  SIGKILL: 9, // 强制终止
  SIGTERM: 15, // 终止
  SIGCONT: 18, // 继续
  SIGSTOP: 19, // 暂停
  SIGUSR1: 10, // 用户自定义信号1
  SIGUSR2: 12 // 用户自定义信号2
}

// 状态选项
const statusOptions = [
  { label: $gettext('All Status'), value: '' },
  { label: $gettext('Running'), value: 'R' },
  { label: $gettext('Sleeping'), value: 'S' },
  { label: $gettext('Stopped'), value: 'T' },
  { label: $gettext('Idle'), value: 'I' },
  { label: $gettext('Zombie'), value: 'Z' },
  { label: $gettext('Waiting'), value: 'W' },
  { label: $gettext('Locked'), value: 'L' }
]

// 右键菜单选项
const dropdownOptions = computed<DropdownOption[]>(() => {
  if (!selectedRow.value) return []
  return [
    { label: $gettext('View Details'), key: 'detail' },
    { type: 'divider', key: 'd1' },
    { label: $gettext('Terminate (SIGTERM)'), key: 'sigterm' },
    { label: $gettext('Kill (SIGKILL)'), key: 'sigkill' },
    { type: 'divider', key: 'd2' },
    { label: $gettext('Stop (SIGSTOP)'), key: 'sigstop' },
    { label: $gettext('Continue (SIGCONT)'), key: 'sigcont' },
    { type: 'divider', key: 'd3' },
    { label: $gettext('Interrupt (SIGINT)'), key: 'sigint' },
    { label: $gettext('Hang Up (SIGHUP)'), key: 'sighup' },
    { label: $gettext('User Signal 1 (SIGUSR1)'), key: 'sigusr1' },
    { label: $gettext('User Signal 2 (SIGUSR2)'), key: 'sigusr2' }
  ]
})

// 渲染状态标签
const renderStatus = (status: string) => {
  switch (status) {
    case 'R':
      return h(NTag, { type: 'success' }, { default: () => $gettext('Running') })
    case 'S':
      return h(NTag, { type: 'warning' }, { default: () => $gettext('Sleeping') })
    case 'T':
      return h(NTag, { type: 'error' }, { default: () => $gettext('Stopped') })
    case 'I':
      return h(NTag, { type: 'primary' }, { default: () => $gettext('Idle') })
    case 'Z':
      return h(NTag, { type: 'error' }, { default: () => $gettext('Zombie') })
    case 'W':
      return h(NTag, { type: 'warning' }, { default: () => $gettext('Waiting') })
    case 'L':
      return h(NTag, { type: 'info' }, { default: () => $gettext('Locked') })
    default:
      return h(NTag, { type: 'default' }, { default: () => status })
  }
}

const columns = computed<any[]>(() => [
  {
    title: 'PID',
    key: 'pid',
    width: 160,
    sortOrder: sortKeyMapOrder.value.pid || false,
    sorter: true
  },
  {
    title: $gettext('Name'),
    key: 'name',
    resizable: true,
    sortOrder: sortKeyMapOrder.value.name || false,
    sorter: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Parent PID'),
    key: 'ppid',
    width: 160,
    sortOrder: sortKeyMapOrder.value.ppid || false,
    sorter: true
  },
  {
    title: $gettext('Threads'),
    key: 'num_threads',
    width: 100,
    sortOrder: sortKeyMapOrder.value.num_threads || false,
    sorter: true
  },
  {
    title: $gettext('User'),
    key: 'username',
    width: 100,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 100,
    render(row: any) {
      return renderStatus(row.status)
    }
  },
  {
    title: 'CPU',
    key: 'cpu',
    width: 100,
    sortOrder: sortKeyMapOrder.value.cpu || false,
    sorter: true,
    render(row: any): string {
      return formatPercent(row.cpu) + '%'
    }
  },
  {
    title: $gettext('Memory'),
    key: 'rss',
    width: 140,
    sortOrder: sortKeyMapOrder.value.rss || false,
    sorter: true,
    render(row: any): string {
      return formatBytes(row.rss)
    }
  },
  {
    title: $gettext('Start Time'),
    key: 'start_time',
    width: 240,
    sortOrder: sortKeyMapOrder.value.start_time || false,
    sorter: true
  }
])

// 行属性 - 右键菜单
const rowProps = (row: any) => {
  return {
    onContextmenu: (e: MouseEvent) => {
      e.preventDefault()
      showDropdown.value = false
      nextTick().then(() => {
        showDropdown.value = true
        selectedRow.value = row
        dropdownX.value = e.clientX
        dropdownY.value = e.clientY
      })
    }
  }
}

// 关闭右键菜单
const onCloseDropdown = () => {
  showDropdown.value = false
  selectedRow.value = null
}

// 处理右键菜单选择
const handleDropdownSelect = (key: string) => {
  showDropdown.value = false
  if (!selectedRow.value) return

  const pid = selectedRow.value.pid

  switch (key) {
    case 'detail':
      handleShowDetail(pid)
      break
    case 'sigterm':
      handleSignal(pid, SIGNALS.SIGTERM, 'SIGTERM')
      break
    case 'sigkill':
      handleSignal(pid, SIGNALS.SIGKILL, 'SIGKILL')
      break
    case 'sigstop':
      handleSignal(pid, SIGNALS.SIGSTOP, 'SIGSTOP')
      break
    case 'sigcont':
      handleSignal(pid, SIGNALS.SIGCONT, 'SIGCONT')
      break
    case 'sigint':
      handleSignal(pid, SIGNALS.SIGINT, 'SIGINT')
      break
    case 'sighup':
      handleSignal(pid, SIGNALS.SIGHUP, 'SIGHUP')
      break
    case 'sigusr1':
      handleSignal(pid, SIGNALS.SIGUSR1, 'SIGUSR1')
      break
    case 'sigusr2':
      handleSignal(pid, SIGNALS.SIGUSR2, 'SIGUSR2')
      break
  }
}

// 发送信号
const handleSignal = (pid: number, signal: number, signalName: string) => {
  window.$dialog.warning({
    title: $gettext('Confirm'),
    content: $gettext('Are you sure you want to send %{ signal } to process %{ pid }?', {
      signal: signalName,
      pid: pid.toString()
    }),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(process.signal(pid, signal)).onSuccess(() => {
        refresh()
        window.$message.success(
          $gettext('Signal %{ signal } has been sent to process %{ pid }', {
            signal: signalName,
            pid: pid.toString()
          })
        )
      })
    }
  })
}

// 显示进程详情
const handleShowDetail = (pid: number) => {
  detailLoading.value = true
  detailModal.value = true
  useRequest(process.detail(pid))
    .onSuccess(({ data }) => {
      processDetail.value = data
    })
    .onComplete(() => {
      detailLoading.value = false
    })
}

// 处理排序变化
const handleSorterChange = (sorter: DataTableSortState | DataTableSortState[] | null) => {
  if (Array.isArray(sorter)) {
    sortState.value = sorter[0] || null
  } else {
    sortState.value = sorter
  }
}

// 分页获取进程列表
const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => {
    const sort = sortState.value?.columnKey as string | undefined
    // descend(箭头向下) -> desc(大到小), ascend(箭头向上) -> asc(小到大)
    const order = sortState.value?.order
      ? sortState.value.order === 'descend'
        ? 'desc'
        : 'asc'
      : undefined
    const params: ProcessListParams = {
      page,
      limit: pageSize,
      sort,
      order,
      status: statusFilter.value || undefined,
      keyword: keyword.value || undefined
    }
    return process.list(params)
  },
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 50,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [sortState, statusFilter, keyword]
  }
)
</script>

<template>
  <n-flex vertical :size="16">
    <!-- 工具栏 -->
    <n-flex :size="12">
      <n-input
        v-model:value="keyword"
        :placeholder="$gettext('Search by PID or name')"
        clearable
        style="width: 250px"
      >
        <template #prefix>
          <the-icon :size="16" icon="mdi:magnify" />
        </template>
      </n-input>
      <n-select
        v-model:value="statusFilter"
        :options="statusOptions"
        style="width: 150px"
        @update:value="page = 1"
      />
      <n-button @click="refresh" type="primary" ghost>{{ $gettext('Refresh') }}</n-button>
    </n-flex>

    <!-- 进程列表 -->
    <n-data-table
      striped
      remote
      virtual-scroll
      :scroll-x="1400"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.pid"
      :row-props="rowProps"
      max-height="60vh"
      @update:sorter="handleSorterChange"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [50, 100, 200, 500]
      }"
    />

    <!-- 右键菜单 -->
    <n-dropdown
      placement="bottom-start"
      trigger="manual"
      :x="dropdownX"
      :y="dropdownY"
      :options="dropdownOptions"
      :show="showDropdown"
      :on-clickoutside="onCloseDropdown"
      @select="handleDropdownSelect"
    />

    <!-- 进程详情弹窗 -->
    <n-modal
      v-model:show="detailModal"
      preset="card"
      :title="$gettext('Process Details')"
      style="width: 80vw; max-width: 900px"
      size="huge"
      :bordered="false"
      :segmented="false"
    >
      <n-spin :show="detailLoading">
        <n-descriptions v-if="processDetail" :column="2" bordered label-placement="left">
          <n-descriptions-item :label="'PID'">
            {{ processDetail.pid }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Parent PID')">
            {{ processDetail.ppid }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Name')">
            {{ processDetail.name }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('User')">
            {{ processDetail.username }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Status')">
            <component :is="() => renderStatus(processDetail.status)" />
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Threads')">
            {{ processDetail.num_threads }}
          </n-descriptions-item>
          <n-descriptions-item :label="'CPU'">
            {{ formatPercent(processDetail.cpu) }}%
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Memory (RSS)')">
            {{ formatBytes(processDetail.rss) }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Virtual Memory')">
            {{ formatBytes(processDetail.vms) }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Swap')">
            {{ formatBytes(processDetail.swap) }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Disk Read')">
            {{ formatBytes(processDetail.disk_read) }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Disk Write')">
            {{ formatBytes(processDetail.disk_write) }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Start Time')" :span="2">
            {{ formatDateTime(processDetail.start_time) }}
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Executable Path')" :span="2">
            <n-ellipsis style="max-width: 600px">
              {{ processDetail.exe || '-' }}
            </n-ellipsis>
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Working Directory')" :span="2">
            <n-ellipsis style="max-width: 600px">
              {{ processDetail.cwd || '-' }}
            </n-ellipsis>
          </n-descriptions-item>
          <n-descriptions-item :label="$gettext('Command Line')" :span="2">
            <n-ellipsis :line-clamp="3" style="max-width: 600px">
              {{ processDetail.cmd_line || '-' }}
            </n-ellipsis>
          </n-descriptions-item>
        </n-descriptions>
        <n-collapse v-if="processDetail" style="margin-top: 16px">
          <n-collapse-item
            v-if="processDetail.envs?.length"
            :title="$gettext('Environment Variables')"
            name="env"
          >
            <n-scrollbar style="max-height: 200px">
              <n-log
                :log="
                  processDetail.envs?.length
                    ? processDetail.envs.join('\n')
                    : $gettext('No environment variables')
                "
                word-wrap
              />
            </n-scrollbar>
          </n-collapse-item>
          <n-collapse-item
            v-if="processDetail.open_files?.length"
            :title="$gettext('Open Files')"
            name="files"
          >
            <n-scrollbar style="max-height: 200px">
              <n-log
                :log="
                  processDetail.open_files?.length
                    ? processDetail.open_files.map((f: any) => f.path).join('\n')
                    : $gettext('No open files')
                "
                word-wrap
              />
            </n-scrollbar>
          </n-collapse-item>
          <n-collapse-item
            v-if="processDetail.connections?.length"
            :title="$gettext('Network Connections')"
            name="connections"
          >
            <n-scrollbar style="max-height: 200px">
              <n-log
                :log="
                  processDetail.connections?.length
                    ? processDetail.connections
                        .map(
                          (c: any) =>
                            `${c.localaddr?.ip || '*'}:${c.localaddr?.port || '*'} -> ${c.remoteaddr?.ip || '*'}:${c.remoteaddr?.port || '*'} (${c.status || 'UNKNOWN'})`
                        )
                        .join('\n')
                    : $gettext('No network connections')
                "
                word-wrap
              />
            </n-scrollbar>
          </n-collapse-item>
        </n-collapse>
      </n-spin>
    </n-modal>
  </n-flex>
</template>

<script lang="ts" setup>
import { NButton, NDataTable, NFlex, NPopconfirm, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import project from '@/api/panel/project'
import systemctl from '@/api/panel/systemctl'
import RealtimeLog from '@/components/common/RealtimeLog.vue'

const type = defineModel<string>('type', { type: String, required: true })
const createModal = defineModel<boolean>('createModal', { type: Boolean, required: true })
const editModal = defineModel<boolean>('editModal', { type: Boolean, required: true })
const editId = defineModel<number>('editId', { type: Number, required: true })
const logModal = ref(false)
const logService = ref('')

const { $gettext } = useGettext()
const selectedRowKeys = ref<any>([])

const typeMap: Record<string, string> = {
  general: $gettext('General'),
  php: 'PHP',
  java: 'Java',
  go: 'Go',
  python: 'Python',
  nodejs: 'Node.js'
}

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Name'),
    key: 'name',
    width: 160,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Description'),
    key: 'description',
    width: 300,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render(row: any) {
      return h(NTag, { type: 'info' }, { default: () => typeMap[row.type] || row.type })
    }
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 100,
    render(row: any) {
      return h(
        NTag,
        { type: row.status === 'running' ? 'success' : 'default' },
        { default: () => (row.status === 'running' ? $gettext('Running') : $gettext('Stopped')) }
      )
    }
  },
  {
    title: $gettext('Directory'),
    key: 'root_dir',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 300,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: row.status === 'running' ? 'warning' : 'success',
            onClick: () => handleToggleStatus(row)
          },
          { default: () => (row.status === 'running' ? $gettext('Stop') : $gettext('Start')) }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            style: 'margin-left: 10px;',
            onClick: () => handleShowLog(row)
          },
          { default: () => $gettext('Logs') }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            style: 'margin-left: 10px;',
            onClick: () => handleEdit(row)
          },
          { default: () => $gettext('Edit') }
        ),
        h(
          NPopconfirm,
          {
            showIcon: false,
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            default: () =>
              $gettext('Are you sure you want to delete project %{ name }?', { name: row.name }),
            trigger: () =>
              h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 10px;'
                },
                { default: () => $gettext('Delete') }
              )
          }
        )
      ]
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => project.list(type.value, page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleToggleStatus = (row: any) => {
  if (row.status === 'running') {
    useRequest(systemctl.stop(row.name)).onSuccess(() => {
      row.status = 'stopped'
      window.$message.success($gettext('Stopped successfully'))
    })
  } else {
    useRequest(systemctl.start(row.name)).onSuccess(() => {
      row.status = 'running'
      window.$message.success($gettext('Started successfully'))
    })
  }
}

const handleShowLog = (row: any) => {
  logService.value = row.name
  logModal.value = true
}

const handleEdit = (row: any) => {
  editId.value = row.id
  editModal.value = true
}

const handleDelete = (id: number) => {
  useRequest(project.delete(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const bulkDelete = async () => {
  const promises = selectedRowKeys.value.map((id: any) => project.delete(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

onMounted(() => {
  refresh()
  window.$bus.on('project:refresh', refresh)
})

watch(type, () => {
  refresh()
})
</script>

<template>
  <n-flex vertical>
    <n-flex>
      <n-button type="primary" @click="createModal = true">
        {{ $gettext('Create Project') }}
      </n-button>
      <n-popconfirm @positive-click="bulkDelete">
        <template #trigger>
          <n-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Delete') }}
          </n-button>
        </template>
        {{ $gettext('Are you sure you want to delete the selected projects?') }}
      </n-popconfirm>
    </n-flex>
    <n-data-table
      striped
      remote
      :loading="loading"
      :scroll-x="1200"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200]
      }"
    />
  </n-flex>
  <n-modal
    v-model:show="logModal"
    preset="card"
    :title="$gettext('Logs')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="logModal = false"
    @mask-click="logModal = false"
  >
    <realtime-log :service="logService" />
  </n-modal>
</template>

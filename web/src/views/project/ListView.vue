<script lang="ts" setup>
import { NButton, NCheckbox, NDataTable, NFlex, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import project from '@/api/panel/project'
import systemctl from '@/api/panel/systemctl'
import RealtimeLog from '@/components/common/RealtimeLog.vue'
import { useTablePagination } from '@/components/system/composables/useTablePagination'
import ConfirmDialog from '@/components/system/ConfirmDialog.vue'
import { useFileStore } from '@/stores'

import ProxyModal from './ProxyModal.vue'

const type = defineModel<string>('type', { type: String, required: true })
const createModal = defineModel<boolean>('createModal', { type: Boolean, required: true })
const editModal = defineModel<boolean>('editModal', { type: Boolean, required: true })
const editId = defineModel<number>('editId', { type: Number, required: true })
const logModal = ref(false)
const logService = ref('')
const proxyModal = ref(false)
const proxyProject = ref({ id: 0, name: '' })
// 删除项目时是否一并删除绑定的网站，默认只解除绑定
const deleteWebsites = ref(false)

const fileStore = useFileStore()
const { $gettext } = useGettext()
const router = useRouter()
const selectedRowKeys = ref<any>([])
const keyword = ref('')

const typeMap: Record<string, string> = {
  general: $gettext('General'),
  php: 'PHP',
  java: 'Java',
  go: 'Go',
  python: 'Python',
  nodejs: 'Node.js',
  dotnet: '.NET',
}

// systemd ActiveState 映射，未列出的状态原样显示
const statusMap: Record<
  string,
  { type: 'default' | 'success' | 'info' | 'warning' | 'error'; label: string }
> = {
  active: { type: 'success', label: $gettext('Running') },
  activating: { type: 'info', label: $gettext('Starting') },
  reloading: { type: 'info', label: $gettext('Reloading') },
  deactivating: { type: 'info', label: $gettext('Stopping') },
  inactive: { type: 'warning', label: $gettext('Stopped') },
  failed: { type: 'error', label: $gettext('Failed') },
}

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Name'),
    key: 'name',
    width: 160,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Description'),
    key: 'description',
    width: 300,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render(row: any) {
      return h(NTag, { type: 'info' }, { default: () => typeMap[row.type] || row.type })
    },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 110,
    render(row: any) {
      const { type, label } = statusMap[row.status] ?? {
        type: 'default',
        label: row.status || $gettext('Unknown'),
      }
      return h(NTag, { type }, { default: () => label })
    },
  },
  {
    title: $gettext('Autostart'),
    key: 'enabled',
    width: 100,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.enabled,
        loading: switching.has(row.name),
        onUpdateValue: (value: boolean) => handleToggleAutostart(row, value),
      })
    },
  },
  {
    title: $gettext('Directory'),
    key: 'root_dir',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      return h(
        NTag,
        {
          class: 'tag-ellipsis cursor-pointer hover:opacity-60',
          type: 'info',
          title: row.root_dir,
          onClick: () => {
            if (fileStore.activeTab) fileStore.updateTabPath(fileStore.activeTabId, row.root_dir)
            router.push({ name: 'file-index' })
          },
        },
        { default: () => row.root_dir },
      )
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 520,
    hideInExcel: true,
    render(row: any) {
      const buttons = [
        h(
          NButton,
          {
            size: 'small',
            type: row.status === 'active' ? 'warning' : 'success',
            onClick: () => handleToggleStatus(row),
          },
          { default: () => (row.status === 'active' ? $gettext('Stop') : $gettext('Start')) },
        ),
      ]

      // 仅为运行中的项目显示重启和重载按钮
      if (row.status === 'active') {
        buttons.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'warning',
              secondary: true,
              onClick: () => handleRestart(row),
            },
            { default: () => $gettext('Restart') },
          ),
          h(
            NButton,
            {
              size: 'small',
              type: 'info',
              secondary: true,
              onClick: () => handleReload(row),
            },
            { default: () => $gettext('Reload') },
          ),
        )
      }

      buttons.push(
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleShowLog(row),
          },
          { default: () => $gettext('Logs') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            secondary: true,
            onClick: () => handleShowProxy(row),
          },
          { default: () => $gettext('Reverse Proxy') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => handleEdit(row),
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          ConfirmDialog,
          {
            type: 'delete',
            countdown: 5,
            onConfirm: () => handleDelete(row.id),
          },
          {
            default: () =>
              h(NFlex, { vertical: true }, () => [
                h(
                  'strong',
                  $gettext('Are you sure you want to delete project %{ name }?', {
                    name: row.name,
                  }),
                ),
                h(
                  NCheckbox,
                  {
                    checked: deleteWebsites.value,
                    onUpdateChecked: (v: boolean) => (deleteWebsites.value = v),
                  },
                  { default: () => $gettext('Also delete bound websites') },
                ),
              ]),
            trigger: () =>
              h(NButton, { size: 'small', type: 'error' }, { default: () => $gettext('Delete') }),
          },
        ),
      )

      return h(NFlex, { size: 'small', align: 'center' }, () => buttons)
    },
  },
]

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) => project.list(type.value, page, pageSize, keyword.value),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [type, keyword],
    debounce: [0, 300],
  },
)
const pagination = useTablePagination(page, pageSize, total)

const handleToggleStatus = (row: any) => {
  if (row.status === 'active') {
    useRequest(systemctl.stop(row.name)).onSuccess(() => {
      row.status = 'inactive'
      window.$message.success($gettext('Stopped successfully'))
    })
  } else {
    useRequest(systemctl.start(row.name)).onSuccess(() => {
      row.status = 'active'
      window.$message.success($gettext('Started successfully'))
    })
  }
}

const handleRestart = (row: any) => {
  useRequest(systemctl.restart(row.name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Restarted successfully'))
  })
}

const handleReload = (row: any) => {
  useRequest(systemctl.reload(row.name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Reloaded successfully'))
  })
}

const switching = reactive(new Set<string>())

const handleToggleAutostart = (row: any, enabled: boolean) => {
  switching.add(row.name)
  useRequest(enabled ? systemctl.enable(row.name) : systemctl.disable(row.name))
    .onSuccess(() => {
      row.enabled = enabled
      window.$message.success(
        enabled ? $gettext('Autostart enabled') : $gettext('Autostart disabled'),
      )
    })
    .onComplete(() => switching.delete(row.name))
}

const handleShowLog = (row: any) => {
  logService.value = row.name
  logModal.value = true
}

const handleShowProxy = (row: any) => {
  proxyProject.value = { id: row.id, name: row.name }
  proxyModal.value = true
}

const handleEdit = (row: any) => {
  editId.value = row.id
  editModal.value = true
}

const handleDelete = (id: number) => {
  useRequest(project.delete(id, deleteWebsites.value)).onSuccess(() => {
    if (deleteWebsites.value) window.$bus.emit('website:refresh')
    deleteWebsites.value = false
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
</script>

<template>
  <n-flex vertical>
    <n-flex justify="space-between">
      <n-flex>
        <n-button type="primary" @click="createModal = true">
          {{ $gettext('Create Project') }}
        </n-button>
        <ConfirmDialog
          type="delete"
          :countdown="5"
          :content="$gettext('Are you sure you want to delete the selected projects?')"
          @confirm="bulkDelete"
        >
          <template #trigger>
            <n-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
              {{ $gettext('Delete') }}
            </n-button>
          </template>
        </ConfirmDialog>
      </n-flex>
      <n-input
        v-model:value="keyword"
        :placeholder="$gettext('Search')"
        clearable
        class="md:!w-60"
      />
    </n-flex>
    <n-data-table
      striped
      remote
      :loading="loading"
      :scroll-x="1600"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="pagination"
    />
  </n-flex>
  <app-modal v-model:show="logModal" :title="$gettext('Logs')" @mask-click="logModal = false">
    <realtime-log :service="logService" />
  </app-modal>
  <proxy-modal v-model:show="proxyModal" :project="proxyProject" />
</template>

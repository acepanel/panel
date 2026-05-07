<script setup lang="ts">
import { NButton, NDataTable, NFlex, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cron from '@/api/panel/cron'
import file from '@/api/panel/file'
import CronPreview from '@/components/common/CronPreview.vue'
import PtyTerminalModal from '@/components/common/PtyTerminalModal.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { decodeBase64, formatDateTime } from '@/utils'
import CreateModal from '@/views/task/CreateModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const logPath = ref('')
const logModal = ref(false)
const logModalRef = ref<{ clear: () => void } | null>(null)

const handleClearLog = () => {
  if (!logPath.value) return
  useRequest(file.truncate(logPath.value)).onSuccess(() => {
    logModalRef.value?.clear()
    window.$message.success($gettext('Cleared successfully'))
  })
}
const shellEditModal = ref(false)
const visualEditModal = ref(false)
const saveTaskEditLoading = ref(false)
const runModal = ref(false)
const runCommand = ref('')
const runTaskName = ref('')

// shell 类型编辑
const shellEditTask = ref({
  id: 0,
  name: '',
  type: 'shell',
  flock: false,
  time: '',
  script: '',
})

// backup/cutoff 类型编辑数据
const visualEditData = ref<any>(null)
const selectedRowKeys = ref<any[]>([])

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Task Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Task Type'),
    key: 'type',
    width: 200,
    resizable: true,
    render(row: any) {
      const typeMap: Record<
        string,
        { type: 'default' | 'error' | 'warning' | 'success' | 'info' | 'primary'; label: string }
      > = {
        shell: { type: 'warning', label: $gettext('Run Script') },
        backup: { type: 'success', label: $gettext('Backup Data') },
        url: { type: 'default', label: $gettext('Access URL') },
        synctime: { type: 'primary', label: $gettext('Sync Time') },
      }
      const info = typeMap[row.type] || { type: 'info' as const, label: $gettext('Log Rotation') }
      return h(NTag, { type: info.type }, { default: () => info.label })
    },
  },
  {
    title: $gettext('Enabled'),
    key: 'status',
    width: 120,
    resizable: true,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.status,
        onUpdateValue: () => handleStatusChange(row),
      })
    },
  },
  {
    title: $gettext('Task Schedule'),
    key: 'time',
    width: 300,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(CronPreview, { cron: row.time })
    },
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Last Update Time'),
    key: 'updated_at',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatDateTime(row.updated_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 350,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'success',
            secondary: true,
            onClick: () => handleRun(row),
          },
          { default: () => $gettext('Run') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            onClick: () => {
              logPath.value = row.log
              logModal.value = true
            },
          },
          { default: () => $gettext('Logs') },
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
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this task?'),
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

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => cron.list(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)

const handleStatusChange = (row: any) => {
  useRequest(cron.status(row.id, !row.status)).onSuccess(() => {
    row.status = !row.status
    window.$message.success($gettext('Modified successfully'))
  })
}

const handleRun = (row: any) => {
  useRequest(cron.get(row.id)).onSuccess(({ data }) => {
    runTaskName.value = row.name
    runCommand.value = `bash '${data.shell}'`
    runModal.value = true
  })
}

const handleEdit = (row: any) => {
  if (
    row.type === 'backup' ||
    row.type === 'cutoff' ||
    row.type === 'url' ||
    row.type === 'synctime'
  ) {
    // 可视化编辑
    useRequest(cron.get(row.id)).onSuccess(({ data }) => {
      visualEditData.value = data
      visualEditModal.value = true
    })
  } else {
    // shell 脚本编辑
    useRequest(cron.get(row.id)).onSuccess(({ data }) => {
      useRequest(file.content(encodeURIComponent(data.shell))).onSuccess(({ data: fileData }) => {
        shellEditTask.value.id = row.id
        shellEditTask.value.name = row.name
        shellEditTask.value.type = row.type
        shellEditTask.value.flock = data.config?.flock ?? false
        shellEditTask.value.time = row.time
        shellEditTask.value.script = decodeBase64(fileData.content)
        shellEditModal.value = true
      })
    })
  }
}

const handleDelete = async (id: number) => {
  useRequest(cron.delete(id)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    window.$bus.emit('task:refresh-cron')
  })
}

const bulkDelete = async () => {
  const promises = selectedRowKeys.value.map((id: any) => cron.delete(id))
  await Promise.all(promises)
  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

defineExpose({ selectedRowKeys, bulkDelete })

const saveShellEdit = async () => {
  saveTaskEditLoading.value = true
  useRequest(
    cron.update(shellEditTask.value.id, {
      name: shellEditTask.value.name,
      type: shellEditTask.value.type,
      flock: shellEditTask.value.flock,
      time: shellEditTask.value.time,
      script: shellEditTask.value.script,
    }),
  )
    .onSuccess(() => {
      shellEditModal.value = false
      window.$message.success($gettext('Modified successfully'))
      window.$bus.emit('task:refresh-cron')
    })
    .onComplete(() => {
      saveTaskEditLoading.value = false
    })
}

onMounted(() => {
  refresh()
  window.$bus.on('task:refresh-cron', () => {
    refresh()
  })
})

onUnmounted(() => {
  window.$bus.off('task:refresh-cron')
})
</script>

<template>
  <n-data-table
    v-model:checked-row-keys="selectedRowKeys"
    v-model:page="page"
    v-model:pageSize="pageSize"
    striped
    remote
    :scroll-x="1600"
    :loading="loading"
    :columns="columns"
    :data="data"
    :row-key="(row: any) => row.id"
    :pagination="{
      page: page,
      pageSize: pageSize,
      itemCount: total,
      showQuickJumper: true,
      showSizePicker: true,
      pageSizes: [20, 50, 100, 200],
    }"
  />
  <realtime-log-modal
    ref="logModalRef"
    v-model:show="logModal"
    :path="logPath"
    clearable
    @clear="handleClearLog"
  />
  <!-- Shell 脚本编辑模态框 -->
  <n-modal
    v-model:show="shellEditModal"
    preset="card"
    :title="$gettext('Edit Task')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form>
      <n-form-item :label="$gettext('Task Name')">
        <n-input v-model:value="shellEditTask.name" :placeholder="$gettext('Task Name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Task Schedule')">
        <cron-selector v-model:value="shellEditTask.time"></cron-selector>
      </n-form-item>
      <n-form-item :label="$gettext('Process Lock')">
        <n-switch v-model:value="shellEditTask.flock" />
        <n-text class="ml-2" depth="3">
          {{
            $gettext(
              'Prevent duplicate execution: skip this run if the previous one is still running',
            )
          }}
        </n-text>
      </n-form-item>
    </n-form>
    <common-editor v-model:value="shellEditTask.script" lang="shell" height="40vh" />
    <n-button
      class="mt-2"
      type="info"
      block
      :loading="saveTaskEditLoading"
      :disabled="saveTaskEditLoading"
      @click="saveShellEdit"
    >
      {{ $gettext('Save') }}
    </n-button>
  </n-modal>
  <create-modal v-model:show="visualEditModal" mode="edit" :edit-data="visualEditData" />
  <pty-terminal-modal
    v-model:show="runModal"
    :title="$gettext('Run Task - %{ name }', { name: runTaskName })"
    :command="runCommand"
  />
</template>

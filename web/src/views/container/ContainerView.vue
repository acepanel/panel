<script setup lang="ts">
import { NButton, NDataTable, NDropdown, NFlex, NInput, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
import ws from '@/api/ws'
import { createXterm } from '@/components/common/composables/useXterm'
import type { Xterm } from '@/components/common/composables/useXterm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { useResponsive } from '@/components/system/composables/useResponsive'
import ContainerCreate from '@/views/container/ContainerCreate.vue'
import ContainerInfoModal from '@/views/container/ContainerInfoModal.vue'

const { $gettext } = useGettext()

const keyword = ref('')
const logModal = ref(false)
const logId = ref('')
const infoModal = ref(false)
const infoContainerId = ref('')
const editModal = ref(false)
const editContainerId = ref('')
const renameModal = ref(false)
const renameModel = ref({
  id: '',
  name: '',
})

// 终端相关状态
const terminalModal = ref(false)
const terminalContainerName = ref('')
const terminalRef = ref<HTMLElement | null>(null)
let xterm: Xterm | null = null

const containerCreateModal = ref(false)
const selectedRowKeys = ref<any>([])

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Container Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Status'),
    key: 'state',
    width: 100,
    resizable: true,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.state === 'running',
        loading: switching.has(row.id),
        onUpdateValue: (value: boolean) => run(value ? 'start' : 'stop', [row.id]),
      })
    },
  },
  {
    title: $gettext('Image'),
    key: 'image',
    minWidth: 300,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Ports (Host->Container)'),
    key: 'ports',
    minWidth: 200,
    resizable: true,
    render(row: any): any {
      return h(NFlex, null, {
        default: () =>
          row.ports.map((port: any) =>
            h(NTag, null, {
              default: () => {
                const container =
                  port.container_start == port.container_end
                    ? `${port.container_start}/${port.protocol}`
                    : `${port.container_start}-${port.container_end}/${port.protocol}`
                // 未映射到宿主机的端口只显示容器端口
                if (!port.host_start) {
                  return container
                }
                const host =
                  port.host_start == port.host_end
                    ? `${port.host_start}`
                    : `${port.host_start}-${port.host_end}`
                return `${port.host ? port.host + ':' : ''}${host}->${container}`
              },
            }),
          ),
      })
    },
  },
  {
    title: $gettext('Running Status'),
    key: 'status',
    width: 300,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 380,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            secondary: true,
            onClick: () => {
              infoContainerId.value = row.id
              infoModal.value = true
            },
          },
          {
            default: () => $gettext('Info'),
          },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleOpenTerminal(row),
            disabled: row.state !== 'running',
          },
          {
            default: () => $gettext('Terminal'),
          },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            onClick: () => handleShowLog(row),
          },
          {
            default: () => $gettext('Logs'),
          },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'success',
            onClick: () => {
              renameModel.value.id = row.id
              renameModel.value.name = row.name
              renameModal.value = true
            },
          },
          {
            default: () => $gettext('Rename'),
          },
        ),
        h(
          NDropdown,
          {
            disabled: switching.has(row.id),
            options: [
              { label: $gettext('Edit'), key: 'edit' },
              ...actions.map((item) => ({
                label: item.label,
                key: item.key,
                disabled: item.running !== undefined && item.running !== (row.state === 'running'),
              })),
            ],
            onSelect: (key: string) => {
              if (key === 'edit') {
                editContainerId.value = row.id
                editModal.value = true
              } else {
                run(key, [row.id])
              }
            },
          },
          {
            default: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'primary',
                  loading: switching.has(row.id),
                },
                {
                  default: () => $gettext('More'),
                },
              )
            },
          },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable(
  (page, pageSize) => container.containerList(page, pageSize, keyword.value),
  {
    watchingStates: [keyword],
    debounce: [300],
  },
)

const handleShowLog = (row: any) => {
  logId.value = row.id
  logModal.value = true
}

const handleRename = async () => {
  await container.containerRename(renameModel.value.id, renameModel.value.name)
  refresh()
  renameModal.value = false
  window.$message.success($gettext('Rename successful'))
}

// 用 type 而非 interface：要能直接当作下拉选项传给 n-dropdown
type ContainerAction = {
  key: string
  label: string
  // 要求容器所处的运行状态，缺省不限
  running?: boolean
  request: (id: string) => any
  success: string
}

// 行内开关与下拉、批量按钮组、手机批量下拉共用
const actions: ContainerAction[] = [
  {
    key: 'start',
    label: $gettext('Start'),
    running: false,
    request: container.containerStart,
    success: $gettext('Start successful'),
  },
  {
    key: 'stop',
    label: $gettext('Stop'),
    running: true,
    request: container.containerStop,
    success: $gettext('Stop successful'),
  },
  {
    key: 'restart',
    label: $gettext('Restart'),
    running: true,
    request: container.containerRestart,
    success: $gettext('Restart successful'),
  },
  {
    key: 'forceStop',
    label: $gettext('Force Stop'),
    running: true,
    request: container.containerKill,
    success: $gettext('Force stop successful'),
  },
  {
    key: 'pause',
    label: $gettext('Pause'),
    running: true,
    request: container.containerPause,
    success: $gettext('Pause successful'),
  },
  {
    key: 'unpause',
    label: $gettext('Resume'),
    running: false,
    request: container.containerUnpause,
    success: $gettext('Resume successful'),
  },
  {
    key: 'delete',
    label: $gettext('Delete'),
    request: container.containerRemove,
    success: $gettext('Delete successful'),
  },
]

// 操作进行中的容器，行内开关和更多按钮显示加载态，更多下拉禁止再选
const switching = reactive(new Set<string>())

const run = async (key: string, ids: string[]) => {
  const action = actions.find((item) => item.key === key)!
  ids.forEach((id) => switching.add(id))
  try {
    await Promise.all(ids.map((id) => action.request(id)))
    selectedRowKeys.value = []
    refresh()
    window.$message.success(action.success)
  } finally {
    ids.forEach((id) => switching.delete(id))
  }
}

const handlePrune = async () => {
  await container.containerPrune()
  refresh()
  window.$message.success($gettext('Cleanup successful'))
}

// 批量操作：桌面按钮组，手机七个按钮放不下改下拉
const { isMobile } = useResponsive()
const batchRunning = ref(false)

const handleBatchSelect = async (key: string) => {
  batchRunning.value = true
  try {
    await run(key, selectedRowKeys.value)
  } finally {
    batchRunning.value = false
  }
}

const closeContainerCreateModal = () => {
  refresh()
}

// 打开容器终端
const handleOpenTerminal = async (row: any) => {
  terminalContainerName.value = row.name
  terminalModal.value = true

  await nextTick()

  // 确保终端容器存在
  const el = terminalRef.value
  if (!el) {
    window.$message.error($gettext('Terminal container not found'))
    return
  }

  try {
    xterm = createXterm(el, await ws.container(row.id), {
      theme: { background: '#111', foreground: '#fff' },
    })
  } catch (error) {
    console.error('Failed to connect to container terminal:', error)
    window.$message.error($gettext('Failed to connect to container terminal'))
    terminalModal.value = false
  }
}

// 关闭容器终端
const closeTerminal = () => {
  xterm?.dispose()
  xterm = null
}

onUnmounted(() => {
  closeTerminal()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex justify="space-between">
      <n-flex>
        <n-button type="primary" @click="containerCreateModal = true">
          {{ $gettext('Create Container') }}
        </n-button>
        <app-button type="primary" @click="handlePrune" ghost>
          {{ $gettext('Cleanup Containers') }}
        </app-button>
        <n-dropdown
          v-if="isMobile"
          trigger="click"
          :options="actions"
          :disabled="selectedRowKeys.length === 0 || batchRunning"
          @select="handleBatchSelect"
        >
          <n-button :loading="batchRunning" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Actions') }}
          </n-button>
        </n-dropdown>
        <n-button-group v-else>
          <app-button
            v-for="item in actions"
            :key="item.key"
            :disabled="selectedRowKeys.length === 0"
            ghost
            @click="run(item.key, selectedRowKeys)"
          >
            {{ item.label }}
          </app-button>
        </n-button-group>
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
      :data="data"
      :columns="columns"
      :row-key="(row: any) => row.id"
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="pagination"
    />
  </n-flex>
  <realtime-log-modal v-model:show="logModal" :container="logId" />
  <app-modal v-model:show="renameModal" :title="$gettext('Rename')">
    <n-form :model="renameModel">
      <n-form-item path="name" :label="$gettext('New Name')">
        <n-input
          v-model:value="renameModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter new name')"
        />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleRename">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
  <app-modal
    v-model:show="terminalModal"
    size="full"
    :title="$gettext('Terminal') + ' - ' + terminalContainerName"
    style="height: 80vh"
    @after-leave="closeTerminal"
  >
    <div ref="terminalRef" style="height: 100%; min-height: 60vh; background: #111"></div>
  </app-modal>
  <ContainerCreate v-model:show="containerCreateModal" @update:show="closeContainerCreateModal" />
  <ContainerCreate
    v-model:show="editModal"
    :edit-id="editContainerId"
    @update:show="closeContainerCreateModal"
  />
  <ContainerInfoModal v-model:show="infoModal" :container-id="infoContainerId" />
</template>

<style scoped lang="scss">
:deep(.xterm) {
  padding: 1rem !important;
}

:deep(.xterm .xterm-viewport::-webkit-scrollbar) {
  border-radius: 0.4rem;
  height: 6px;
  width: 8px;
}

:deep(.xterm .xterm-viewport::-webkit-scrollbar-thumb) {
  background-color: #666;
  border-radius: 0.4rem;
  box-shadow: inset 0 0 5px rgba(0, 0, 0, 0.2);
  transition: all 1s;
}

:deep(.xterm .xterm-viewport:hover::-webkit-scrollbar-thumb) {
  background-color: #aaa;
}

:deep(.xterm .xterm-viewport::-webkit-scrollbar-track) {
  background-color: #111;
  border-radius: 0.4rem;
  box-shadow: inset 0 0 5px rgba(0, 0, 0, 0.2);
  transition: all 1s;
}

:deep(.xterm .xterm-viewport:hover::-webkit-scrollbar-track) {
  background-color: #444;
}
</style>

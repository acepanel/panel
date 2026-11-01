<script setup lang="ts">
import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import { FitAddon } from '@xterm/addon-fit'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { WebglAddon } from '@xterm/addon-webgl'
import { Terminal } from '@xterm/xterm'

import '@xterm/xterm/css/xterm.css'
import { NButton, NDataTable, NDropdown, NFlex, NInput, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
import ws from '@/api/ws'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { useTablePagination } from '@/components/system/composables/useTablePagination'
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
const renameLoading = ref(false)
const renameModel = ref({
  id: '',
  name: '',
})

// 终端相关状态
const terminalModal = ref(false)
const terminalContainerName = ref('')
const terminalRef = ref<HTMLElement | null>(null)
const term = ref<Terminal | null>(null)
let containerWs: WebSocket | null = null
let fitAddon: FitAddon | null = null
let webglAddon: WebglAddon | null = null

const containerCreateModal = ref(false)
const pruneLoading = ref(false)
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
        onUpdateValue: (value: boolean) => {
          if (value) {
            handleStart(row.id)
          } else {
            handleStop(row.id)
          }
        },
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
            options: [
              {
                label: $gettext('Edit'),
                key: 'edit',
              },
              {
                label: $gettext('Start'),
                key: 'start',
                disabled: row.state === 'running',
              },
              {
                label: $gettext('Stop'),
                key: 'stop',
                disabled: row.state !== 'running',
              },
              {
                label: $gettext('Restart'),
                key: 'restart',
                disabled: row.state !== 'running',
              },
              {
                label: $gettext('Force Stop'),
                key: 'forceStop',
                disabled: row.state !== 'running',
              },
              {
                label: $gettext('Pause'),
                key: 'pause',
                disabled: row.state !== 'running',
              },
              {
                label: $gettext('Resume'),
                key: 'unpause',
                disabled: row.state === 'running',
              },
              {
                label: $gettext('Delete'),
                key: 'delete',
              },
            ],
            onSelect: (key: string) => {
              switch (key) {
                case 'edit':
                  editContainerId.value = row.id
                  editModal.value = true
                  break
                case 'start':
                  handleStart(row.id)
                  break
                case 'stop':
                  handleStop(row.id)
                  break
                case 'restart':
                  handleRestart(row.id)
                  break
                case 'forceStop':
                  handleForceStop(row.id)
                  break
                case 'pause':
                  handlePause(row.id)
                  break
                case 'unpause':
                  handleUnpause(row.id)
                  break
                case 'delete':
                  handleDelete(row.id)
                  break
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

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) => container.containerList(page, pageSize, keyword.value),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [keyword],
    debounce: [300],
  },
)
const pagination = useTablePagination(page, pageSize, total)

const handleShowLog = (row: any) => {
  logId.value = row.id
  logModal.value = true
}

const handleRename = () => {
  renameLoading.value = true
  useRequest(container.containerRename(renameModel.value.id, renameModel.value.name))
    .onSuccess(() => {
      refresh()
      renameModal.value = false
      window.$message.success($gettext('Rename successful'))
    })
    .onComplete(() => {
      renameLoading.value = false
    })
}

const switching = reactive(new Set<string>())

const handleStart = (id: string) => {
  switching.add(id)
  useRequest(container.containerStart(id))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Start successful'))
    })
    .onComplete(() => switching.delete(id))
}

const handleStop = (id: string) => {
  switching.add(id)
  useRequest(container.containerStop(id))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Stop successful'))
    })
    .onComplete(() => switching.delete(id))
}

const handleRestart = (id: string) => {
  useRequest(container.containerRestart(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Restart successful'))
  })
}

const handleForceStop = (id: string) => {
  useRequest(container.containerKill(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Force stop successful'))
  })
}

const handlePause = (id: string) => {
  useRequest(container.containerPause(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Pause successful'))
  })
}

const handleUnpause = (id: string) => {
  useRequest(container.containerUnpause(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Resume successful'))
  })
}

const handleDelete = (id: string) => {
  useRequest(container.containerRemove(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Delete successful'))
  })
}

const handlePrune = () => {
  pruneLoading.value = true
  useRequest(container.containerPrune())
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Cleanup successful'))
    })
    .onComplete(() => {
      pruneLoading.value = false
    })
}

const bulkStart = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.containerStart(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Start successful'))
}

const bulkStop = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.containerStop(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Stop successful'))
}

const bulkRestart = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.containerRestart(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Restart successful'))
}

const bulkForceStop = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.containerKill(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Force stop successful'))
}

const bulkDelete = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.containerRemove(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Delete successful'))
}

const bulkPause = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.containerPause(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Pause successful'))
}

const bulkUnpause = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.containerUnpause(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Resume successful'))
}

// 批量操作：桌面按钮组，手机七个按钮放不下改下拉
const { isMobile } = useResponsive()
const bulkActions = [
  { label: $gettext('Start'), key: 'start', handler: bulkStart },
  { label: $gettext('Stop'), key: 'stop', handler: bulkStop },
  { label: $gettext('Restart'), key: 'restart', handler: bulkRestart },
  { label: $gettext('Force Stop'), key: 'forceStop', handler: bulkForceStop },
  { label: $gettext('Pause'), key: 'pause', handler: bulkPause },
  { label: $gettext('Resume'), key: 'unpause', handler: bulkUnpause },
  { label: $gettext('Delete'), key: 'delete', handler: bulkDelete },
]
const handleBulkSelect = (key: string) => bulkActions.find((item) => item.key === key)?.handler()

const closeContainerCreateModal = () => {
  refresh()
}

// 打开容器终端
const handleOpenTerminal = async (row: any) => {
  terminalContainerName.value = row.name
  terminalModal.value = true

  await nextTick()

  // 确保终端容器存在
  if (!terminalRef.value) {
    window.$message.error($gettext('Terminal container not found'))
    return
  }

  try {
    containerWs = await ws.container(row.id)
    containerWs.binaryType = 'arraybuffer'

    term.value = new Terminal({
      allowProposedApi: true,
      lineHeight: 1.2,
      fontSize: 14,
      fontFamily: `'JetBrains Mono Variable', monospace`,
      cursorBlink: true,
      cursorStyle: 'underline',
      tabStopWidth: 4,
      scrollOnEraseInDisplay: true,
      theme: { background: '#111', foreground: '#fff' },
    })

    fitAddon = new FitAddon()
    webglAddon = new WebglAddon()

    term.value.loadAddon(fitAddon)
    term.value.loadAddon(new ClipboardAddon())
    term.value.loadAddon(new WebLinksAddon())
    term.value.loadAddon(new Unicode11Addon())
    term.value.unicode.activeVersion = '11'
    term.value.loadAddon(webglAddon)
    webglAddon.onContextLoss(() => {
      webglAddon?.dispose()
    })
    term.value.open(terminalRef.value)

    containerWs.onmessage = (ev) => {
      const data: ArrayBuffer | string = ev.data
      term.value?.write(typeof data === 'string' ? data : new Uint8Array(data))
    }
    term.value.onData((data) => {
      if (containerWs?.readyState === WebSocket.OPEN) {
        containerWs?.send(data)
      }
    })
    term.value.onBinary((data) => {
      if (containerWs?.readyState === WebSocket.OPEN) {
        const buffer = new Uint8Array(data.length)
        for (let i = 0; i < data.length; ++i) {
          buffer[i] = data.charCodeAt(i) & 255
        }
        containerWs?.send(buffer)
      }
    })
    term.value.onResize(({ rows, cols }) => {
      if (containerWs && containerWs.readyState === WebSocket.OPEN) {
        containerWs.send(
          JSON.stringify({
            resize: true,
            columns: cols,
            rows: rows,
          }),
        )
      }
    })

    fitAddon.fit()
    term.value.focus()
    window.addEventListener('resize', onTerminalResize, false)

    containerWs.onclose = () => {
      if (term.value) {
        term.value.write('\r\n' + $gettext('Connection closed.'))
      }
      window.removeEventListener('resize', onTerminalResize)
    }

    containerWs.onerror = (event) => {
      if (term.value) {
        term.value.write('\r\n' + $gettext('Connection error.'))
      }
      console.error(event)
      containerWs?.close()
    }
  } catch (error) {
    console.error('Failed to connect to container terminal:', error)
    window.$message.error($gettext('Failed to connect to container terminal'))
    terminalModal.value = false
  }
}

// 关闭容器终端
const closeTerminal = () => {
  try {
    if (containerWs) {
      containerWs.close()
      containerWs = null
    }
    if (term.value) {
      term.value.dispose()
      term.value = null
    }
    fitAddon = null
    webglAddon = null
    if (terminalRef.value) {
      terminalRef.value.innerHTML = ''
    }
    window.removeEventListener('resize', onTerminalResize)
  } catch {
    /* empty */
  }
}

// 终端大小调整
const onTerminalResize = () => {
  if (fitAddon && term.value) {
    fitAddon.fit()
  }
}

// 终端滚轮缩放
const onTerminalWheel = (event: WheelEvent) => {
  if (event.ctrlKey && term.value && fitAddon) {
    event.preventDefault()
    if (event.deltaY > 0) {
      if (term.value.options.fontSize! > 12) {
        term.value.options.fontSize = term.value.options.fontSize! - 1
      }
    } else {
      term.value.options.fontSize = term.value.options.fontSize! + 1
    }
    fitAddon.fit()
  }
}

// 终端模态框关闭后清理
const handleTerminalModalClose = () => {
  closeTerminal()
}

onMounted(() => {
  refresh()
})

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
        <n-button
          type="primary"
          :loading="pruneLoading"
          :disabled="pruneLoading"
          @click="handlePrune"
          ghost
        >
          {{ $gettext('Cleanup Containers') }}
        </n-button>
        <n-dropdown
          v-if="isMobile"
          trigger="click"
          :options="bulkActions"
          :disabled="selectedRowKeys.length === 0"
          @select="handleBulkSelect"
        >
          <n-button :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Actions') }}
          </n-button>
        </n-dropdown>
        <n-button-group v-else>
          <n-button
            v-for="item in bulkActions"
            :key="item.key"
            :disabled="selectedRowKeys.length === 0"
            ghost
            @click="item.handler"
          >
            {{ item.label }}
          </n-button>
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
    <n-button
      type="info"
      block
      :loading="renameLoading"
      :disabled="renameLoading"
      @click="handleRename"
    >
      {{ $gettext('Submit') }}
    </n-button>
  </app-modal>
  <app-modal
    v-model:show="terminalModal"
    size="full"
    :title="$gettext('Terminal') + ' - ' + terminalContainerName"
    style="height: 80vh"
    @after-leave="handleTerminalModalClose"
  >
    <div
      ref="terminalRef"
      @wheel="onTerminalWheel"
      style="height: 100%; min-height: 60vh; background: #111"
    ></div>
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

<script setup lang="ts">
import { NButton, NDataTable, NDropdown, NFlex, NInput, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'
import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import { FitAddon } from '@xterm/addon-fit'
import { AttachAddon } from '@xterm/addon-attach'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { WebglAddon } from '@xterm/addon-webgl'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

import container from '@/api/panel/container'
import ws from '@/api/ws'
import ContainerCreate from '@/views/container/ContainerCreate.vue'

const { $gettext } = useGettext()

const logModal = ref(false)
const logs = ref('')
const renameModal = ref(false)
const renameModel = ref({
  id: '',
  name: ''
})

// 终端相关状态
const terminalModal = ref(false)
const terminalContainerName = ref('')
const terminalRef = ref<HTMLElement | null>(null)
const term = ref<Terminal | null>(null)
let containerWs: WebSocket | null = null
const fitAddon = new FitAddon()
const webglAddon = new WebglAddon()

const containerCreateModal = ref(false)
const selectedRowKeys = ref<any>([])

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Container Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true }
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
        onUpdateValue: (value: boolean) => {
          if (value) {
            handleStart(row.id)
          } else {
            handleStop(row.id)
          }
        }
      })
    }
  },
  {
    title: $gettext('Image'),
    key: 'image',
    minWidth: 300,
    resizable: true,
    render(row: any): any {
      return h(NTag, null, {
        default: () => row.image
      })
    }
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
                if (port.container_start == port.container_end) {
                  return `${port.host ? port.host + ':' : ''}${port.host_start}->${port.container_start}/${port.protocol}`
                }
                return `${port.host ? port.host + ':' : ''}${port.host_start}-${port.host_end}->${port.container_start}-${port.container_end}/${port.protocol}`
              }
            })
          )
      })
    }
  },
  {
    title: $gettext('Running Status'),
    key: 'status',
    width: 300,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 320,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleOpenTerminal(row),
            disabled: row.state !== 'running'
          },
          {
            default: () => $gettext('Terminal')
          }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            style: 'margin-left: 10px;',
            onClick: () => handleShowLog(row)
          },
          {
            default: () => $gettext('Logs')
          }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'success',
            style: 'margin-left: 10px;',
            onClick: () => {
              renameModel.value.id = row.id
              renameModel.value.name = row.name
              renameModal.value = true
            }
          },
          {
            default: () => $gettext('Rename')
          }
        ),
        h(
          NDropdown,
          {
            options: [
              {
                label: $gettext('Start'),
                key: 'start',
                disabled: row.state === 'running'
              },
              {
                label: $gettext('Stop'),
                key: 'stop',
                disabled: row.state !== 'running'
              },
              {
                label: $gettext('Restart'),
                key: 'restart',
                disabled: row.state !== 'running'
              },
              {
                label: $gettext('Force Stop'),
                key: 'forceStop',
                disabled: row.state !== 'running'
              },
              {
                label: $gettext('Pause'),
                key: 'pause',
                disabled: row.state !== 'running'
              },
              {
                label: $gettext('Resume'),
                key: 'unpause',
                disabled: row.state === 'running'
              },
              {
                label: $gettext('Delete'),
                key: 'delete'
              }
            ],
            onSelect: (key: string) => {
              switch (key) {
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
            }
          },
          {
            default: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'primary',
                  style: 'margin-left: 10px;'
                },
                {
                  default: () => $gettext('More')
                }
              )
            }
          }
        )
      ]
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => container.containerList(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleShowLog = async (row: any) => {
  useRequest(container.containerLogs(row.id)).onSuccess(({ data }) => {
    logs.value = data
    logModal.value = true
  })
}

const handleRename = () => {
  useRequest(container.containerRename(renameModel.value.id, renameModel.value.name)).onSuccess(
    () => {
      refresh()
      renameModal.value = false
      window.$message.success($gettext('Rename successful'))
    }
  )
}

const handleStart = (id: string) => {
  useRequest(container.containerStart(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Start successful'))
  })
}

const handleStop = (id: string) => {
  useRequest(container.containerStop(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Stop successful'))
  })
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
  useRequest(container.containerPrune()).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Cleanup successful'))
  })
}

const bulkStart = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select containers to start'))
    return
  }

  const promises = selectedRowKeys.value.map((id: any) => container.containerStart(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Start successful'))
}

const bulkStop = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select containers to stop'))
    return
  }

  const promises = selectedRowKeys.value.map((id: any) => container.containerStop(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Stop successful'))
}

const bulkRestart = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select containers to restart'))
    return
  }

  const promises = selectedRowKeys.value.map((id: any) => container.containerRestart(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Restart successful'))
}

const bulkForceStop = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select containers to force stop'))
    return
  }

  const promises = selectedRowKeys.value.map((id: any) => container.containerKill(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Force stop successful'))
}

const bulkDelete = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select containers to delete'))
    return
  }

  const promises = selectedRowKeys.value.map((id: any) => container.containerRemove(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Delete successful'))
}

const bulkPause = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select containers to pause'))
    return
  }

  const promises = selectedRowKeys.value.map((id: any) => container.containerPause(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Pause successful'))
}

const bulkUnpause = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select containers to resume'))
    return
  }

  const promises = selectedRowKeys.value.map((id: any) => container.containerUnpause(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Resume successful'))
}

const closeContainerCreateModal = () => {
  refresh()
}

// 打开容器终端
const handleOpenTerminal = async (row: any) => {
  terminalContainerName.value = row.name
  terminalModal.value = true

  // 等待 DOM 更新后初始化终端
  await nextTick()

  // 确保终端容器存在
  if (!terminalRef.value) {
    window.$message.error($gettext('Terminal container not found'))
    return
  }

  try {
    containerWs = await ws.container(row.id)

    term.value = new Terminal({
      allowProposedApi: true,
      lineHeight: 1.2,
      fontSize: 14,
      fontFamily: `'JetBrains Mono Variable', monospace`,
      cursorBlink: true,
      cursorStyle: 'underline',
      tabStopWidth: 4,
      theme: { background: '#111', foreground: '#fff' }
    })

    term.value.loadAddon(new AttachAddon(containerWs))
    term.value.loadAddon(fitAddon)
    term.value.loadAddon(new ClipboardAddon())
    term.value.loadAddon(new WebLinksAddon())
    term.value.loadAddon(new Unicode11Addon())
    term.value.unicode.activeVersion = '11'
    term.value.loadAddon(webglAddon)
    webglAddon.onContextLoss(() => {
      webglAddon.dispose()
    })
    term.value.open(terminalRef.value)

    onTerminalResize()
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
    if (term.value) {
      term.value.dispose()
      term.value = null
    }
    if (containerWs) {
      containerWs.close()
      containerWs = null
    }
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
  fitAddon.fit()
  if (containerWs != null && containerWs.readyState === 1 && term.value) {
    const { cols, rows } = term.value
    containerWs.send(
      JSON.stringify({
        resize: true,
        columns: cols,
        rows: rows
      })
    )
  }
}

// 终端滚轮缩放
const onTerminalWheel = (event: WheelEvent) => {
  if (event.ctrlKey && term.value) {
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
    <n-flex>
      <n-button type="primary" @click="containerCreateModal = true">{{
        $gettext('Create Container')
      }}</n-button>
      <n-button type="primary" @click="handlePrune" ghost>{{
        $gettext('Cleanup Containers')
      }}</n-button>
      <n-button-group>
        <n-button @click="bulkStart">{{ $gettext('Start') }}</n-button>
        <n-button @click="bulkStop">{{ $gettext('Stop') }}</n-button>
        <n-button @click="bulkRestart">{{ $gettext('Restart') }}</n-button>
        <n-button @click="bulkForceStop">{{ $gettext('Force Stop') }}</n-button>
        <n-button @click="bulkPause">{{ $gettext('Pause') }}</n-button>
        <n-button @click="bulkUnpause">{{ $gettext('Resume') }}</n-button>
        <n-button @click="bulkDelete">{{ $gettext('Delete') }}</n-button>
      </n-button-group>
    </n-flex>
    <n-data-table
      striped
      remote
      :loading="loading"
      :scroll-x="1000"
      :data="data"
      :columns="columns"
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
    style="width: 80vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <common-editor v-model:value="logs" height="60vh" read-only />
  </n-modal>
  <n-modal
    v-model:show="renameModal"
    preset="card"
    :title="$gettext('Rename')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
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
    <n-button type="info" block @click="handleRename">{{ $gettext('Submit') }}</n-button>
  </n-modal>
  <n-modal
    v-model:show="terminalModal"
    preset="card"
    :title="$gettext('Terminal') + ' - ' + terminalContainerName"
    style="width: 90vw; height: 80vh"
    size="huge"
    :bordered="false"
    :segmented="false"
    @after-leave="handleTerminalModalClose"
  >
    <div
      ref="terminalRef"
      @wheel="onTerminalWheel"
      style="height: 100%; min-height: 60vh; background: #111"
    ></div>
  </n-modal>
  <ContainerCreate v-model:show="containerCreateModal" @update:show="closeContainerCreateModal" />
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

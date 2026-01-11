<script setup lang="ts">
defineOptions({
  name: 'ssh-index'
})

import ssh from '@/api/panel/ssh'
import ws from '@/api/ws'
import CreateModal from '@/views/ssh/CreateModal.vue'
import UpdateModal from '@/views/ssh/UpdateModal.vue'
import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import { FitAddon } from '@xterm/addon-fit'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { WebglAddon } from '@xterm/addon-webgl'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { NButton, NFlex, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const terminal = ref<HTMLElement | null>(null)
const term = ref<Terminal | null>(null)
let sshWs: WebSocket | null = null
let fitAddon: FitAddon | null = null
let webglAddon: WebglAddon | null = null

const current = ref(0)
const collapsed = ref(true)
const create = ref(false)
const update = ref(false)
const updateId = ref(0)

const list = ref<any[]>([])

const fetchData = async () => {
  list.value = []
  const data = await ssh.list(1, 10000)
  if (data.items.length === 0) {
    window.$message.info($gettext('Please create a host first'))
    return
  }
  data.items.forEach((item: any) => {
    list.value.push({
      label: item.name === '' ? item.host : item.name,
      key: item.id,
      extra: () => {
        return h(
          NFlex,
          {
            size: 'small',
            style: 'float: right'
          },
          {
            default: () => [
              h(
                NButton,
                {
                  type: 'primary',
                  size: 'small',
                  onClick: () => {
                    update.value = true
                    updateId.value = item.id
                  }
                },
                {
                  default: () => {
                    return $gettext('Edit')
                  }
                }
              ),
              h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleDelete(item.id)
                },
                {
                  default: () => {
                    return $gettext('Are you sure you want to delete this host?')
                  },
                  trigger: () => {
                    return h(
                      NButton,
                      {
                        size: 'small',
                        type: 'error'
                      },
                      {
                        default: () => {
                          return $gettext('Delete')
                        }
                      }
                    )
                  }
                }
              )
            ]
          }
        )
      }
    })
  })
  await openSession(updateId.value === 0 ? Number(list.value[0].key) : updateId.value)
}

const handleDelete = (id: number) => {
  useRequest(ssh.delete(id)).onSuccess(() => {
    list.value = list.value.filter((item: any) => item.key !== id)
    if (current.value === id) {
      if (list.value.length > 0) {
        openSession(Number(list.value[0].key))
      } else {
        term.value?.dispose()
      }
      if (list.value.length === 0) {
        create.value = true
      }
    }
  })
}

const handleChange = (key: number) => {
  openSession(key)
}

const openSession = async (id: number) => {
  closeSession()
  await ws.ssh(id).then((socket) => {
    sshWs = socket
    sshWs.binaryType = 'arraybuffer'

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
    term.value.open(terminal.value!)

    sshWs.onmessage = (ev) => {
      const data: ArrayBuffer | string = ev.data
      term.value?.write(typeof data === 'string' ? data : new Uint8Array(data))
    }
    term.value?.onData((data) => {
      if (sshWs?.readyState === WebSocket.OPEN) {
        sshWs?.send(data)
      }
    })
    term.value?.onBinary((data) => {
      if (sshWs?.readyState === WebSocket.OPEN) {
        const buffer = new Uint8Array(data.length)
        for (let i = 0; i < data.length; ++i) {
          buffer[i] = data.charCodeAt(i) & 255
        }
        sshWs?.send(buffer)
      }
    })
    term.value.onResize(({ rows, cols }) => {
      if (sshWs?.readyState === WebSocket.OPEN) {
        sshWs?.send(
          JSON.stringify({
            resize: true,
            columns: cols,
            rows: rows
          })
        )
      }
    })

    fitAddon.fit()
    term.value.focus()
    window.addEventListener('resize', onResize, false)
    current.value = id

    sshWs.onclose = () => {
      term.value?.write('\r\n' + $gettext('Connection closed. Please refresh.'))
      window.removeEventListener('resize', onResize)
    }

    sshWs.onerror = (event) => {
      term.value?.write('\r\n' + $gettext('Connection error. Please refresh.'))
      console.error(event)
      sshWs?.close()
    }
  })
}

const closeSession = () => {
  try {
    if (sshWs) {
      sshWs.close()
      sshWs = null
    }
    if (term.value) {
      term.value.dispose()
      term.value = null
    }
    fitAddon = null
    webglAddon = null
    if (terminal.value) {
      terminal.value.innerHTML = ''
    }
  } catch {
    /* empty */
  }
}

const onResize = () => {
  if (fitAddon && term.value) {
    fitAddon.fit()
  }
}

const onTermWheel = (event: WheelEvent) => {
  if (event.ctrlKey && term.value && fitAddon) {
    event.preventDefault()
    const fontSize = term.value.options.fontSize ?? 14
    if (event.deltaY > 0) {
      if (fontSize > 12) {
        term.value.options.fontSize = fontSize - 1
      }
    } else {
      term.value.options.fontSize = fontSize + 1
    }
    fitAddon.fit()
  }
}

onMounted(() => {
  // https://github.com/xtermjs/xterm.js/pull/5178
  document.fonts.ready.then((fontFaceSet: any) =>
    Promise.all(Array.from(fontFaceSet).map((el: any) => el.load())).then(fetchData)
  )
  window.$bus.on('ssh:refresh', fetchData)
})

onUnmounted(() => {
  closeSession()
  window.$bus.off('ssh:refresh')
})
</script>

<template>
  <common-page show-footer>
    <n-layout has-sider sider-placement="right">
      <n-layout content-style="overflow: visible" bg-hex-111>
        <div ref="terminal" @wheel="onTermWheel" h-75vh></div>
      </n-layout>
      <n-layout-sider
        bordered
        :collapsed-width="0"
        :collapsed="collapsed"
        show-trigger
        :native-scrollbar="false"
        @collapse="collapsed = true"
        @expand="collapsed = false"
        @after-enter="onResize"
        @after-leave="onResize"
        pl-10
      >
        <div class="text-center">
          <n-button type="primary" @click="create = true">
            {{ $gettext('Create Host') }}
          </n-button>
        </div>
        <n-menu
          v-model:value="current"
          :collapsed="collapsed"
          :collapsed-width="0"
          :collapsed-icon-size="0"
          :options="list"
          @update-value="handleChange"
        />
      </n-layout-sider>
    </n-layout>
  </common-page>
  <create-modal v-model:show="create" />
  <update-modal v-model:show="update" v-model:id="updateId" />
</template>

<style scoped lang="scss">
:deep(.xterm) {
  padding: 4rem !important;
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

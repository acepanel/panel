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
import { useGettext } from 'vue3-gettext'

import ws from '@/api/ws'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps({
  title: {
    type: String,
    default: ''
  },
  command: {
    type: String,
    required: true
  }
})

const emit = defineEmits<{
  (e: 'complete'): void
  (e: 'error', error: string): void
}>()

const isRunning = ref(false)
const terminalRef = ref<HTMLElement | null>(null)
const term = ref<Terminal | null>(null)
let ptyWs: WebSocket | null = null
let fitAddon: FitAddon | null = null
let webglAddon: WebglAddon | null = null

// 初始化终端
const initTerminal = async () => {
  if (!terminalRef.value || !props.command) {
    return
  }

  isRunning.value = true

  try {
    ptyWs = await ws.pty(props.command)
    ptyWs.binaryType = 'arraybuffer'

    term.value = new Terminal({
      allowProposedApi: true,
      lineHeight: 1.2,
      fontSize: 14,
      fontFamily: `'JetBrains Mono Variable', monospace`,
      cursorBlink: true,
      cursorStyle: 'underline',
      tabStopWidth: 4,
      disableStdin: false,
      convertEol: true,
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
    term.value.open(terminalRef.value)

    ptyWs.onmessage = (ev) => {
      const data: ArrayBuffer | string = ev.data
      term.value?.write(typeof data === 'string' ? data : new Uint8Array(data))
    }

    term.value?.onData((data) => {
      if (ptyWs?.readyState === WebSocket.OPEN) {
        ptyWs?.send(data)
      }
    })
    term.value?.onBinary((data) => {
      if (ptyWs?.readyState === WebSocket.OPEN) {
        const buffer = new Uint8Array(data.length)
        for (let i = 0; i < data.length; ++i) {
          buffer[i] = data.charCodeAt(i) & 255
        }
        ptyWs?.send(buffer)
      }
    })
    term.value.onResize(({ rows, cols }) => {
      if (ptyWs && ptyWs.readyState === WebSocket.OPEN) {
        ptyWs.send(
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
    window.addEventListener('resize', onTerminalResize, false)

    ptyWs.onclose = () => {
      isRunning.value = false
      if (term.value) {
        term.value.write('\r\n' + $gettext('Connection closed.'))
      }
      window.removeEventListener('resize', onTerminalResize)
      emit('complete')
    }

    ptyWs.onerror = (event) => {
      isRunning.value = false
      if (term.value) {
        term.value.write('\r\n' + $gettext('Connection error.'))
      }
      console.error(event)
      ptyWs?.close()
      emit('error', $gettext('Connection error'))
    }
  } catch (error) {
    console.error('Failed to start PTY:', error)
    isRunning.value = false
    emit('error', $gettext('Failed to connect'))
  }
}

// 关闭终端
const closeTerminal = () => {
  try {
    if (ptyWs) {
      ptyWs.close()
      ptyWs = null
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

// 处理窗口大小变化
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

// 模态框关闭后清理
const handleModalClose = () => {
  closeTerminal()
  isRunning.value = false
}

// 处理关闭前确认
const handleBeforeClose = (): Promise<boolean> => {
  return new Promise((resolve) => {
    if (isRunning.value) {
      window.$dialog.warning({
        title: $gettext('Confirm'),
        content: $gettext(
          'Command may still running. Closing the window will terminate the command. Are you sure?'
        ),
        positiveText: $gettext('Confirm'),
        negativeText: $gettext('Cancel'),
        onPositiveClick: () => {
          resolve(true)
        },
        onNegativeClick: () => {
          resolve(false)
        },
        onClose: () => {
          resolve(false)
        },
        onMaskClick: () => {
          resolve(false)
        }
      })
    } else {
      resolve(true)
    }
  })
}

// 处理遮罩点击
const handleMaskClick = async () => {
  if (await handleBeforeClose()) {
    show.value = false
  }
}

// 监听 show 变化，自动初始化终端
watch(
  () => show.value,
  async (newVal) => {
    if (newVal) {
      await nextTick()
      await initTerminal()
    }
  }
)

onUnmounted(() => {
  closeTerminal()
})

defineExpose({
  initTerminal,
  closeTerminal
})
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="title || $gettext('Terminal')"
    style="width: 90vw; height: 80vh"
    size="huge"
    :bordered="false"
    :segmented="false"
    :mask-closable="false"
    :closable="true"
    :on-close="handleBeforeClose"
    @mask-click="handleMaskClick"
    @after-leave="handleModalClose"
  >
    <div
      ref="terminalRef"
      @wheel="onTerminalWheel"
      style="height: 100%; min-height: 60vh; background: #111"
    ></div>
  </n-modal>
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

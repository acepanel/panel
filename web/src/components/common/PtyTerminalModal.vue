<script setup lang="ts">
import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import { FitAddon } from '@xterm/addon-fit'
import { Unicode11Addon } from '@xterm/addon-unicode11'
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

    fitAddon = new FitAddon()
    webglAddon = new WebglAddon()

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

    term.value.loadAddon(fitAddon)
    term.value.loadAddon(new Unicode11Addon())
    term.value.unicode.activeVersion = '11'
    term.value.loadAddon(webglAddon)
    webglAddon.onContextLoss(() => {
      webglAddon?.dispose()
    })
    term.value.open(terminalRef.value)
    fitAddon.fit()

    // 发送初始窗口大小
    sendResize()

    // 监听终端大小变化
    term.value.onResize(({ rows, cols }) => {
      if (ptyWs && ptyWs.readyState === WebSocket.OPEN) {
        ptyWs.send(JSON.stringify({ type: 'resize', rows, cols }))
      }
    })

    // 转发用户输入到 WebSocket
    term.value.onData((data) => {
      if (ptyWs && ptyWs.readyState === WebSocket.OPEN) {
        ptyWs.send(data)
      }
    })

    // 处理 WebSocket 消息
    ptyWs.binaryType = 'arraybuffer'
    ptyWs.onmessage = (event) => {
      if (term.value) {
        const data =
          event.data instanceof ArrayBuffer
            ? new TextDecoder().decode(event.data)
            : event.data
        term.value.write(data)
      }
    }

    ptyWs.onclose = () => {
      isRunning.value = false
      if (term.value) {
        term.value.write('\r\n' + $gettext('Connection closed.'))
      }
      emit('complete')
    }

    ptyWs.onerror = (event) => {
      isRunning.value = false
      if (term.value) {
        term.value.write('\r\n' + $gettext('Connection error.'))
      }
      console.error(event)
      emit('error', $gettext('Connection error'))
    }
  } catch (error) {
    console.error('Failed to start PTY:', error)
    isRunning.value = false
    emit('error', $gettext('Failed to connect'))
  }
}

// 发送窗口大小到后端
const sendResize = () => {
  if (term.value && ptyWs && ptyWs.readyState === WebSocket.OPEN) {
    const { rows, cols } = term.value
    ptyWs.send(JSON.stringify({ type: 'resize', rows, cols }))
  }
}

// 处理窗口大小变化
const handleResize = () => {
  if (fitAddon && term.value) {
    fitAddon.fit()
    // fit() 会触发 term.onResize，所以不需要手动发送 resize
  }
}

// 关闭终端
const closeTerminal = () => {
  try {
    if (term.value) {
      term.value.dispose()
      term.value = null
    }
    if (ptyWs) {
      ptyWs.close()
      ptyWs = null
    }
    if (terminalRef.value) {
      terminalRef.value.innerHTML = ''
    }
    fitAddon = null
    webglAddon = null
  } catch {
    /* empty */
  }
}

// 终端滚轮缩放
const onTerminalWheel = (event: WheelEvent) => {
  if (event.ctrlKey && term.value && fitAddon) {
    event.preventDefault()
    const currentFontSize = term.value.options.fontSize ?? 14
    if (event.deltaY > 0) {
      if (currentFontSize > 12) {
        term.value.options.fontSize = currentFontSize - 1
      }
    } else {
      term.value.options.fontSize = currentFontSize + 1
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
        content: $gettext('Command is still running. Closing the window will terminate the command. Are you sure?'),
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
      initTerminal()
      // 添加窗口 resize 监听
      window.addEventListener('resize', handleResize)
    } else {
      // 移除窗口 resize 监听
      window.removeEventListener('resize', handleResize)
    }
  }
)

onUnmounted(() => {
  closeTerminal()
  window.removeEventListener('resize', handleResize)
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

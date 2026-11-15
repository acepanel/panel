<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import ws from '@/api/ws'
import { createXterm } from '@/components/common/composables/useXterm'
import type { Xterm } from '@/components/common/composables/useXterm'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps({
  title: {
    type: String,
    default: '',
  },
  command: {
    type: String,
    required: true,
  },
})

const emit = defineEmits<{
  (e: 'complete'): void
  (e: 'error', error: string): void
}>()

const isRunning = ref(false)
const terminalRef = ref<HTMLElement | null>(null)
let xterm: Xterm | null = null

// 初始化终端
const initTerminal = async () => {
  const el = terminalRef.value
  if (!el || !props.command) {
    return
  }

  isRunning.value = true

  try {
    xterm = createXterm(el, await ws.pty(props.command), {
      convertEol: true,
      onClose: () => {
        isRunning.value = false
        emit('complete')
      },
      onError: () => emit('error', $gettext('Connection error')),
    })
  } catch (error) {
    console.error('Failed to start PTY:', error)
    isRunning.value = false
    emit('error', $gettext('Failed to connect'))
  }
}

// 关闭终端
const closeTerminal = () => {
  xterm?.dispose()
  xterm = null
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
          'Command may still running. Closing the window will terminate the command. Are you sure?',
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
        },
      })
    } else {
      resolve(true)
    }
  })
}

// 监听 show 变化，自动初始化终端
watch(
  () => show.value,
  async (newVal) => {
    if (newVal) {
      await nextTick()
      await initTerminal()
    }
  },
)

onUnmounted(() => {
  closeTerminal()
})

defineExpose({
  initTerminal,
  closeTerminal,
})
</script>

<template>
  <app-modal
    v-model:show="show"
    size="full"
    :title="title || $gettext('Terminal')"
    style="height: 80vh; max-height: 80vh"
    :content-style="{
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
      flex: 1,
      minHeight: 0,
    }"
    :mask-closable="false"
    :close-on-esc="false"
    :closable="true"
    :on-close="handleBeforeClose"
    @after-leave="handleModalClose"
  >
    <div class="terminal-shell flex-1 min-h-0">
      <header class="terminal-titlebar">
        <div class="terminal-title">
          <span class="status-dot" :class="isRunning ? 'ok' : 'err'"></span>
          <span class="terminal-title-text">{{ command }}</span>
        </div>
      </header>
      <div ref="terminalRef" class="terminal-content"></div>
    </div>
  </app-modal>
</template>

<style scoped lang="scss">
.terminal-shell {
  display: flex;
  flex-direction: column;
  background: var(--color-bg-terminal);
  border: 1px solid var(--color-border-default);
  border-radius: 3px;
  overflow: hidden;
}

.terminal-titlebar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;
}

.terminal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.65);
  font-family: 'JetBrains Mono Variable', monospace;
}

.terminal-title-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;

  &.ok {
    background: #22c55e;
    box-shadow: 0 0 6px rgba(34, 197, 94, 0.5);
  }

  &.err {
    background: #ef4444;
  }
}

.terminal-content {
  flex: 1;
  min-height: 0;
  background: var(--color-bg-terminal);
}

:deep(.xterm) {
  padding: 12px !important;
  height: 100%;
}

:deep(.xterm-viewport) {
  background-color: var(--color-bg-terminal) !important;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.18) transparent;

  &::-webkit-scrollbar {
    width: 8px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.18);
    border-radius: 4px;

    &:hover {
      background: rgba(255, 255, 255, 0.28);
    }
  }
}
</style>

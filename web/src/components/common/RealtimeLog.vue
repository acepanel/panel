<script setup lang="ts">
import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import Anser from 'anser'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import ws from '@/api/ws'

const { $gettext } = useGettext()
const props = defineProps({
  path: {
    type: String,
    required: false,
  },
  service: {
    type: String,
    required: false,
  },
})

interface LogLine {
  id: number
  html: string
  raw: string
}

const lines = ref<LogLine[]>([])
const followMode = ref(true)
const isLoadingMore = ref(false)
const hasMore = ref(false)
const fontSize = ref(13)
const connected = ref(false)
const searchKeyword = ref('')
const scrollEl = ref<HTMLElement | null>(null)

let nextId = 0
let pendingTail = ''
let loadedFromEnd = 0
let followWs: WebSocket | null = null
let suppressScrollHandler = false
let isManuallyClosed = false
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let lastSearchIndex = -1

const sourceParams = computed(() =>
  props.path ? { path: props.path } : props.service ? { service: props.service } : null,
)

const titleLabel = computed(() => props.path || props.service || '')

const supported = computed(() => !!sourceParams.value)

const parseLine = (raw: string): LogLine => ({
  id: nextId++,
  raw,
  html: Anser.ansiToHtml(Anser.escapeForHtml(raw), { use_classes: true }),
})

const scrollToBottom = () => {
  const el = scrollEl.value
  if (!el) return
  suppressScrollHandler = true
  el.scrollTop = el.scrollHeight
  requestAnimationFrame(() => {
    suppressScrollHandler = false
  })
}

const scrollToTop = () => {
  const el = scrollEl.value
  if (el) el.scrollTop = 0
}

const scheduleReconnect = () => {
  if (isManuallyClosed) return
  if (reconnectTimer) clearTimeout(reconnectTimer)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    if (!isManuallyClosed) startFollow()
  }, 3000)
}

const startFollow = () => {
  if (!sourceParams.value) return
  isManuallyClosed = false
  ws.follow(sourceParams.value)
    .then((socket) => {
      followWs = socket
      socket.binaryType = 'arraybuffer'
      connected.value = true

      socket.onmessage = (ev) => {
        const data: string =
          typeof ev.data === 'string' ? ev.data : new TextDecoder().decode(new Uint8Array(ev.data))
        const combined = pendingTail + data
        const parts = combined.split('\n')
        pendingTail = parts.pop() ?? ''
        if (parts.length > 0) {
          lines.value.push(...parts.map(parseLine))
          if (followMode.value) {
            nextTick(() => scrollToBottom())
          }
        }
      }

      socket.onclose = () => {
        connected.value = false
        scheduleReconnect()
      }

      socket.onerror = () => {
        connected.value = false
        socket.close()
      }
    })
    .catch(() => {
      connected.value = false
      scheduleReconnect()
    })
}

const PAGE_SIZE = 100

const loadInitial = () => {
  if (!sourceParams.value) return
  useRequest(file.tail({ ...sourceParams.value, offset: 0, limit: PAGE_SIZE })).onSuccess(
    ({ data }: any) => {
      const newLines: string[] = data?.lines ?? []
      lines.value = newLines.map(parseLine)
      loadedFromEnd = newLines.length
      hasMore.value = data?.has_more ?? false
      nextTick(() => {
        scrollToBottom()
        followMode.value = true
        startFollow()
      })
    },
  )
}

const loadOlder = () => {
  if (!sourceParams.value || isLoadingMore.value || !hasMore.value) return
  isLoadingMore.value = true
  const el = scrollEl.value
  const oldScrollTop = el?.scrollTop ?? 0
  const oldScrollHeight = el?.scrollHeight ?? 0

  useRequest(file.tail({ ...sourceParams.value, offset: loadedFromEnd, limit: PAGE_SIZE }))
    .onSuccess(({ data }: any) => {
      const newOldLines: string[] = data?.lines ?? []
      if (newOldLines.length === 0) {
        hasMore.value = false
        return
      }
      lines.value.unshift(...newOldLines.map(parseLine))
      loadedFromEnd += newOldLines.length
      hasMore.value = data?.has_more ?? false
      // 保持视觉位置:scrollTop = 新 scrollHeight - 旧 scrollHeight + 旧 scrollTop
      nextTick(() => {
        const target = scrollEl.value
        if (target) {
          suppressScrollHandler = true
          target.scrollTop = target.scrollHeight - oldScrollHeight + oldScrollTop
          requestAnimationFrame(() => {
            suppressScrollHandler = false
          })
        }
      })
    })
    .onComplete(() => {
      isLoadingMore.value = false
    })
}

const onScroll = () => {
  if (suppressScrollHandler) return
  const el = scrollEl.value
  if (!el) return
  const { scrollTop, scrollHeight, clientHeight } = el
  followMode.value = scrollHeight - scrollTop - clientHeight < 30
  if (scrollTop < 60 && hasMore.value && !isLoadingMore.value) {
    loadOlder()
  }
}

const toggleFollow = () => {
  if (followMode.value) {
    followMode.value = false
  } else {
    scrollToBottom()
    followMode.value = true
  }
}

const increaseFont = () => {
  if (fontSize.value < 24) fontSize.value++
}

const decreaseFont = () => {
  if (fontSize.value > 10) fontSize.value--
}

const handleSearch = () => {
  if (!searchKeyword.value || !scrollEl.value) return
  const kw = searchKeyword.value.toLowerCase()
  const startIdx = lastSearchIndex + 1
  // 从上次匹配位置之后查找,找不到再从头查(回环)
  let idx = lines.value.findIndex((l, i) => i >= startIdx && l.raw.toLowerCase().includes(kw))
  if (idx < 0) {
    idx = lines.value.findIndex((l) => l.raw.toLowerCase().includes(kw))
  }
  if (idx >= 0) {
    lastSearchIndex = idx
    const lineEls = scrollEl.value.querySelectorAll('.log-line')
    const target = lineEls[idx] as HTMLElement | undefined
    if (target) {
      target.scrollIntoView({ block: 'center' })
      followMode.value = false
    }
  } else {
    window.$message.warning($gettext('Not found'))
  }
}

// 关键字变化时重置搜索游标
watch(searchKeyword, () => {
  lastSearchIndex = -1
})

const cleanup = () => {
  isManuallyClosed = true
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  followWs?.close()
  followWs = null
  lines.value = []
  loadedFromEnd = 0
  pendingTail = ''
  hasMore.value = false
  connected.value = false
  lastSearchIndex = -1
}

watch(
  () => [props.path, props.service],
  () => {
    cleanup()
    nextTick(() => loadInitial())
  },
)

onMounted(() => {
  loadInitial()
})

onUnmounted(() => {
  cleanup()
})

const clear = () => {
  cleanup()
  nextTick(() => loadInitial())
}

defineExpose({ clear })
</script>

<template>
  <div v-if="supported" class="log-shell">
    <header class="log-titlebar">
      <div class="log-title">
        <span class="status-dot" :class="connected ? 'ok' : 'err'"></span>
        <span class="log-title-text">{{ titleLabel }}</span>
      </div>
      <div class="titlebar-actions">
        <button
          class="action-btn"
          :class="{ active: followMode }"
          :title="followMode ? $gettext('Pause Auto-scroll') : $gettext('Resume Auto-scroll')"
          @click="toggleFollow"
        >
          <i-mdi-pause v-if="followMode" class="text-base" />
          <i-mdi-play v-else class="text-base" />
        </button>
        <button class="action-btn" :title="$gettext('Jump to Top')" @click="scrollToTop">
          <i-mdi-arrow-collapse-up class="text-base" />
        </button>
        <button class="action-btn" :title="$gettext('Jump to Bottom')" @click="scrollToBottom">
          <i-mdi-arrow-collapse-down class="text-base" />
        </button>
        <n-popover trigger="click" placement="bottom-end">
          <template #trigger>
            <button class="action-btn" :title="$gettext('Search')">
              <i-mdi-magnify class="text-base" />
            </button>
          </template>
          <n-input
            v-model:value="searchKeyword"
            size="small"
            :placeholder="$gettext('Enter keyword and press Enter')"
            class="!w-60"
            @keyup.enter="handleSearch"
          />
        </n-popover>
        <div class="action-divider"></div>
        <button class="action-btn" :title="$gettext('Decrease Font Size')" @click="decreaseFont">
          <i-mdi-format-font-size-decrease class="text-base" />
        </button>
        <button class="action-btn" :title="$gettext('Increase Font Size')" @click="increaseFont">
          <i-mdi-format-font-size-increase class="text-base" />
        </button>
      </div>
    </header>

    <div
      ref="scrollEl"
      class="log-content"
      :style="{ fontSize: `${fontSize}px` }"
      @scroll="onScroll"
    >
      <div v-for="line in lines" :key="line.id" class="log-line" v-html="line.html"></div>
    </div>
  </div>
  <n-empty v-else :description="$gettext('No logs available')" />
</template>

<style lang="scss">
/* anser 默认 use_classes 输出的 ANSI 颜色类名,需要全局可达 */
.log-line {
  .ansi-black-fg {
    color: #3f3f3f;
  }
  .ansi-red-fg {
    color: #c91b00;
  }
  .ansi-green-fg {
    color: #00c200;
  }
  .ansi-yellow-fg {
    color: #c7c400;
  }
  .ansi-blue-fg {
    color: #2f80ed;
  }
  .ansi-magenta-fg {
    color: #c930c7;
  }
  .ansi-cyan-fg {
    color: #00c5c7;
  }
  .ansi-white-fg {
    color: #c7c7c7;
  }
  .ansi-bright-black-fg {
    color: #686868;
  }
  .ansi-bright-red-fg {
    color: #ff6e67;
  }
  .ansi-bright-green-fg {
    color: #5ffa68;
  }
  .ansi-bright-yellow-fg {
    color: #fffc67;
  }
  .ansi-bright-blue-fg {
    color: #6871ff;
  }
  .ansi-bright-magenta-fg {
    color: #ff77ff;
  }
  .ansi-bright-cyan-fg {
    color: #60fdff;
  }
  .ansi-bright-white-fg {
    color: #ffffff;
  }

  .ansi-black-bg {
    background-color: #3f3f3f;
  }
  .ansi-red-bg {
    background-color: #c91b00;
  }
  .ansi-green-bg {
    background-color: #00c200;
  }
  .ansi-yellow-bg {
    background-color: #c7c400;
  }
  .ansi-blue-bg {
    background-color: #2f80ed;
  }
  .ansi-magenta-bg {
    background-color: #c930c7;
  }
  .ansi-cyan-bg {
    background-color: #00c5c7;
  }
  .ansi-white-bg {
    background-color: #c7c7c7;
  }

  .ansi-bold {
    font-weight: bold;
  }
  .ansi-italic {
    font-style: italic;
  }
  .ansi-underline {
    text-decoration: underline;
  }
  .ansi-strikethrough {
    text-decoration: line-through;
  }
  .ansi-dim {
    opacity: 0.6;
  }
}
</style>

<style scoped lang="scss">
.log-shell {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 60vh;
  min-height: 300px;
  background: var(--color-bg-terminal);
  border: 1px solid var(--color-border-default);
  border-radius: 3px;
  overflow: hidden;
  box-shadow: var(--shadow-md);
  color: #e6edf3;
}

.log-titlebar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;
}

.log-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.65);
  font-family: 'JetBrains Mono Variable', monospace;
}

.log-title-text {
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

.titlebar-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 3px;
  background: transparent;
  border: none;
  color: rgba(255, 255, 255, 0.55);
  cursor: pointer;
  transition: all 150ms ease;

  &:hover {
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.95);
  }

  &.active {
    background: rgba(34, 197, 94, 0.18);
    color: rgb(74, 222, 128);
  }
}

.action-divider {
  width: 1px;
  height: 16px;
  background: rgba(255, 255, 255, 0.1);
  margin: 0 4px;
}

.log-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: auto;
  font-family: 'JetBrains Mono Variable', monospace;
  font-variant-numeric: tabular-nums;
  line-height: 1.5;
  padding: 4px 0;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.18) transparent;

  &::-webkit-scrollbar {
    width: 8px;
    height: 8px;
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

.log-line {
  padding: 0 12px;
  white-space: pre;
  user-select: text;
  cursor: text;

  &:hover {
    background: rgba(255, 255, 255, 0.03);
  }
}
</style>

<script setup lang="ts">
import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import copy2clipboard from '@vavt/copy2clipboard'
import Anser from 'anser'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import ws from '@/api/ws'

const { $gettext, $ngettext } = useGettext()
const props = defineProps({
  path: {
    type: String,
    required: false,
  },
  service: {
    type: String,
    required: false,
  },
  container: {
    type: String,
    required: false,
  },
})

interface LogLine {
  id: number
  html: string
  text: string
  // wrap 模式下的实测行高，未测量按单行估计
  h?: number
}

type ConnStatus = 'connecting' | 'connected' | 'error'

// 实时追加的最大保留行数
const MAX_LINES = 5000
// 视口外上下各多渲染的行数，吸收快速滚动时的空白
const OVERSCAN = 15

const lines = ref<LogLine[]>([])
const followMode = ref(true)
const initialLoading = ref(true)
const isLoadingMore = ref(false)
const hasMore = ref(false)
// 仅在用户翻过历史后才提示到顶，避免首屏日志不足一页时凭空出现边界提示
const loadedOlder = ref(false)
const fontSize = useStorage('log-font-size', 13)
const wrapLines = useStorage('log-wrap-lines', false)
const status = ref<ConnStatus>('connecting')
const searchKeyword = ref('')
const matchedLineId = ref<number | null>(null)
const pendingNew = ref(0)
const scrollEl = ref<HTMLElement | null>(null)
const windowEl = ref<HTMLElement | null>(null)
const shellEl = ref<HTMLElement | null>(null)

const { isFullscreen, toggle: toggleFullscreen } = useFullscreen(shellEl)

const statusText = computed(() =>
  status.value === 'connected'
    ? $gettext('Connected')
    : status.value === 'connecting'
      ? $gettext('Connecting...')
      : $gettext('Connection failed, retrying...'),
)

// 弹出层挂进组件内部：挂 body 在全屏时不可见，在 n-modal 内还会被其焦点陷阱抢走输入焦点
const popoverTo = computed(() => shellEl.value ?? 'body')

const decoder = new TextDecoder()

let nextId = 0
let pendingTail = ''
// 帧内攒下的原始行，由 flushIncoming 统一落盘
let incoming: string[] = []
let flushScheduled = false
let loadedFromEnd = 0
// 首屏时的文件大小，作为反向翻页与实时跟踪的共同锚点
let anchorSize = 0
let nextCursor = ''
let followWs: WebSocket | null = null
let suppressScrollHandler = false
// cleanup 时自增，此前发出的请求回调与 socket 据此作废
let session = 0
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

const sourceParams = computed(() =>
  props.path
    ? { path: props.path }
    : props.service
      ? { service: props.service }
      : props.container
        ? { container: props.container }
        : null,
)

const titleLabel = computed(() => props.path || props.service || props.container || '')

const supported = computed(() => !!sourceParams.value)

const NON_CSI_ESCAPE =
  // oxlint-disable-next-line no-control-regex
  /\x1b[\]PX^_][\s\S]*?(?:\x07|\x1b\\|$)|\x1b[\x20-\x2f]+[\x30-\x7e]|\x1b[\x30-\x5a\x5c-\x7e]|\x1b$/g

const normalizeLine = (raw: string) => {
  let line = raw.replace(/\r+$/, '')
  line = line.slice(line.lastIndexOf('\r') + 1).replace(NON_CSI_ESCAPE, '')
  if (!line.includes('\b')) return line
  const out: string[] = []
  for (const ch of line) {
    if (ch === '\b') out.pop()
    else out.push(ch)
  }
  return out.join('')
}

// xterm 256 色的 16–231 为 6×6×6 色立方、232–255 为灰阶；0–15 Anser 已映射成标准色类名
const paletteColor = (n: number) => {
  if (n >= 232) {
    const v = 8 + (n - 232) * 10
    return `rgb(${v}, ${v}, ${v})`
  }
  const level = (x: number) => (x ? 55 + x * 40 : 0)
  const c = n - 16
  return `rgb(${level(Math.floor(c / 36))}, ${level(Math.floor(c / 6) % 6)}, ${level(c % 6)})`
}

// Anser 把 256 色和真彩色输出成没有样式的类名与 data 属性，转成行内颜色
const inlineColors = (html: string) =>
  html.replace(
    /<span class="([^"]*)"((?: data-ansi-truecolor-(?:fg|bg)="[^"]*")*)>/g,
    (tag, classes: string, attrs: string) => {
      const styles: string[] = []
      for (const [, n, kind] of classes.matchAll(/ansi-palette-(\d+)-(fg|bg)/g)) {
        styles.push(`${kind === 'fg' ? 'color' : 'background-color'}: ${paletteColor(Number(n))}`)
      }
      for (const [, kind, rgb] of attrs.matchAll(/data-ansi-truecolor-(fg|bg)="([^"]*)"/g)) {
        styles.push(`${kind === 'fg' ? 'color' : 'background-color'}: rgb(${rgb})`)
      }
      return styles.length ? `<span class="${classes}" style="${styles.join('; ')}">` : tag
    },
  )

// text 为剥离 ANSI 后的纯文本供搜索/复制/关键词标注使用
// 行创建后 html/text 不再变更，markRaw 免掉每行一层 Proxy 与逐字段依赖（5000 行量级下省数 MB）
const parseLine = (raw: string): LogLine => {
  const line = normalizeLine(raw)
  return markRaw({
    id: nextId++,
    text: Anser.ansiToText(line),
    html: inlineColors(Anser.ansiToHtml(Anser.escapeForHtml(line), { use_classes: true })),
  })
}

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

// 命中行额外标出关键词舍弃该行 ANSI 着色换取定位清晰
const renderLine = (line: LogLine) => {
  const kw = searchKeyword.value
  if (!kw || line.id !== matchedLineId.value) return line.html
  const re = new RegExp(escapeRegExp(Anser.escapeForHtml(kw)), 'gi')
  return Anser.escapeForHtml(line.text).replace(re, (m) => `<mark class="log-mark">${m}</mark>`)
}

// ---------- 虚拟滚动几何 ----------

// 行高钉死为整数像素，虚拟定位的数学计算与 CSS 渲染才能严格一致
const lineH = computed(() => Math.round(fontSize.value * 1.5))

const { width: viewportW, height: viewportH } = useElementSize(scrollEl)
// scrollTop 的响应式镜像
const scrollTopM = ref(0)

// h 挂在 markRaw 对象上，修正后靠 triggerRef(lines) 驱动重算；
// 扫描走裸数组，免掉逐下标的响应式依赖追踪
let lastGeom = { total: 0, start: 0, end: 0, topPad: 0 }
const geometry = computed(() => {
  const n = lines.value.length
  const arr = toRaw(lines.value)
  const lh = lineH.value
  const st = scrollTopM.value
  const vh = viewportH.value
  const pad = OVERSCAN * lh
  let total: number
  let start: number
  let end: number
  let topPad: number
  if (!wrapLines.value) {
    start = Math.min(n, Math.max(0, Math.floor((st - pad) / lh)))
    end = Math.min(n, Math.max(start, Math.ceil((st + vh + pad) / lh)))
    total = n * lh
    topPad = start * lh
  } else {
    const topLimit = st - pad
    const bottomLimit = st + vh + pad
    total = 0
    start = -1
    end = n
    topPad = 0
    for (let i = 0; i < n; i++) {
      const h = arr[i]?.h ?? lh
      if (start < 0 && total + h > topLimit) {
        start = i
        topPad = total
      }
      if (end === n && total >= bottomLimit) end = i
      total += h
    }
    if (start < 0) {
      start = n
      topPad = total
    }
  }
  // 结果等价时复用旧引用，滚动未跨行时不级联重渲染
  const g = lastGeom
  if (g.total === total && g.start === start && g.end === end && g.topPad === topPad) return g
  return (lastGeom = { total, start, end, topPad })
})

const visibleLines = computed(() => {
  const { start, end } = geometry.value
  return lines.value.slice(start, end)
})

// wrap 模式下行高不定：渲染后同步量回真实高度，视口上方的修正量回补 scrollTop 防画面跳动
const measureRendered = () => {
  if (!wrapLines.value) return
  const win = windowEl.value
  if (!win) return
  const vis = visibleLines.value
  // 测量按位置与渲染行一一对应，窗口容器内不得放入行以外的元素
  const children = win.children
  if (children.length !== vis.length) return
  const lh = lineH.value
  const st = scrollTopM.value
  let offset = geometry.value.topPad
  let deltaAbove = 0
  let changed = false
  for (let i = 0; i < vis.length; i++) {
    const line = vis[i]
    if (!line) continue
    if (line.h === undefined) {
      const real = (children[i] as HTMLElement).offsetHeight
      // 全屏切换等瞬间元素不可见时测得 0，保持未测量待下轮
      if (real > 0) {
        line.h = real
        if (real !== lh) {
          changed = true
          // 整行位于视口上沿之上，增减的高度会平移当前画面
          if (offset + lh <= st) deltaAbove += real - lh
        }
      }
    }
    offset += line.h ?? lh
  }
  if (!changed) return
  triggerRef(lines)
  if (deltaAbove !== 0) setScrollTop((el) => el.scrollTop + deltaAbove)
}

watch(visibleLines, measureRendered, { flush: 'post' })

// 非 wrap 时 h 不参与几何，留到切回 wrap 的 watch 触发时再清
const invalidateHeights = () => {
  if (!wrapLines.value) return
  for (const l of toRaw(lines.value)) l.h = undefined
  triggerRef(lines)
}

watch([wrapLines, fontSize, viewportW], invalidateHeights)

// ---------- 滚动控制 ----------

// 程序化滚动的统一入口：期间抑制滚动回调，否则会被误判为用户手动滚动而退出跟随或触发翻页
const setScrollTop = (calc: (el: HTMLElement) => number) => {
  const el = scrollEl.value
  if (!el) return
  suppressScrollHandler = true
  el.scrollTop = calc(el)
  // 同步镜像，不等异步 scroll 事件
  scrollTopM.value = el.scrollTop
  requestAnimationFrame(() => {
    suppressScrollHandler = false
  })
}

const scrollToBottom = () => setScrollTop((el) => el.scrollHeight)

// 跳到已加载内容的开头；抑制滚动回调也顺带避免了落到顶部立刻触发翻页又被位置补偿拽回来
const scrollToTop = () => {
  followMode.value = false
  setScrollTop(() => 0)
}

// 贴底后布局仍会变化（入场动画、wrap 实测修正、tab 转可见），单次 scrollToBottom 必然落空，
// 跟随模式下视口尺寸或内容总高一变就重新贴底
const stickToBottom = () => {
  if (followMode.value) scrollToBottom()
}
watch([viewportW, viewportH], stickToBottom)
watch(() => geometry.value.total, stickToBottom, { flush: 'post' })

const scheduleReconnect = () => {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    startFollow()
  }, 3000)
}

// 繁忙日志下 ws 帧率可达上千每秒，逐帧改 lines 就是逐帧重算渲染；
// 攒到下一帧统一落盘，渲染次数压到 60/秒量级。锚点续读补发积压时这个差距最明显
const flushIncoming = () => {
  flushScheduled = false
  const total = incoming.length
  if (total === 0) return
  // 超出上限的部分永远不会被显示，先裁再解析，省掉白做的 ANSI 转换
  const batch = total > MAX_LINES ? incoming.slice(-MAX_LINES) : incoming
  incoming = []
  lines.value.push(...batch.map(parseLine))
  // 跟随与暂停都要裁剪；裁掉的历史行要同步退还翻页游标，否则下次上翻会跳过这一段
  if (lines.value.length > MAX_LINES) {
    const removed = lines.value.splice(0, lines.value.length - MAX_LINES)
    loadedFromEnd = Math.max(0, loadedFromEnd - removed.length)
    // 暂停查看时头部裁剪令内容整体上移，按裁掉的高度回拉画面保持稳定
    if (!followMode.value) {
      const lh = lineH.value
      const removedH = removed.reduce((s, l) => s + (l.h ?? lh), 0)
      nextTick(() => setScrollTop((el) => el.scrollTop - removedH))
    }
  }
  // 稳态下追加与裁剪行数相抵、内容高度不变，观察器不会触发，这里必须自己贴底
  if (followMode.value) {
    nextTick(scrollToBottom)
  } else {
    pendingNew.value += total
  }
}

// fromAnchor 仅首次连接时为真：从首屏锚点续读补齐空档，
// 重连改用默认的“只跟新增”，否则会把锚点之后已显示的内容整段重放
const startFollow = (fromAnchor = false) => {
  if (!sourceParams.value) return
  const sid = session
  status.value = 'connecting'
  // 上次连接残留的半行与新流拼接会拼出错行
  pendingTail = ''
  ws.follow({ ...sourceParams.value, offset: fromAnchor ? anchorSize : 0 })
    .then((socket) => {
      if (sid !== session) {
        socket.close()
        return
      }
      followWs = socket
      socket.binaryType = 'arraybuffer'
      status.value = 'connected'

      socket.onmessage = (ev) => {
        if (sid !== session) return
        const data: string =
          typeof ev.data === 'string' ? ev.data : decoder.decode(new Uint8Array(ev.data))
        const parts = (pendingTail + data).split('\n')
        pendingTail = parts.pop() ?? ''
        if (parts.length === 0) return
        incoming.push(...parts)
        if (!flushScheduled) {
          flushScheduled = true
          requestAnimationFrame(flushIncoming)
        }
      }

      socket.onclose = () => {
        if (sid !== session) return
        status.value = 'connecting'
        scheduleReconnect()
      }

      socket.onerror = () => {
        if (sid !== session) return
        status.value = 'error'
        socket.close()
      }
    })
    .catch(() => {
      if (sid !== session) return
      status.value = 'error'
      scheduleReconnect()
    })
}

// hasMore 只表达服务端还有没有更早的日志；要不要继续拿是客户端策略，
// 分开后实时裁剪把行数降回上限以下时，向上翻页能自动恢复
const canLoadOlder = computed(() => hasMore.value && lines.value.length < MAX_LINES)

const PAGE_SIZE = 100

const buildTailParams = (initial: boolean) => {
  const base = { ...sourceParams.value, limit: PAGE_SIZE } as Record<string, unknown>
  if (props.service) {
    if (!initial) base.cursor = nextCursor
  } else {
    base.offset = initial ? 0 : loadedFromEnd
    // 带上锚点，翻页始终相对首屏那一刻的文件末尾，不受期间写入的新日志影响
    if (!initial && anchorSize > 0) base.size = anchorSize
  }
  return base as any
}

const loadInitial = () => {
  if (!sourceParams.value) return
  const sid = session
  initialLoading.value = true
  useRequest(file.tail(buildTailParams(true)))
    .onSuccess(({ data }: any) => {
      if (sid !== session) return
      const newLines: string[] = data?.lines ?? []
      lines.value = newLines.map(parseLine)
      loadedFromEnd = newLines.length
      anchorSize = data?.size ?? 0
      nextCursor = data?.next_cursor ?? ''
      hasMore.value = data?.has_more ?? false
      // 与日志行同一次渲染中撤下加载占位，否则 nextTick 时 DOM 里还没有行，贴底会落空
      initialLoading.value = false
      nextTick(() => {
        if (sid !== session) return
        scrollToBottom()
        followMode.value = true
        startFollow(true)
      })
    })
    .onComplete(() => {
      if (sid !== session) return
      initialLoading.value = false
    })
}

const loadOlder = () => {
  if (!sourceParams.value || isLoadingMore.value || !canLoadOlder.value) return
  if (props.service && !nextCursor) {
    hasMore.value = false
    return
  }
  const sid = session
  isLoadingMore.value = true
  loadedOlder.value = true

  useRequest(file.tail(buildTailParams(false)))
    .onSuccess(({ data }: any) => {
      if (sid !== session) return
      const newOldLines: string[] = data?.lines ?? []
      if (newOldLines.length === 0) {
        hasMore.value = false
        return
      }
      const parsed = newOldLines.map(parseLine)
      lines.value.unshift(...parsed)
      loadedFromEnd += parsed.length
      nextCursor = data?.next_cursor ?? ''
      hasMore.value = data?.has_more ?? false
      // 头部插入按新行占位高度回推 scrollTop，保持画面不动
      const addedH = parsed.length * lineH.value
      nextTick(() => setScrollTop((el) => el.scrollTop + addedH))
    })
    .onComplete(() => {
      if (sid !== session) return
      isLoadingMore.value = false
    })
}

const onScroll = () => {
  const el = scrollEl.value
  if (!el) return
  // 镜像无条件更新，抑制期间窗口计算也依赖它
  const st = el.scrollTop
  scrollTopM.value = st
  if (suppressScrollHandler) return
  // 用内存镜像判定，避免读 scrollHeight 强制同步布局
  followMode.value = geometry.value.total - st - viewportH.value < 30
  if (st < 60 && canLoadOlder.value && !isLoadingMore.value) {
    loadOlder()
  }
}

const resumeFollow = () => {
  scrollToBottom()
  followMode.value = true
}

const toggleFollow = () => {
  if (followMode.value) {
    followMode.value = false
  } else {
    resumeFollow()
  }
}

const toggleWrap = () => {
  wrapLines.value = !wrapLines.value
}

const copyAll = () => {
  if (lines.value.length === 0) return
  copy2clipboard(lines.value.map((l) => l.text).join('\n')).then(() => {
    window.$message.success($gettext('Copied successfully'))
  })
}

const increaseFont = () => {
  if (fontSize.value < 24) fontSize.value++
}

const decreaseFont = () => {
  if (fontSize.value > 10) fontSize.value--
}

// 用不区分大小写的正则匹配，免去为每行常驻一份小写副本
const matches = computed(() => {
  const kw = searchKeyword.value
  if (!kw) return []
  const re = new RegExp(escapeRegExp(kw), 'i')
  return lines.value.filter((l) => re.test(l.text))
})

const matchPos = computed(() => matches.value.findIndex((m) => m.id === matchedLineId.value))

// 虚拟滚动下目标行未必存在于 DOM，落点由行高数据直接算出
const scrollLineIntoCenter = (idx: number) => {
  const arr = toRaw(lines.value)
  const lh = lineH.value
  let top = 0
  for (let i = 0; i < idx; i++) top += arr[i]?.h ?? lh
  const h = arr[idx]?.h ?? lh
  followMode.value = false
  setScrollTop(() => top - (viewportH.value - h) / 2)
}

// step 为 1 下一个、-1 上一个，游标基于行 id，翻页/裁剪导致的下标漂移不影响定位
const goToMatch = (step: 1 | -1) => {
  if (!searchKeyword.value) return
  const ms = matches.value
  if (ms.length === 0) {
    matchedLineId.value = null
    window.$message.warning($gettext('Not found'))
    return
  }
  const cur = matchPos.value
  const next = cur < 0 ? (step === 1 ? 0 : ms.length - 1) : (cur + step + ms.length) % ms.length
  const target = ms[next]
  if (!target) return
  matchedLineId.value = target.id
  const idx = lines.value.findIndex((l) => l.id === target.id)
  if (idx < 0) return
  scrollLineIntoCenter(idx)
  // wrap 下目标附近行渲染实测后再校正一次；nextTick 早于下一次 rAF 裁剪，idx 不漂移
  if (wrapLines.value) nextTick(() => scrollLineIntoCenter(idx))
}

watch(searchKeyword, () => {
  matchedLineId.value = null
})

// 恢复跟随即视为已读到最新，积压计数统一在此清零
watch(followMode, (on) => {
  if (on) pendingNew.value = 0
})

const cleanup = () => {
  session++
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  followWs?.close()
  followWs = null
  lines.value = []
  loadedFromEnd = 0
  anchorSize = 0
  nextCursor = ''
  pendingTail = ''
  incoming = []
  hasMore.value = false
  loadedOlder.value = false
  isLoadingMore.value = false
  status.value = 'connecting'
  matchedLineId.value = null
  pendingNew.value = 0
  scrollTopM.value = 0
}

watch(
  () => [props.path, props.service, props.container],
  () => {
    cleanup()
    nextTick(() => loadInitial())
  },
)

onMounted(() => {
  loadInitial()
  // webfont 就绪后字符度量变化，wrap 实测行高需重测
  document.fonts.ready.then(invalidateHeights)
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
  <div v-if="supported" ref="shellEl" class="log-shell">
    <header class="log-titlebar">
      <div class="log-title">
        <span class="status-dot" :class="status" :title="statusText"></span>
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
        <n-popover trigger="click" placement="bottom-end" :to="popoverTo">
          <template #trigger>
            <button class="action-btn" :title="$gettext('Search')">
              <i-mdi-magnify class="text-base" />
            </button>
          </template>
          <n-flex vertical size="small" class="w-70">
            <n-input-group>
              <n-input
                v-model:value="searchKeyword"
                size="small"
                :placeholder="$gettext('Enter keyword and press Enter')"
                @keyup.enter="goToMatch(1)"
              />
              <n-button size="small" :title="$gettext('Previous')" @click="goToMatch(-1)">
                <i-mdi-chevron-up />
              </n-button>
              <n-button size="small" :title="$gettext('Next')" @click="goToMatch(1)">
                <i-mdi-chevron-down />
              </n-button>
            </n-input-group>
            <n-flex justify="space-between" align="center" class="text-xs opacity-60">
              <span>{{ $gettext('Only loaded logs are searched') }}</span>
              <span v-if="searchKeyword">{{ matchPos + 1 }}/{{ matches.length }}</span>
            </n-flex>
          </n-flex>
        </n-popover>
        <div class="action-divider"></div>
        <button
          class="action-btn"
          :class="{ active: wrapLines }"
          :title="wrapLines ? $gettext('Disable Line Wrap') : $gettext('Enable Line Wrap')"
          @click="toggleWrap"
        >
          <i-mdi-wrap class="text-base" />
        </button>
        <button class="action-btn" :title="$gettext('Copy All')" @click="copyAll">
          <i-mdi-content-copy class="text-base" />
        </button>
        <div class="action-divider"></div>
        <button class="action-btn" :title="$gettext('Decrease Font Size')" @click="decreaseFont">
          <i-mdi-format-font-size-decrease class="text-base" />
        </button>
        <button class="action-btn" :title="$gettext('Increase Font Size')" @click="increaseFont">
          <i-mdi-format-font-size-increase class="text-base" />
        </button>
        <div class="action-divider"></div>
        <button
          class="action-btn"
          :title="isFullscreen ? $gettext('Exit Fullscreen') : $gettext('Fullscreen')"
          @click="toggleFullscreen"
        >
          <i-mdi-fullscreen-exit v-if="isFullscreen" class="text-base" />
          <i-mdi-fullscreen v-else class="text-base" />
        </button>
      </div>
      <transition name="pill">
        <div v-if="isLoadingMore || (loadedOlder && !hasMore)" class="log-topbar">
          <n-spin v-if="isLoadingMore" :size="12" />
          <span v-else>{{ $gettext('No more logs') }}</span>
        </div>
      </transition>
    </header>

    <div
      ref="scrollEl"
      class="log-content"
      :class="{ wrap: wrapLines }"
      :style="{ fontSize: `${fontSize}px`, '--log-lh': `${lineH}px` }"
      @scroll="onScroll"
    >
      <div v-if="initialLoading" class="log-loading"><n-spin :size="18" /></div>
      <div v-else-if="lines.length === 0" class="log-boundary">
        {{ $gettext('No logs available') }}
      </div>
      <!-- spacer 以总高撑出滚动条，window 仅承载可视窗口内的行，靠 translateY 落位 -->
      <div v-else :style="{ height: `${geometry.total}px` }">
        <div ref="windowEl" :style="{ transform: `translateY(${geometry.topPad}px)` }">
          <div
            v-for="line in visibleLines"
            :key="line.id"
            class="log-line"
            :class="{ matched: line.id === matchedLineId }"
            v-html="renderLine(line)"
          ></div>
        </div>
      </div>
    </div>

    <transition name="pill">
      <button v-if="pendingNew > 0" class="new-logs-pill" @click="resumeFollow">
        <i-mdi-arrow-down class="text-sm" />
        {{
          $ngettext('%{ count } new line', '%{ count } new lines', pendingNew, {
            count: String(pendingNew),
          })
        }}
      </button>
    </transition>
  </div>
  <n-empty v-else :description="$gettext('No logs available')" />
</template>

<style lang="scss">
/* anser 默认 use_classes 输出的 ANSI 颜色类名，需要全局可达 */
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
  .ansi-bright-black-bg {
    background-color: #686868;
  }
  .ansi-bright-red-bg {
    background-color: #ff6e67;
  }
  .ansi-bright-green-bg {
    background-color: #5ffa68;
  }
  .ansi-bright-yellow-bg {
    background-color: #fffc67;
  }
  .ansi-bright-blue-bg {
    background-color: #6871ff;
  }
  .ansi-bright-magenta-bg {
    background-color: #ff77ff;
  }
  .ansi-bright-cyan-bg {
    background-color: #60fdff;
  }
  .ansi-bright-white-bg {
    background-color: #ffffff;
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

  // 搜索命中的关键词，由 v-html 注入，须置于全局样式
  .log-mark {
    background: #facc15;
    color: #1f1f1f;
    border-radius: 2px;
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

  &:fullscreen {
    height: 100vh;
    min-height: 0;
    border: none;
    border-radius: 0;
  }
}

.log-titlebar {
  // 翻页提示胶囊的定位上下文
  position: relative;
  display: flex;
  flex-wrap: wrap;
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
  // 放不下标题和按钮时让按钮换行，而不是把标题挤没
  min-width: 120px;
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

  &.connected {
    background: #22c55e;
    box-shadow: 0 0 6px rgba(34, 197, 94, 0.5);
  }

  &.connecting {
    background: #9ca3af;
    animation: status-dot-pulse 1.4s ease-in-out infinite;
  }

  &.error {
    background: #ef4444;
  }
}

@keyframes status-dot-pulse {
  0%,
  100% {
    opacity: 0.4;
  }
  50% {
    opacity: 1;
  }
}

.titlebar-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  // 窄屏时标题独占一行，按钮行靠右再按需折行
  max-width: 100%;
  margin-left: auto;
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

// 手机上去掉分隔线、收窄按钮，让整排按钮尽量留在一行
@media (max-width: 767.98px) {
  .action-btn {
    width: 28px;
  }

  .action-divider {
    display: none;
  }
}

.log-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: auto;
  // 虚拟滚动自行维护锚定，浏览器原生锚定只会双重补偿
  overflow-anchor: none;
  font-family: 'JetBrains Mono Variable', monospace;
  font-variant-numeric: tabular-nums;
  line-height: var(--log-lh);
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
  // 行高钉死，数学定位与实际渲染严格一致，空行也正常占位
  height: var(--log-lh);
  user-select: text;
  cursor: text;

  &:hover {
    background: rgba(255, 255, 255, 0.03);
  }

  // 搜索命中行，需在 hover 之后声明以覆盖其背景
  &.matched,
  &.matched:hover {
    background: rgba(250, 204, 21, 0.22);
    box-shadow: inset 3px 0 0 #facc15;
  }
}

.log-content.wrap .log-line {
  white-space: pre-wrap;
  word-break: break-all;
  height: auto;
  min-height: var(--log-lh);
}

.log-loading {
  display: flex;
  justify-content: center;
  padding: 24px 0;
}

.log-boundary {
  display: flex;
  justify-content: center;
  padding: 6px 0;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.35);
  user-select: none;
}

// 悬浮胶囊共用外观
.log-topbar,
.new-logs-pill {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 12px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 999px;
  background: rgba(30, 30, 30, 0.9);
  font-size: 12px;
  z-index: 2;
}

.log-topbar {
  top: calc(100% + 7px);
  color: rgba(255, 255, 255, 0.55);
  user-select: none;
}

.new-logs-pill {
  bottom: 14px;
  color: rgb(74, 222, 128);
  cursor: pointer;
  transition: background 150ms ease;

  &:hover {
    background: rgba(50, 50, 50, 0.95);
  }
}

.pill-enter-active,
.pill-leave-active {
  transition:
    opacity 150ms ease,
    transform 150ms ease;
}

.pill-enter-from,
.pill-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(6px);
}
</style>

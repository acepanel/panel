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
import copy2clipboard from '@vavt/copy2clipboard'
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

const LOCAL_SERVER_ID = -1

// 标签页接口
interface TerminalTab {
  id: string
  hostId: number
  name: string
  terminal: Terminal | null
  fitAddon: FitAddon | null
  webglAddon: WebglAddon | null
  ws: WebSocket | null
  element: HTMLElement | null
  connected: boolean
  latency: number
  pingTimer: ReturnType<typeof setInterval> | null
  lastPingTime: number
}

// 状态
const terminalContainer = ref<HTMLElement | null>(null)
const collapsed = ref(true)
const create = ref(false)
const update = ref(false)
const updateId = ref(0)
const isFullscreen = ref(false)
const showSettings = ref(false)

// 字体设置
const fontSize = ref(14)

// 主机列表
const hostList = ref<any[]>([])

// 标签页
const tabs = ref<TerminalTab[]>([])
const activeTabId = ref<string>('')

// 本机选项
const localServerOption = {
  label: $gettext('Local'),
  key: LOCAL_SERVER_ID
}

// 生成唯一ID
const generateTabId = () => `tab-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

// 获取主机名称
const getHostName = (hostId: number) => {
  if (hostId === LOCAL_SERVER_ID) return $gettext('Local')
  const host = hostList.value.find((h) => h.key === hostId)
  return host?.label || `Host ${hostId}`
}

// 获取主机列表
const fetchData = async () => {
  hostList.value = [localServerOption]
  const data = await ssh.list(1, 10000)
  data.items.forEach((item: any) => {
    hostList.value.push({
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
                  default: () => $gettext('Edit')
                }
              ),
              h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleDeleteHost(item.id)
                },
                {
                  default: () => $gettext('Are you sure you want to delete this host?'),
                  trigger: () =>
                    h(
                      NButton,
                      {
                        size: 'small',
                        type: 'error'
                      },
                      {
                        default: () => $gettext('Delete')
                      }
                    )
                }
              )
            ]
          }
        )
      }
    })
  })

  // 默认打开本机标签
  if (tabs.value.length === 0) {
    await addTab(LOCAL_SERVER_ID)
  }
}

// 删除主机
const handleDeleteHost = (id: number) => {
  useRequest(ssh.delete(id)).onSuccess(() => {
    hostList.value = hostList.value.filter((item: any) => item.key !== id)
  })
}

// 从侧边栏选择主机
const handleSelectHost = (key: number) => {
  addTab(key)
}

// 添加新标签
const addTab = async (hostId: number) => {
  const tabId = generateTabId()
  const tab: TerminalTab = {
    id: tabId,
    hostId,
    name: getHostName(hostId),
    terminal: null,
    fitAddon: null,
    webglAddon: null,
    ws: null,
    element: null,
    connected: false,
    latency: 0,
    pingTimer: null,
    lastPingTime: 0
  }
  tabs.value.push(tab)
  activeTabId.value = tabId

  await nextTick()
  await initTerminal(tabId)
}

// 关闭标签
const closeTab = (tabId: string) => {
  const index = tabs.value.findIndex((t) => t.id === tabId)
  if (index === -1) return

  const tab = tabs.value[index]
  if (tab) disposeTab(tab)
  tabs.value.splice(index, 1)

  // 如果关闭的是当前标签，切换到其他标签
  if (activeTabId.value === tabId && tabs.value.length > 0) {
    const newIndex = Math.min(index, tabs.value.length - 1)
    const newTab = tabs.value[newIndex]
    if (newTab) switchTab(newTab.id)
  }

  // 如果没有标签了，创建一个本机标签
  if (tabs.value.length === 0) {
    addTab(LOCAL_SERVER_ID)
  }
}

// 切换标签
const switchTab = async (tabId: string) => {
  activeTabId.value = tabId
  await nextTick()

  const tab = tabs.value.find((t) => t.id === tabId)
  if (tab?.terminal && tab.fitAddon) {
    tab.fitAddon.fit()
    tab.terminal.focus()
  }
}

// 初始化终端
const initTerminal = async (tabId: string) => {
  const tab = tabs.value.find((t) => t.id === tabId)
  if (!tab) return

  const container = document.getElementById(`terminal-${tabId}`)
  if (!container) return

  tab.element = container

  try {
    // 根据ID选择连接方式
    tab.ws = tab.hostId === LOCAL_SERVER_ID ? await ws.pty('bash') : await ws.ssh(tab.hostId)
    tab.ws.binaryType = 'arraybuffer'

    tab.terminal = new Terminal({
      allowProposedApi: true,
      lineHeight: 1.2,
      fontSize: fontSize.value,
      fontFamily: `'JetBrains Mono Variable', monospace`,
      cursorBlink: true,
      cursorStyle: 'underline',
      tabStopWidth: 4,
      theme: { background: '#111', foreground: '#fff' }
    })

    tab.fitAddon = new FitAddon()
    tab.webglAddon = new WebglAddon()

    tab.terminal.loadAddon(tab.fitAddon)
    tab.terminal.loadAddon(new ClipboardAddon())
    tab.terminal.loadAddon(new WebLinksAddon())
    tab.terminal.loadAddon(new Unicode11Addon())
    tab.terminal.unicode.activeVersion = '11'
    tab.terminal.loadAddon(tab.webglAddon)
    tab.webglAddon.onContextLoss(() => {
      tab.webglAddon?.dispose()
    })

    // 选中自动复制
    tab.terminal.onSelectionChange(() => {
      const selection = tab.terminal?.getSelection()
      if (selection) {
        copy2clipboard(selection)
      }
    })

    tab.terminal.open(container)

    tab.ws.onmessage = (ev) => {
      const data: ArrayBuffer | string = ev.data
      // 检查是否是 pong 响应
      if (typeof data === 'string') {
        try {
          const json = JSON.parse(data)
          if (json.pong) {
            handlePong(tab)
            return
          }
        } catch {
          // 不是 JSON，正常处理
        }
      }
      tab.terminal?.write(typeof data === 'string' ? data : new Uint8Array(data))
    }

    tab.terminal.onData((data) => {
      if (tab.ws?.readyState === WebSocket.OPEN) {
        tab.ws.send(data)
      }
    })

    tab.terminal.onBinary((data) => {
      if (tab.ws?.readyState === WebSocket.OPEN) {
        const buffer = new Uint8Array(data.length)
        for (let i = 0; i < data.length; ++i) {
          buffer[i] = data.charCodeAt(i) & 255
        }
        tab.ws.send(buffer)
      }
    })

    tab.terminal.onResize(({ rows, cols }) => {
      if (tab.ws?.readyState === WebSocket.OPEN) {
        tab.ws.send(
          JSON.stringify({
            resize: true,
            columns: cols,
            rows: rows
          })
        )
      }
    })

    tab.fitAddon.fit()
    tab.terminal.focus()
    tab.connected = true

    // 启动延迟检测
    startPingTimer(tab)

    tab.ws.onclose = () => {
      tab.connected = false
      tab.terminal?.write('\r\n' + $gettext('Connection closed. Please refresh.'))
    }

    tab.ws.onerror = (event) => {
      tab.connected = false
      tab.terminal?.write('\r\n' + $gettext('Connection error. Please refresh.'))
      console.error(event)
      tab.ws?.close()
    }
  } catch (error) {
    console.error('Failed to connect:', error)
    tab.connected = false
  }
}

// 销毁标签
const disposeTab = (tab: TerminalTab) => {
  try {
    if (tab.pingTimer) {
      clearInterval(tab.pingTimer)
      tab.pingTimer = null
    }
    tab.ws?.close()
    tab.terminal?.dispose()
    tab.fitAddon = null
    tab.webglAddon = null
    if (tab.element) {
      tab.element.innerHTML = ''
    }
  } catch {
    /* empty */
  }
}

// 窗口大小变化
const onResize = () => {
  const tab = tabs.value.find((t) => t.id === activeTabId.value)
  if (tab?.fitAddon && tab.terminal) {
    tab.fitAddon.fit()
  }
}

// 滚轮缩放
const onTermWheel = (event: WheelEvent) => {
  if (event.ctrlKey) {
    event.preventDefault()
    if (event.deltaY > 0) {
      if (fontSize.value > 10) {
        fontSize.value--
      }
    } else {
      if (fontSize.value < 32) {
        fontSize.value++
      }
    }
    applyFontSettings()
  }
}

// 右键粘贴
const onContextMenu = async (event: MouseEvent) => {
  event.preventDefault()
  const tab = tabs.value.find((t) => t.id === activeTabId.value)
  if (tab?.terminal && tab.ws?.readyState === WebSocket.OPEN) {
    try {
      const text = await navigator.clipboard.readText()
      if (text) {
        tab.ws.send(text)
      }
    } catch {
      /* clipboard access denied */
    }
  }
}

// 键盘快捷键
const onKeyDown = (event: KeyboardEvent) => {
  const tab = tabs.value.find((t) => t.id === activeTabId.value)
  if (!tab?.terminal) return

  // Ctrl+Shift+C 或 Command+C 复制
  if (
    (event.ctrlKey && event.shiftKey && event.key === 'C') ||
    (event.metaKey && event.key === 'c')
  ) {
    event.preventDefault()
    const selection = tab.terminal.getSelection()
    if (selection) {
      copy2clipboard(selection)
    }
  }

  // Ctrl+Shift+V 或 Command+V 粘贴
  if (
    (event.ctrlKey && event.shiftKey && event.key === 'V') ||
    (event.metaKey && event.key === 'v')
  ) {
    event.preventDefault()
    navigator.clipboard.readText().then((text) => {
      if (text && tab.ws?.readyState === WebSocket.OPEN) {
        tab.ws.send(text)
      }
    })
  }
}

// 应用字体设置
const applyFontSettings = () => {
  tabs.value.forEach((tab) => {
    if (tab.terminal) {
      tab.terminal.options.fontSize = fontSize.value
      tab.fitAddon?.fit()
    }
  })
}

// 启动延迟检测定时器
const startPingTimer = (tab: TerminalTab) => {
  // 立即执行一次
  sendPing(tab)
  // 每3秒检测一次
  tab.pingTimer = setInterval(() => {
    sendPing(tab)
  }, 3000)
}

// 发送 ping
const sendPing = (tab: TerminalTab) => {
  if (tab.ws?.readyState === WebSocket.OPEN) {
    tab.lastPingTime = performance.now()
    tab.ws.send(JSON.stringify({ ping: true }))
  }
}

// 处理 pong 响应
const handlePong = (tab: TerminalTab) => {
  if (tab.lastPingTime > 0) {
    tab.latency = Math.round(performance.now() - tab.lastPingTime)
    tab.lastPingTime = 0
  }
}

// 渲染标签标题
const renderTabLabel = (tab: TerminalTab) => {
  const latencyColor = tab.connected ? '#18a058' : '#d03050'
  const icon = tab.connected ? '✓' : '✗'
  return h('span', { class: 'tab-label' }, [
    h('span', { style: { color: latencyColor, marginRight: '4px' } }, `${tab.latency} ms`),
    h('span', { style: { color: latencyColor, marginRight: '4px' } }, icon),
    tab.name
  ])
}

// 全屏切换
const toggleFullscreen = async () => {
  const container = terminalContainer.value
  if (!container) return

  if (!isFullscreen.value) {
    try {
      await container.requestFullscreen()
      isFullscreen.value = true
    } catch {
      /* fullscreen not supported */
    }
  } else {
    try {
      await document.exitFullscreen()
      isFullscreen.value = false
    } catch {
      /* not in fullscreen */
    }
  }
}

// 监听全屏变化
const onFullscreenChange = () => {
  isFullscreen.value = !!document.fullscreenElement
  nextTick(() => onResize())
}

// 监听字体设置变化
watch(fontSize, () => {
  applyFontSettings()
})

onMounted(() => {
  document.fonts.ready.then((fontFaceSet: any) =>
    Promise.all(Array.from(fontFaceSet).map((el: any) => el.load())).then(fetchData)
  )
  window.$bus.on('ssh:refresh', fetchData)
  window.addEventListener('resize', onResize)
  window.addEventListener('keydown', onKeyDown)
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

onUnmounted(() => {
  tabs.value.forEach(disposeTab)
  window.$bus.off('ssh:refresh')
  window.removeEventListener('resize', onResize)
  window.removeEventListener('keydown', onKeyDown)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<template>
  <common-page show-footer>
    <n-layout has-sider sider-placement="right">
      <n-layout content-style="overflow: visible">
        <div
          ref="terminalContainer"
          class="terminal-container"
          :class="{ fullscreen: isFullscreen }"
        >
          <!-- 工具栏 -->
          <div class="terminal-toolbar">
            <!-- 标签页 -->
            <div class="tabs-wrapper">
              <n-tabs
                v-model:value="activeTabId"
                type="card"
                closable
                size="small"
                @update:value="switchTab"
                @close="closeTab"
              >
                <n-tab-pane
                  v-for="tab in tabs"
                  :key="tab.id"
                  :name="tab.id"
                  :tab="renderTabLabel(tab)"
                  display-directive="show:lazy"
                >
                </n-tab-pane>
              </n-tabs>
            </div>

            <!-- 工具按钮 -->
            <div class="toolbar-actions">
              <n-tooltip trigger="hover">
                <template #trigger>
                  <n-button quaternary size="small" @click="showSettings = !showSettings">
                    <template #icon>
                      <i-mdi-cog />
                    </template>
                  </n-button>
                </template>
                {{ $gettext('Settings') }}
              </n-tooltip>
              <n-tooltip trigger="hover">
                <template #trigger>
                  <n-button quaternary size="small" @click="toggleFullscreen">
                    <template #icon>
                      <i-mdi-fullscreen v-if="!isFullscreen" />
                      <i-mdi-fullscreen-exit v-else />
                    </template>
                  </n-button>
                </template>
                {{ isFullscreen ? $gettext('Exit Fullscreen') : $gettext('Fullscreen') }}
              </n-tooltip>
            </div>
          </div>

          <!-- 设置面板 -->
          <n-collapse-transition :show="showSettings">
            <div class="settings-panel">
              <n-space align="center">
                <span>{{ $gettext('Font Size') }}:</span>
                <n-input-number
                  v-model:value="fontSize"
                  size="small"
                  :min="10"
                  :max="32"
                  style="width: 100px"
                />
              </n-space>
            </div>
          </n-collapse-transition>

          <!-- 终端内容区域 -->
          <div class="terminals-content" @wheel="onTermWheel" @contextmenu="onContextMenu">
            <div
              v-for="tab in tabs"
              :key="tab.id"
              :id="`terminal-${tab.id}`"
              class="terminal-pane"
              :class="{ active: tab.id === activeTabId }"
            ></div>
          </div>
        </div>
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
        <div class="mb-2 text-center">
          <n-button type="primary" @click="create = true">
            {{ $gettext('Create Host') }}
          </n-button>
        </div>
        <n-menu
          :collapsed="collapsed"
          :collapsed-width="0"
          :collapsed-icon-size="0"
          :options="hostList"
          @update-value="handleSelectHost"
        />
      </n-layout-sider>
    </n-layout>
  </common-page>
  <create-modal v-model:show="create" />
  <update-modal v-model:show="update" v-model:id="updateId" />
</template>

<style scoped lang="scss">
.terminal-container {
  display: flex;
  flex-direction: column;
  height: 75vh;

  &.fullscreen {
    height: 100vh;
  }
}

.terminal-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-right: 8px;
}

.tabs-wrapper {
  flex: 1;
  overflow: hidden;

  :deep(.n-tabs .n-tab-pane) {
    padding: 0;
  }
}

.toolbar-actions {
  display: flex;
  gap: 4px;
}

.settings-panel {
  padding: 12px 16px;
}

.terminals-content {
  flex: 1;
  position: relative;
  overflow: hidden;
}

.terminal-pane {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: none;

  &.active {
    display: block;
  }
}

:deep(.xterm) {
  padding: 8px !important;
  height: 100%;
}
</style>

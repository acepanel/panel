<script setup lang="ts">
defineOptions({
  name: 'ssh-index',
})

import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import ssh from '@/api/panel/ssh'
import ws from '@/api/ws'
import { createXterm } from '@/components/common/composables/useXterm'
import type { Xterm } from '@/components/common/composables/useXterm'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useResponsive } from '@/components/system/composables/useResponsive'
import HostModal from '@/views/ssh/HostModal.vue'
import SftpPanel from '@/views/ssh/SftpPanel.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const { isMobile } = useResponsive()

const LOCAL_SERVER_ID = -1

interface TerminalTab {
  id: string
  type: 'terminal' | 'sftp'
  hostId: number
  name: string
  xterm: Xterm | null
  ws: WebSocket | null
  connected: boolean
  latency: number
  pingTimer: ReturnType<typeof setInterval> | null
  lastPingTime: number
}

const terminalContainer = ref<HTMLElement | null>(null)
const showHosts = ref(false)
const hostModal = ref(false)
const editHostId = ref(0)
const isFullscreen = ref(false)
const fontSize = ref(14)

const hostList = ref<any[]>([])

const tabs = ref<TerminalTab[]>([])
const activeTabId = ref<string>('')

const readClipboardText = async (): Promise<string> => {
  if (window.isSecureContext && navigator.clipboard?.readText) {
    return navigator.clipboard.readText()
  }
  window.$message.warning(
    $gettext('Clipboard is unavailable in non-HTTPS context, please use Ctrl+V to paste'),
  )
  return ''
}

interface HostItem {
  key: number
  label: string
  host?: string
  port?: number
  user?: string
}

const localHost: HostItem = {
  key: LOCAL_SERVER_ID,
  label: $gettext('Local'),
}

const generateTabId = () => `tab-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

const getHostName = (hostId: number) => {
  if (hostId === LOCAL_SERVER_ID) return $gettext('Local')
  const host = hostList.value.find((h) => h.key === hostId)
  return host?.label || `Host ${hostId}`
}

// 主机下拉选项(用于顶部 + 按钮)
const hostDropdownOptions = computed(() => [
  ...hostList.value.map((h) => ({ label: h.label, key: h.key })),
  { type: 'divider', key: 'divider-sftp' },
  { label: $gettext('File Transfer'), key: 'sftp' },
])

// 文件互传面板的主机选项(含本机)
const sftpHosts = computed(() => hostList.value.map((h) => ({ label: h.label, value: h.key })))

const fetchData = async () => {
  hostList.value = [localHost]
  const data = await ssh.list(1, 10000)
  data.items.forEach((item: any) => {
    hostList.value.push({
      key: item.id,
      label: item.name === '' ? item.host : item.name,
      host: item.host,
      port: item.port,
      user: item.config.user,
    })
  })

  if (tabs.value.length === 0) {
    await addTab(LOCAL_SERVER_ID)
  }
}

const handleCreateHost = () => {
  editHostId.value = 0
  hostModal.value = true
}

const handleEditHost = (id: number) => {
  editHostId.value = id
  hostModal.value = true
}

const handleConfirmDeleteHost = async (id: number) => {
  const ok = await confirmDelete({
    content: $gettext('Are you sure you want to delete this host?'),
  })
  if (ok) await handleDeleteHost(id)
}

const handleDeleteHost = async (id: number) => {
  await ssh.delete(id)
  hostList.value = hostList.value.filter((item: any) => item.key !== id)
}

const handleSelectHost = (key: number | string) => {
  if (key === 'sftp') {
    addSftpTab()
  } else {
    addTab(key as number)
  }
  showHosts.value = false
}

const addTab = async (hostId: number) => {
  const tabId = generateTabId()
  const tab: TerminalTab = {
    id: tabId,
    type: 'terminal',
    hostId,
    name: getHostName(hostId),
    xterm: null,
    ws: null,
    connected: false,
    latency: 0,
    pingTimer: null,
    lastPingTime: 0,
  }
  tabs.value.push(tab)
  activeTabId.value = tabId

  await nextTick()
  await initTerminal(tabId)
}

const addSftpTab = () => {
  const tabId = generateTabId()
  tabs.value.push({
    id: tabId,
    type: 'sftp',
    hostId: LOCAL_SERVER_ID,
    name: $gettext('File Transfer'),
    xterm: null,
    ws: null,
    connected: false,
    latency: 0,
    pingTimer: null,
    lastPingTime: 0,
  })
  activeTabId.value = tabId
}

const closeTab = (tabId: string) => {
  const index = tabs.value.findIndex((t) => t.id === tabId)
  if (index === -1) return

  const tab = tabs.value[index]
  if (tab) disposeTab(tab)
  tabs.value.splice(index, 1)

  if (activeTabId.value === tabId && tabs.value.length > 0) {
    const newIndex = Math.min(index, tabs.value.length - 1)
    const newTab = tabs.value[newIndex]
    if (newTab) switchTab(newTab.id)
  }

  if (tabs.value.length === 0) {
    addTab(LOCAL_SERVER_ID)
  }
}

const switchTab = async (tabId: string) => {
  activeTabId.value = tabId
  await nextTick()

  const xterm = tabs.value.find((t) => t.id === tabId)?.xterm
  xterm?.fit()
  xterm?.term.focus()
}

const initTerminal = async (tabId: string) => {
  const tab = tabs.value.find((t) => t.id === tabId)
  if (!tab) return

  const container = document.getElementById(`terminal-${tabId}`)
  if (!container) return

  try {
    tab.ws = tab.hostId === LOCAL_SERVER_ID ? await ws.pty('bash') : await ws.ssh(tab.hostId)
    tab.xterm = createXterm(container, tab.ws, {
      fontSize,
      closedText: $gettext('Connection closed. Please refresh.'),
      errorText: $gettext('Connection error. Please refresh.'),
      onMessage: (data) => {
        try {
          if (JSON.parse(data).pong) {
            handlePong(tab)
            return true
          }
        } catch {
          /* 不是 JSON 就是终端输出 */
        }
        return false
      },
      onClose: () => {
        tab.connected = false
      },
    })

    const { term } = tab.xterm
    term.onSelectionChange(() => {
      const selection = term.getSelection()
      if (selection) copy2clipboard(selection)
    })

    tab.connected = true

    startPingTimer(tab)
  } catch (error) {
    console.error('Failed to connect:', error)
    tab.connected = false
  }
}

const disposeTab = (tab: TerminalTab) => {
  if (tab.pingTimer) {
    clearInterval(tab.pingTimer)
    tab.pingTimer = null
  }
  tab.xterm?.dispose()
}

const onContextMenu = async (event: MouseEvent) => {
  const tab = tabs.value.find((t) => t.id === activeTabId.value)
  if (tab?.type !== 'terminal') return
  event.preventDefault()
  if (tab.ws?.readyState === WebSocket.OPEN) {
    try {
      const text = await readClipboardText()
      if (text) tab.ws.send(text)
    } catch {
      /* clipboard access denied */
    }
  }
}

const onKeyDown = (event: KeyboardEvent) => {
  const tab = tabs.value.find((t) => t.id === activeTabId.value)
  if (tab?.type !== 'terminal' || !tab.xterm) return

  if (
    (event.ctrlKey && event.shiftKey && event.key === 'C') ||
    (event.metaKey && event.key === 'c')
  ) {
    event.preventDefault()
    const selection = tab.xterm.term.getSelection()
    if (selection) copy2clipboard(selection)
  }

  if (
    (event.ctrlKey && event.shiftKey && event.key === 'V') ||
    (event.metaKey && event.key === 'v')
  ) {
    event.preventDefault()
    readClipboardText().then((text) => {
      if (text && tab.ws?.readyState === WebSocket.OPEN) tab.ws.send(text)
    })
  }
}

const startPingTimer = (tab: TerminalTab) => {
  sendPing(tab)
  tab.pingTimer = setInterval(() => sendPing(tab), 3000)
}

const sendPing = (tab: TerminalTab) => {
  if (tab.ws?.readyState === WebSocket.OPEN) {
    tab.lastPingTime = performance.now()
    tab.ws.send(JSON.stringify({ ping: true }))
  }
}

const handlePong = (tab: TerminalTab) => {
  if (tab.lastPingTime > 0) {
    tab.latency = Math.round(performance.now() - tab.lastPingTime)
    tab.lastPingTime = 0
  }
}

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

const onFullscreenChange = () => {
  isFullscreen.value = !!document.fullscreenElement
}

onMounted(() => {
  document.fonts.ready.then((fontFaceSet: any) =>
    Promise.all(Array.from(fontFaceSet).map((el: any) => el.load())).then(fetchData),
  )
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

useBus('ssh:refresh', fetchData)

onUnmounted(() => {
  tabs.value.forEach(disposeTab)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<template>
  <PageContainer bare class="terminal-page">
    <div ref="terminalContainer" class="terminal-shell" :class="{ fullscreen: isFullscreen }">
      <header class="terminal-topbar">
        <!-- 信号灯 -->
        <div class="window-dots">
          <span class="dot dot-red"></span>
          <span class="dot dot-yellow"></span>
          <span class="dot dot-green"></span>
        </div>

        <!-- Tab strip -->
        <div class="tab-strip">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            class="tab-item"
            :class="{ active: tab.id === activeTabId }"
            :title="tab.name"
            @click="switchTab(tab.id)"
          >
            <span v-if="tab.type === 'sftp'" class="tab-sftp-icon">
              <i-mdi-swap-horizontal class="text-xs" />
            </span>
            <span v-else class="status-dot" :class="tab.connected ? 'ok' : 'err'"></span>
            <span class="tab-name">{{ tab.name }}</span>
            <span v-if="tab.connected" class="tab-latency">{{ tab.latency }}ms</span>
            <span class="tab-close" @click.stop="closeTab(tab.id)" :title="$gettext('Close')">
              <i-mdi-close class="text-xs" />
            </span>
          </button>
          <n-dropdown trigger="click" :options="hostDropdownOptions" @select="handleSelectHost">
            <button class="tab-add" :title="$gettext('New Session')">
              <i-mdi-plus class="text-lg" />
            </button>
          </n-dropdown>
        </div>

        <!-- 操作 -->
        <div class="topbar-actions">
          <n-popover trigger="click" placement="bottom-end">
            <template #trigger>
              <button class="action-btn" :title="$gettext('Settings')">
                <i-mdi-cog class="text-lg" />
              </button>
            </template>
            <div class="settings-popover">
              <n-flex vertical size="small">
                <span class="settings-label">{{ $gettext('Font Size') }}</span>
                <n-input-number
                  v-model:value="fontSize"
                  size="small"
                  :min="10"
                  :max="32"
                  class="w-30"
                />
              </n-flex>
            </div>
          </n-popover>
          <button class="action-btn" :title="$gettext('Manage Hosts')" @click="showHosts = true">
            <i-mdi-server class="text-lg" />
          </button>
          <button
            class="action-btn"
            :title="isFullscreen ? $gettext('Exit Fullscreen') : $gettext('Fullscreen')"
            @click="toggleFullscreen"
          >
            <i-mdi-fullscreen v-if="!isFullscreen" class="text-lg" />
            <i-mdi-fullscreen-exit v-else class="text-lg" />
          </button>
        </div>
      </header>

      <main class="terminals-content" @keydown="onKeyDown" @contextmenu="onContextMenu">
        <template v-for="tab in tabs" :key="tab.id">
          <div
            v-if="tab.type === 'terminal'"
            :id="`terminal-${tab.id}`"
            class="terminal-pane"
            :class="{ active: tab.id === activeTabId }"
          ></div>
          <div v-else class="terminal-pane sftp-pane" :class="{ active: tab.id === activeTabId }">
            <sftp-panel :hosts="sftpHosts" />
          </div>
        </template>
      </main>
    </div>
  </PageContainer>

  <n-drawer v-model:show="showHosts" :width="isMobile ? '100%' : 360" placement="right">
    <n-drawer-content :title="$gettext('Hosts')" closable>
      <n-flex vertical>
        <n-button type="primary" block @click="handleCreateHost">
          <template #icon>
            <i-mdi-plus />
          </template>
          {{ $gettext('Create Host') }}
        </n-button>
        <div class="host-list">
          <div
            v-for="host in hostList"
            :key="host.key"
            class="host-card"
            @click="handleSelectHost(host.key)"
          >
            <div class="host-card__icon">
              <i-mdi-laptop v-if="host.key === LOCAL_SERVER_ID" />
              <i-mdi-server-network v-else />
            </div>
            <div class="host-card__body">
              <div class="host-card__name">{{ host.label }}</div>
              <div v-if="host.host" class="host-card__meta">
                {{ host.user }}@{{ host.host }}:{{ host.port }}
              </div>
              <div v-else class="host-card__meta">
                {{ $gettext('Bash session on this server') }}
              </div>
            </div>
            <div v-if="host.key !== LOCAL_SERVER_ID" class="host-card__actions">
              <n-button
                quaternary
                circle
                size="tiny"
                :title="$gettext('Edit')"
                @click.stop="handleEditHost(host.key)"
              >
                <template #icon>
                  <i-mdi-pencil />
                </template>
              </n-button>
              <app-button
                quaternary
                circle
                size="tiny"
                type="error"
                :title="$gettext('Delete')"
                @click.stop="handleConfirmDeleteHost(host.key)"
              >
                <template #icon>
                  <i-mdi-delete-outline />
                </template>
              </app-button>
            </div>
          </div>
        </div>
      </n-flex>
    </n-drawer-content>
  </n-drawer>

  <host-modal v-model:show="hostModal" :edit-id="editHostId" />
</template>

<style scoped lang="scss">
.terminal-page {
  height: 100%;
}

.terminal-shell {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 140px);
  min-height: 480px;
  background: var(--color-bg-terminal);
  border: 1px solid var(--color-border-default);
  border-radius: 3px;
  overflow: hidden;
  box-shadow: var(--shadow-md);

  &.fullscreen {
    height: 100vh;
    min-height: 0;
    border: none;
    border-radius: 0;
    box-shadow: none;
  }
}

.terminal-topbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;
}

.window-dots {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;

  &.dot-red {
    background: #ff5f57;
  }

  &.dot-yellow {
    background: #febc2e;
  }

  &.dot-green {
    background: #28c840;
  }
}

.tab-strip {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.15) transparent;

  &::-webkit-scrollbar {
    height: 4px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.15);
    border-radius: 2px;
  }
}

.tab-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 30px;
  padding: 0 8px 0 12px;
  border-radius: 3px;
  background: transparent;
  border: 1px solid transparent;
  color: rgba(255, 255, 255, 0.55);
  font-size: 13px;
  font-family: inherit;
  cursor: pointer;
  flex-shrink: 0;
  transition: all 150ms ease;

  &:hover {
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.85);
  }

  &.active {
    background: rgba(255, 255, 255, 0.08);
    color: #fff;
    border-color: rgba(255, 255, 255, 0.08);
  }
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

.tab-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
  font-variant-numeric: tabular-nums;
}

.tab-sftp-icon {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  color: #3b82f6;
}

.sftp-pane {
  background: var(--color-bg-body);

  &.active {
    display: flex;
    flex-direction: column;
  }
}

.tab-latency {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.4);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.tab-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 3px;
  opacity: 0;
  transition: all 150ms ease;
  color: rgba(255, 255, 255, 0.7);

  .tab-item:hover & {
    opacity: 0.7;
  }

  &:hover {
    opacity: 1 !important;
    background: rgba(255, 255, 255, 0.15);
  }

  // 触屏没有悬停，常驻显示
  @media (hover: none) {
    opacity: 0.7;
  }
}

.tab-add {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 3px;
  background: transparent;
  border: none;
  color: rgba(255, 255, 255, 0.5);
  cursor: pointer;
  flex-shrink: 0;
  transition: all 150ms ease;

  &:hover {
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.9);
  }
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
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
}

.settings-popover {
  min-width: 180px;
  padding: 4px 0;
}

.settings-label {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.terminals-content {
  flex: 1;
  position: relative;
  overflow: hidden;
}

.terminal-pane {
  position: absolute;
  inset: 0;
  display: none;

  &.active {
    display: block;
  }
}

:deep(.xterm) {
  padding: 12px !important;
  height: 100%;
}

.host-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 4px;
}

.host-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border-default);
  background: var(--color-bg-elevated);
  cursor: pointer;
  transition: all 150ms ease;

  &:hover {
    border-color: var(--color-brand);
    background: var(--color-brand-subtle);

    .host-card__actions {
      opacity: 1;
    }
  }

  // 删除确认与请求期间鼠标已离开卡片，转圈的按钮不能随悬停一起隐藏
  &:has(.n-button--loading) .host-card__actions {
    opacity: 1;
  }
}

.host-card__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  background: var(--color-bg-subtle);
  color: var(--color-brand);
  font-size: 16px;
  flex-shrink: 0;
}

.host-card__body {
  flex: 1;
  min-width: 0;
}

.host-card__name {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.host-card__actions {
  display: flex;
  align-items: center;
  gap: 2px;
  opacity: 0;
  transition: opacity 150ms ease;
  flex-shrink: 0;

  @media (hover: none) {
    opacity: 1;
  }
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

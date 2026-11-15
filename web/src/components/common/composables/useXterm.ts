import '@fontsource-variable/jetbrains-mono/wght-italic.css'
import '@fontsource-variable/jetbrains-mono/wght.css'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import { FitAddon } from '@xterm/addon-fit'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { WebglAddon } from '@xterm/addon-webgl'
import { Terminal } from '@xterm/xterm'
import type { ITheme } from '@xterm/xterm'

import '@xterm/xterm/css/xterm.css'
import { $gettext } from '@/utils/gettext'

interface XtermOptions {
  // 传 ref 时多个终端共用字号，任一处缩放全部跟随
  fontSize?: MaybeRef<number>
  theme?: ITheme
  convertEol?: boolean
  closedText?: string
  errorText?: string
  // 文本帧先交给它，返回 true 表示已消费，不再写入终端
  onMessage?: (data: string) => boolean
  onClose?: () => void
  onError?: () => void
}

export type Xterm = ReturnType<typeof createXterm>

const MIN_FONT_SIZE = 10
const MAX_FONT_SIZE = 32

/**
 * 在 el 里创建终端并接到已连接的 socket 上，el 尺寸变化时自动适配行列
 */
export function createXterm(el: HTMLElement, socket: WebSocket, options: XtermOptions = {}) {
  const fontSize = toRef(options.fontSize ?? 14)
  let disposed = false

  const term = new Terminal({
    allowProposedApi: true,
    lineHeight: 1.2,
    fontSize: fontSize.value,
    fontFamily: `'JetBrains Mono Variable', monospace`,
    cursorBlink: true,
    cursorStyle: 'underline',
    tabStopWidth: 4,
    convertEol: options.convertEol ?? false,
    scrollOnEraseInDisplay: true,
    theme: options.theme ?? {
      background:
        getComputedStyle(document.documentElement).getPropertyValue('--color-bg-terminal').trim() ||
        '#0a0e1a',
      foreground: '#e6edf3',
    },
  })

  const fitAddon = new FitAddon()
  const webglAddon = new WebglAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new ClipboardAddon())
  term.loadAddon(new WebLinksAddon())
  term.loadAddon(new Unicode11Addon())
  term.unicode.activeVersion = '11'
  term.loadAddon(webglAddon)
  webglAddon.onContextLoss(() => webglAddon.dispose())
  term.open(el)

  const fit = () => fitAddon.fit()

  // 关闭 socket 后仍可能收到残留帧和 close 事件，此时终端已释放
  const write = (data: string | Uint8Array) => {
    if (!disposed) term.write(data)
  }
  const send = (data: string | Uint8Array<ArrayBuffer>) => {
    if (socket.readyState === WebSocket.OPEN) socket.send(data)
  }

  // 终端输出是 Binary 帧，Text 帧可能是控制类回包
  socket.binaryType = 'arraybuffer'
  socket.onmessage = ({ data }: MessageEvent<string | ArrayBuffer>) => {
    if (typeof data !== 'string') {
      write(new Uint8Array(data))
    } else if (!options.onMessage?.(data)) {
      write(data)
    }
  }
  socket.onclose = () => {
    write('\r\n' + (options.closedText ?? $gettext('Connection closed.')))
    options.onClose?.()
  }
  socket.onerror = (event) => {
    console.error(event)
    write('\r\n' + (options.errorText ?? $gettext('Connection error.')))
    socket.close()
    options.onError?.()
  }

  term.onData(send)
  term.onBinary((data) => send(Uint8Array.from(data, (char) => char.charCodeAt(0) & 255)))
  term.onResize(({ rows, cols }) => send(JSON.stringify({ resize: true, columns: cols, rows })))

  const stopFontWatch = watch(
    fontSize,
    (size) => {
      term.options.fontSize = size
      fit()
    },
    { flush: 'sync' },
  )
  const onWheel = (event: WheelEvent) => {
    if (!event.ctrlKey) return
    event.preventDefault()
    const size = fontSize.value + (event.deltaY > 0 ? -1 : 1)
    fontSize.value = Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, size))
  }
  el.addEventListener('wheel', onWheel, { passive: false })

  const observer = new ResizeObserver(fit)
  observer.observe(el)

  fit()
  term.focus()

  const dispose = () => {
    disposed = true
    stopFontWatch()
    observer.disconnect()
    el.removeEventListener('wheel', onWheel)
    socket.close()
    try {
      term.dispose()
    } catch {
      // xterm 释放失败不能打断调用方的收尾
    }
  }

  // 句柄常被放进响应式对象，xterm 实例不能被深层代理
  return markRaw({ term, fit, dispose })
}

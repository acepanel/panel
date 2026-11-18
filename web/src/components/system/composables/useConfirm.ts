import type { DialogReactive } from 'naive-ui'
import type { VNodeChild } from 'vue'
import { useGettext } from 'vue3-gettext'

interface DeleteOptions {
  title?: string
  content: string
  countdown?: number
  positiveText?: string
  negativeText?: string
  onConfirm?: () => unknown
}

interface ActionOptions {
  type?: 'info' | 'warning' | 'success' | 'error'
  title: string
  content: string | (() => VNodeChild)
  positiveText?: string
  negativeText?: string
  onConfirm?: () => unknown
}

/**
 * 命令式确认 API
 * - confirmDelete: 危险删除(带 5 秒倒计时,沿用旧 DeleteConfirm 行为)
 * - confirmAction: 普通确认
 * - onConfirm: 没有触发按钮可以转圈时(右键菜单、快捷键),把确认后的请求放这里,确认按钮转圈到结束
 */
export function useConfirm() {
  const { $gettext } = useGettext()

  // 请求期间不能取消或关闭；失败时弹窗留着，恢复原状供重试或取消
  const runConfirm = async (dialog: DialogReactive | undefined, onConfirm?: () => unknown) => {
    if (!dialog || !onConfirm) return
    const { closable, closeOnEsc, negativeButtonProps } = dialog
    Object.assign(dialog, {
      loading: true,
      closable: false,
      closeOnEsc: false,
      negativeButtonProps: { ...negativeButtonProps, disabled: true },
    })
    try {
      await onConfirm()
    } finally {
      Object.assign(dialog, { loading: false, closable, closeOnEsc, negativeButtonProps })
    }
  }

  const confirmDelete = (opts: DeleteOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      const total = opts.countdown ?? 0
      let remain = total
      let timer: ReturnType<typeof setInterval> | null = null
      const positiveBase = opts.positiveText ?? $gettext('Delete')
      const dialog = window.$dialog?.warning({
        title: opts.title ?? $gettext('Confirm Deletion'),
        content: opts.content,
        positiveText: total > 0 ? `${positiveBase} (${remain}s)` : positiveBase,
        negativeText: opts.negativeText ?? $gettext('Cancel'),
        positiveButtonProps: { type: 'error', disabled: total > 0 },
        autoFocus: false,
        maskClosable: false,
        onPositiveClick: async () => {
          if (remain > 0) return false
          stop()
          await runConfirm(dialog, opts.onConfirm)
          resolve(true)
        },
        onNegativeClick: () => {
          stop()
          resolve(false)
        },
        onClose: () => {
          stop()
          resolve(false)
        },
        // 按 Esc 关闭不经过上面的回调，这里兜底，否则 Promise 永远不结束
        onAfterLeave: () => {
          stop()
          resolve(false)
        },
      })
      const stop = () => {
        if (timer) {
          clearInterval(timer)
          timer = null
        }
      }
      if (total > 0 && dialog) {
        timer = setInterval(() => {
          remain -= 1
          dialog.positiveText = `${positiveBase}${remain > 0 ? ` (${remain}s)` : ''}`
          dialog.positiveButtonProps = { type: 'error', disabled: remain > 0 }
          if (remain <= 0) stop()
        }, 1000)
      }
    })
  }

  const confirmAction = (opts: ActionOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      const fn = window.$dialog?.[opts.type ?? 'warning']
      if (!fn) {
        resolve(false)
        return
      }
      const dialog = fn({
        title: opts.title,
        content: opts.content,
        positiveText: opts.positiveText ?? $gettext('Confirm'),
        negativeText: opts.negativeText ?? $gettext('Cancel'),
        autoFocus: false,
        maskClosable: false,
        onPositiveClick: async () => {
          await runConfirm(dialog, opts.onConfirm)
          resolve(true)
        },
        onNegativeClick: () => resolve(false),
        onClose: () => resolve(false),
        onAfterLeave: () => resolve(false),
      })
    })
  }

  return { confirmDelete, confirmAction }
}

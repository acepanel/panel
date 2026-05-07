import { useGettext } from 'vue3-gettext'

interface DeleteOptions {
  title?: string
  content: string
  countdown?: number
  positiveText?: string
  negativeText?: string
}

interface ActionOptions {
  type?: 'info' | 'warning' | 'success' | 'error'
  title: string
  content: string
  positiveText?: string
  negativeText?: string
}

/**
 * 命令式确认 API
 * - confirmDelete: 危险删除(带 5 秒倒计时,沿用旧 DeleteConfirm 行为)
 * - confirmAction: 普通确认
 */
export function useConfirm() {
  const { $gettext } = useGettext()

  const confirmDelete = (opts: DeleteOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      const total = opts.countdown ?? 5
      let remain = total
      let timer: ReturnType<typeof setInterval> | null = null
      const dialog = window.$dialog?.warning({
        title: opts.title ?? $gettext('Confirm Deletion'),
        content: opts.content,
        positiveText: `${opts.positiveText ?? $gettext('Delete')} (${remain}s)`,
        negativeText: opts.negativeText ?? $gettext('Cancel'),
        positiveButtonProps: { type: 'error', disabled: true },
        autoFocus: false,
        maskClosable: false,
        onPositiveClick: () => {
          if (remain > 0) return false
          stop()
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
      })
      const stop = () => {
        if (timer) {
          clearInterval(timer)
          timer = null
        }
      }
      timer = setInterval(() => {
        remain -= 1
        if (!dialog) return
        dialog.positiveText = `${opts.positiveText ?? $gettext('Delete')}${remain > 0 ? ` (${remain}s)` : ''}`
        dialog.positiveButtonProps = { type: 'error', disabled: remain > 0 }
        if (remain <= 0) stop()
      }, 1000)
    })
  }

  const confirmAction = (opts: ActionOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      const fn = window.$dialog?.[opts.type ?? 'warning']
      if (!fn) {
        resolve(false)
        return
      }
      fn({
        title: opts.title,
        content: opts.content,
        positiveText: opts.positiveText ?? $gettext('Confirm'),
        negativeText: opts.negativeText ?? $gettext('Cancel'),
        autoFocus: false,
        maskClosable: false,
        onPositiveClick: () => resolve(true),
        onNegativeClick: () => resolve(false),
        onClose: () => resolve(false),
      })
    })
  }

  return { confirmDelete, confirmAction }
}

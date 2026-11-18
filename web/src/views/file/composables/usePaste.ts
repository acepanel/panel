import { NButton, NFlex, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useEditorStore, useFileStore } from '@/stores'
import { joinPath } from '@/utils/file'

// 剪贴板全局共享，工具栏按钮、快捷键、右键菜单任一入口粘贴进行中，其它入口都不再触发
const pasting = ref(false)

export function usePaste() {
  const { $gettext } = useGettext()
  const fileStore = useFileStore()
  const editorStore = useEditorStore()

  const handlePaste = async (targetPath: string) => {
    if (pasting.value) return
    const { marked, markedType } = fileStore.clipboard
    if (!marked.length) {
      window.$message.error($gettext('Please mark the files/folders to copy or move first'))
      return
    }

    const paths = marked.map((item) => ({
      name: item.name,
      source: item.source,
      target: joinPath(targetPath, item.name),
      force: false,
    }))

    const executePaste = async () => {
      const request = markedType === 'copy' ? file.copy(paths) : file.move(paths)
      const successMsg =
        markedType === 'copy' ? $gettext('Copied successfully') : $gettext('Moved successfully')
      await request
      // 移动成功后同步编辑器中已打开的标签页路径
      if (markedType === 'move') {
        for (const item of paths) {
          editorStore.movePath(item.source, item.target)
        }
      }
      fileStore.clearClipboard()
      window.$bus.emit('file:refresh')
      window.$message.success(successMsg)
    }

    // 弹出重命名输入对话框
    const promptRename = (name: string, source: string): Promise<string | null> => {
      return new Promise((resolve) => {
        let newName = name
        const d = window.$dialog.info({
          title: $gettext('Rename'),
          content: () =>
            h('div', [
              h('p', { style: 'margin-bottom: 8px; word-break: break-all' }, source),
              h(NInput, {
                defaultValue: name,
                onUpdateValue: (v: string) => {
                  newName = v
                },
                autofocus: true,
                placeholder: $gettext('Please enter a new name'),
                onKeydown: (e: KeyboardEvent) => {
                  if (e.key === 'Enter') {
                    const trimmed = newName.trim()
                    if (!trimmed) {
                      window.$message.error($gettext('Name cannot be empty'))
                      return
                    }
                    d.destroy()
                    resolve(trimmed)
                  }
                },
              }),
            ]),
          positiveText: $gettext('Confirm'),
          negativeText: $gettext('Cancel'),
          onPositiveClick: () => {
            const trimmed = newName.trim()
            if (!trimmed) {
              window.$message.error($gettext('Name cannot be empty'))
              return false
            }
            resolve(trimmed)
          },
          onNegativeClick: () => resolve(null),
          onClose: () => resolve(null),
          onMaskClick: () => resolve(null),
          // Esc 关闭不经过 onClose，兜底结束等待，否则粘贴按钮一直转圈
          onAfterLeave: () => resolve(null),
        })
      })
    }

    // 处理重命名所有冲突项
    const handleRename = async () => {
      // 失败时冲突对话框留着供重试，各项要还原，否则再点覆盖会按改过的名字粘贴
      const original = paths.map((item) => ({ ...item }))
      try {
        let conflictItems = paths.filter((item) => item.force)
        while (conflictItems.length > 0) {
          for (const item of conflictItems) {
            const newName = await promptRename(item.name, item.source)
            if (newName === null) {
              window.$message.info($gettext('Canceled'))
              return
            }
            item.target = joinPath(targetPath, newName)
            item.name = newName
            item.force = false
          }
          // 重命名后直接检查冲突，避免重新弹出警告对话框导致无限循环
          const targets = paths.map((item) => item.target)
          const data: boolean[] = await file.exist(targets)
          for (let i = 0; i < data.length; i++) {
            const pathItem = paths[i]
            if (pathItem) pathItem.force = !!data[i]
          }
          conflictItems = paths.filter((item) => item.force)
        }
        await executePaste()
      } catch (error) {
        paths.forEach((item, i) => Object.assign(item, original[i]))
        throw error
      }
    }

    // 检查冲突并执行
    const checkAndExecute = async () => {
      const targets = paths.map((item) => item.target)
      const data: boolean[] = await file.exist(targets)
      let hasConflict = false
      for (let i = 0; i < data.length; i++) {
        if (data[i]) {
          hasConflict = true
          const pathItem = paths[i]
          if (pathItem) pathItem.force = true
        } else {
          const pathItem = paths[i]
          if (pathItem) pathItem.force = false
        }
      }

      if (hasConflict) {
        const conflictNames = paths
          .filter((item) => item.force)
          .map((item) => item.name)
          .join(', ')

        // 对话框关掉才结束等待：选了重命名或覆盖后对话框留到操作结束，失败时留着供重试或取消
        await new Promise<void>((resolve) => {
          const running = ref<'rename' | 'overwrite'>()
          const d = window.$dialog.warning({
            title: $gettext('Warning'),
            content: $gettext(
              'There are items with the same name %{ items } Do you want to overwrite?',
              { items: conflictNames },
            ),
            onAfterLeave: () => resolve(),
            action: () =>
              h(NFlex, { justify: 'end' }, () => [
                h(
                  NButton,
                  {
                    disabled: !!running.value,
                    onClick: () => {
                      d.destroy()
                      window.$message.info($gettext('Canceled'))
                    },
                  },
                  () => $gettext('Cancel'),
                ),
                h(
                  NButton,
                  {
                    type: 'info',
                    loading: running.value === 'rename',
                    disabled: running.value === 'overwrite',
                    onClick: () => run('rename', handleRename),
                  },
                  () => $gettext('Rename'),
                ),
                h(
                  NButton,
                  {
                    type: 'warning',
                    loading: running.value === 'overwrite',
                    disabled: running.value === 'rename',
                    onClick: () => run('overwrite', executePaste),
                  },
                  () => $gettext('Overwrite'),
                ),
              ]),
          })
          // 被点的按钮转圈、其它按钮禁用、不可关闭，结束后恢复原状
          const run = async (key: 'rename' | 'overwrite', action: () => Promise<void>) => {
            const { closable, closeOnEsc, maskClosable } = d
            running.value = key
            Object.assign(d, { closable: false, closeOnEsc: false, maskClosable: false })
            try {
              await action()
            } finally {
              running.value = undefined
              Object.assign(d, { closable, closeOnEsc, maskClosable })
            }
            d.destroy()
          }
        })
      } else {
        await executePaste()
      }
    }

    pasting.value = true
    try {
      await checkAndExecute()
    } finally {
      pasting.value = false
    }
  }

  return { pasting, handlePaste }
}

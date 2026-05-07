import { NButton, NFlex, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useFileStore } from '@/stores'

export function usePaste() {
  const { $gettext } = useGettext()
  const fileStore = useFileStore()

  const handlePaste = (targetPath: string) => {
    const { marked, markedType } = fileStore.clipboard
    if (!marked.length) {
      window.$message.error($gettext('Please mark the files/folders to copy or move first'))
      return
    }

    const paths = marked.map((item) => ({
      name: item.name,
      source: item.source,
      target: targetPath + '/' + item.name,
      force: false,
    }))

    const executePaste = () => {
      const request = markedType === 'copy' ? file.copy(paths) : file.move(paths)
      const successMsg =
        markedType === 'copy' ? $gettext('Copied successfully') : $gettext('Moved successfully')
      useRequest(request).onSuccess(() => {
        fileStore.clearClipboard()
        window.$bus.emit('file:refresh')
        window.$message.success(successMsg)
      })
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
        })
      })
    }

    // 处理重命名所有冲突项
    const handleRename = async () => {
      let conflictItems = paths.filter((item) => item.force)
      while (conflictItems.length > 0) {
        for (const item of conflictItems) {
          const newName = await promptRename(item.name, item.source)
          if (newName === null) {
            window.$message.info($gettext('Canceled'))
            return
          }
          item.target = targetPath + '/' + newName
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
      executePaste()
    }

    // 检查冲突并执行
    const checkAndExecute = () => {
      const targets = paths.map((item) => item.target)
      useRequest(file.exist(targets)).onSuccess(({ data }) => {
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

          const d = window.$dialog.warning({
            title: $gettext('Warning'),
            content: $gettext(
              'There are items with the same name %{ items } Do you want to overwrite?',
              { items: conflictNames },
            ),
            action: () =>
              h(NFlex, { justify: 'end' }, () => [
                h(
                  NButton,
                  {
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
                    onClick: () => {
                      d.destroy()
                      handleRename()
                    },
                  },
                  () => $gettext('Rename'),
                ),
                h(
                  NButton,
                  {
                    type: 'warning',
                    onClick: () => {
                      d.destroy()
                      executePaste()
                    },
                  },
                  () => $gettext('Overwrite'),
                ),
              ]),
          })
        } else {
          executePaste()
        }
      })
    }

    checkAndExecute()
  }

  return { handlePaste }
}

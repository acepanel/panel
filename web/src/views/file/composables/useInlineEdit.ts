import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { checkName, getFilename, joinPath } from '@/utils/file'
import { useFileOps } from '@/views/file/composables/useFileOps'

// 文件列表里的内联新建与内联重命名，path 为列表当前所在目录
export function useInlineEdit(path: Ref<string>) {
  const { $gettext } = useGettext()
  const { movePath } = useFileOps()
  const { confirmAction } = useConfirm()

  // 内联重命名状态
  const inlineRenameItem = ref<any>(null)
  const inlineRenameName = ref('')
  const inlineRenameInputRef = ref<HTMLInputElement | null>(null)

  // 内联新建状态
  const inlineCreateActive = ref(false)
  const inlineCreateIsDir = ref(false)
  const inlineCreateName = ref('')
  const inlineCreateInputRef = ref<HTMLInputElement | null>(null)

  // 回车提交后到请求结束前输入框还在，这期间失焦（包括被覆盖确认框夺走焦点）不能再提交一次
  // 期间输入框只读而不是禁用：禁用会丢掉焦点，取消覆盖或失败后失焦提交就不再生效
  const inlineSubmitting = ref(false)

  // 设置内联重命名输入框 ref 的函数
  const setInlineRenameRef = (el: any) => {
    if (el) {
      inlineRenameInputRef.value = el
    }
  }

  // 设置内联新建输入框 ref 的函数
  const setInlineCreateRef = (el: any) => {
    if (el) {
      inlineCreateInputRef.value = el
    }
  }

  // 启动内联新建
  const startInlineCreate = (isDir: boolean) => {
    // 取消正在进行的重命名
    cancelInlineRename()

    inlineCreateActive.value = true
    inlineCreateIsDir.value = isDir
    inlineCreateName.value = ''

    // 等待 DOM 更新后聚焦输入框
    nextTick(() => {
      if (inlineCreateInputRef.value) {
        inlineCreateInputRef.value.focus()
      }
    })
  }

  // 取消内联新建
  const cancelInlineCreate = () => {
    inlineCreateActive.value = false
    inlineCreateIsDir.value = false
    inlineCreateName.value = ''
  }

  // 提交内联新建
  const submitInlineCreate = () => {
    if (!inlineCreateActive.value || inlineSubmitting.value) return

    const name = inlineCreateName.value.trim()

    // 如果名称为空，直接取消
    if (!name) {
      cancelInlineCreate()
      return
    }

    // 验证名称
    if (!checkName(name)) {
      window.$message.error($gettext('Invalid name'))
      return
    }

    const fullPath = joinPath(path.value, name)

    inlineSubmitting.value = true
    useRequest(file.create(fullPath, inlineCreateIsDir.value))
      .onSuccess(() => {
        window.$bus.emit('file:refresh')
        window.$message.success($gettext('Created successfully'))
      })
      .onComplete(() => {
        inlineSubmitting.value = false
        cancelInlineCreate()
      })
  }

  // 处理内联新建键盘事件
  const handleInlineCreateKeydown = (event: KeyboardEvent) => {
    if (inlineSubmitting.value) return
    if (event.key === 'Enter') {
      event.preventDefault()
      submitInlineCreate()
    } else if (event.key === 'Escape') {
      event.preventDefault()
      cancelInlineCreate()
    }
  }

  // 启动内联重命名
  const startInlineRename = (item: any) => {
    inlineRenameItem.value = item
    inlineRenameName.value = getFilename(item.name)
    // 等待 DOM 更新后聚焦输入框
    nextTick(() => {
      if (inlineRenameInputRef.value) {
        inlineRenameInputRef.value.focus()
        // 选中文件名（不包括扩展名）
        const dotIndex = inlineRenameName.value.lastIndexOf('.')
        if (dotIndex > 0 && !item.dir) {
          inlineRenameInputRef.value.setSelectionRange(0, dotIndex)
        } else {
          inlineRenameInputRef.value.select()
        }
      }
    })
  }

  // 取消内联重命名
  const cancelInlineRename = () => {
    inlineRenameItem.value = null
    inlineRenameName.value = ''
  }

  // 提交内联重命名
  const submitInlineRename = async () => {
    if (!inlineRenameItem.value || inlineSubmitting.value) return

    const item = inlineRenameItem.value
    const sourceName = getFilename(item.name)
    const targetName = inlineRenameName.value.trim()

    // 如果名称没有变化，直接取消
    if (sourceName === targetName) {
      cancelInlineRename()
      return
    }

    // 验证名称
    if (!checkName(targetName)) {
      window.$message.error($gettext('Invalid name'))
      return
    }

    const source = joinPath(path.value, sourceName)
    const target = joinPath(path.value, targetName)

    // 成功后编辑器标签页同步与列表刷新由 movePath 收敛处理
    const rename = async (force: boolean) => {
      if (await movePath(source, target, force)) {
        window.$message.success(
          $gettext('Renamed %{ source } to %{ target } successfully', {
            source: sourceName,
            target: targetName,
          }),
        )
      }
      cancelInlineRename()
    }

    inlineSubmitting.value = true
    try {
      const [exists] = await file.exist([target])
      // 同名项在覆盖确认框里改名，不覆盖就保持编辑状态，让用户修改名称
      if (exists) {
        await confirmAction({
          title: $gettext('Warning'),
          content: $gettext('There are items with the same name. Do you want to overwrite?'),
          positiveText: $gettext('Overwrite'),
          negativeText: $gettext('Cancel'),
          onConfirm: () => rename(true),
        })
      } else {
        await rename(false)
      }
    } catch {
      // 请求失败的错误弹窗由全局拦截器负责，保持编辑状态
    } finally {
      inlineSubmitting.value = false
    }
  }

  // 处理内联重命名键盘事件
  const handleInlineRenameKeydown = (event: KeyboardEvent) => {
    if (inlineSubmitting.value) return
    if (event.key === 'Enter') {
      event.preventDefault()
      submitInlineRename()
    } else if (event.key === 'Escape') {
      event.preventDefault()
      cancelInlineRename()
    }
  }

  return {
    inlineSubmitting,
    inlineCreateActive,
    inlineCreateIsDir,
    inlineCreateName,
    setInlineCreateRef,
    startInlineCreate,
    submitInlineCreate,
    handleInlineCreateKeydown,
    inlineRenameItem,
    inlineRenameName,
    setInlineRenameRef,
    startInlineRename,
    submitInlineRename,
    handleInlineRenameKeydown,
  }
}

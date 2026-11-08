import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useEditorStore, useFileStore } from '@/stores'
import { lastDirectory } from '@/utils/file'

// 文件操作的统一入口
// 操作成功后的编辑器标签页同步、列表刷新、选中清理在此收敛，调用方只负责确认交互
// 请求失败的错误弹窗由全局拦截器负责
export function useFileOps() {
  const { $gettext } = useGettext()
  const fileStore = useFileStore()
  const editorStore = useEditorStore()

  // 删除多个路径，allSettled 保证部分失败时也刷新列表
  async function deletePaths(paths: string[]): Promise<void> {
    const results = await Promise.allSettled(paths.map((p) => file.delete(p)))
    const deleted = paths.filter((_, i) => results[i]!.status === 'fulfilled')

    // 关闭编辑器中已打开的对应标签页，并从选中中移除
    deleted.forEach((p) => editorStore.closePath(p))
    if (fileStore.activeTab) {
      const deletedSet = new Set(deleted)
      fileStore.activeTab.selected = fileStore.activeTab.selected.filter((p) => !deletedSet.has(p))
    }

    window.$bus.emit('file:refresh')
    if (deleted.length === paths.length) {
      window.$message.success($gettext('Deleted successfully'))
    }
  }

  // 移动/重命名单个路径，成功后同步编辑器标签页并刷新列表
  async function movePath(source: string, target: string, force: boolean): Promise<boolean> {
    try {
      await file.move([{ source, target, force }])
    } catch {
      return false
    }
    editorStore.movePath(source, target)
    window.$bus.emit('file:refresh')
    return true
  }

  // 标记复制/移动到剪贴板，并清空选中
  function markClipboard(paths: string[], type: 'copy' | 'move') {
    fileStore.setClipboard(
      paths.map((p) => ({ name: lastDirectory(p), source: p, force: false })),
      type,
    )
    if (fileStore.activeTab) {
      fileStore.activeTab.selected = []
    }
    window.$message.success(
      $gettext('Marked successfully, please navigate to the destination path to paste'),
    )
  }

  // 收藏/取消收藏目录
  async function toggleFavorite(path: string) {
    const added = await fileStore.toggleFavorite(path)
    window.$message.success(
      added ? $gettext('Added to favorites') : $gettext('Removed from favorites'),
    )
  }

  return { deletePaths, movePath, markClipboard, toggleFavorite }
}

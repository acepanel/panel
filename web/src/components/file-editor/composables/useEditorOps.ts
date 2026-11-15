import file from '@/api/panel/file'
import { useEditorStore } from '@/stores'
import { decodeBase64 } from '@/utils'

// 编辑器文件加载/保存的统一入口
// 组件不再各自拼请求，加载状态、行分隔符规范化、保存标记在此收敛
// 请求失败的错误弹窗由全局拦截器负责，调用方按返回值决定后续提示
export function useEditorOps() {
  const editorStore = useEditorStore()

  // 从磁盘加载文件内容到已存在的标签页
  async function loadTab(path: string, encoding?: string): Promise<boolean> {
    editorStore.setLoading(path, true)
    try {
      const tab = editorStore.tabs.find((t) => t.path === path)
      const data = await file.content(path, encoding ?? tab?.encoding ?? 'auto')
      editorStore.reloadFile(path, decodeBase64(data.content), data.encoding)
      return true
    } catch {
      return false
    } finally {
      editorStore.setLoading(path, false)
    }
  }

  // 打开文件到编辑器：已打开则切换，否则新建标签页并加载，加载失败关闭标签页
  async function openInEditor(path: string): Promise<void> {
    if (editorStore.tabs.some((t) => t.path === path)) {
      editorStore.switchTab(path)
      return
    }
    editorStore.openFile(path, '')
    if (!(await loadTab(path, 'auto'))) {
      editorStore.closeTab(path)
    }
  }

  // 保存标签页（内容按状态栏显示的行分隔符规范化，由后端转成标签页编码）
  async function saveTab(path: string): Promise<boolean> {
    try {
      const tab = editorStore.tabs.find((t) => t.path === path)
      await file.save(path, editorStore.contentForSave(path), tab?.encoding)
      editorStore.markSaved(path)
      return true
    } catch {
      return false
    }
  }

  // 批量保存，返回失败的路径列表
  async function saveTabs(paths: string[]): Promise<string[]> {
    const failed: string[] = []
    for (const path of paths) {
      if (!(await saveTab(path))) {
        failed.push(path)
      }
    }
    return failed
  }

  return { loadTab, openInEditor, saveTab, saveTabs }
}

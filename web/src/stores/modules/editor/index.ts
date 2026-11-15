import { languageByPath } from '@/utils/file'

// 打开的文件标签页
export interface EditorTab {
  path: string // 文件完整路径
  name: string // 文件名
  content: string // 文件内容
  originalContent: string // 原始内容（用于判断是否修改）
  language: string // 语言类型
  modified: boolean // 是否已修改
  loading: boolean // 是否正在加载
  lineEnding: 'LF' | 'CRLF' // 行分隔符
  encoding: string // 文件编码
  cursorLine: number // 光标行
  cursorColumn: number // 光标列
}

// 编辑器设置
export interface EditorSettings {
  tabSize: number // 缩进大小
  insertSpaces: boolean // 使用空格缩进
  wordWrap: 'on' | 'off' | 'wordWrapColumn' | 'bounded' // 自动换行
  fontSize: number // 字体大小
  minimap: boolean // 是否显示小地图
  // 高级设置
  lineNumbers: 'on' | 'off' | 'relative' | 'interval' // 行号显示
  renderWhitespace: 'none' | 'boundary' | 'selection' | 'trailing' | 'all' // 空白字符显示
  cursorBlinking: 'blink' | 'smooth' | 'phase' | 'expand' | 'solid' // 光标闪烁
  cursorStyle: 'line' | 'block' | 'underline' | 'line-thin' | 'block-outline' | 'underline-thin' // 光标样式
  smoothScrolling: boolean // 平滑滚动
  mouseWheelZoom: boolean // 鼠标滚轮缩放
  bracketPairColorization: boolean // 括号配对着色
  guides: boolean // 缩进参考线
  folding: boolean // 代码折叠
  formatOnPaste: boolean // 粘贴时格式化
  formatOnType: boolean // 输入时格式化
}

export interface EditorState {
  tabs: EditorTab[] // 打开的标签页
  activeTabPath: string | null // 当前激活的标签页路径
  settings: EditorSettings // 编辑器设置
  rootPath: string // 文件树根目录
}

const defaultSettings: EditorSettings = {
  tabSize: 4,
  insertSpaces: true,
  wordWrap: 'on',
  fontSize: 14,
  minimap: true,
  lineNumbers: 'on',
  renderWhitespace: 'selection',
  cursorBlinking: 'blink',
  cursorStyle: 'line',
  smoothScrolling: true,
  mouseWheelZoom: true,
  bracketPairColorization: true,
  guides: true,
  folding: true,
  formatOnPaste: false,
  formatOnType: false,
}

export const useEditorStore = defineStore('editor', {
  state: (): EditorState => ({
    tabs: [],
    activeTabPath: null,
    settings: { ...defaultSettings },
    rootPath: '/',
  }),

  getters: {
    // 获取当前激活的标签页
    activeTab(): EditorTab | null {
      if (!this.activeTabPath) return null
      return this.tabs.find((tab) => tab.path === this.activeTabPath) || null
    },

    // 是否有未保存的文件
    hasUnsavedFiles(): boolean {
      return this.tabs.some((tab) => tab.modified)
    },

    // 获取未保存的文件列表
    unsavedTabs(): EditorTab[] {
      return this.tabs.filter((tab) => tab.modified)
    },
  },

  actions: {
    // 打开文件（添加标签页）
    openFile(path: string, content: string = '', encoding: string = 'utf-8') {
      const existingTab = this.tabs.find((tab) => tab.path === path)
      if (existingTab) {
        // 文件已打开，切换到该标签页
        this.activeTabPath = path
        return existingTab
      }

      // 检测行分隔符
      const lineEnding = content.includes('\r\n') ? 'CRLF' : 'LF'

      // 创建新标签页
      const newTab: EditorTab = {
        path,
        name: path.split('/').pop() || path,
        content,
        originalContent: content,
        language: languageByPath(path),
        modified: false,
        loading: false,
        lineEnding,
        encoding,
        cursorLine: 1,
        cursorColumn: 1,
      }

      this.tabs.push(newTab)
      this.activeTabPath = path
      return newTab
    },

    // 关闭标签页
    closeTab(path: string) {
      const index = this.tabs.findIndex((tab) => tab.path === path)
      if (index === -1) return

      this.tabs.splice(index, 1)

      // 如果关闭的是当前激活的标签页，切换到相邻标签页
      if (this.activeTabPath === path) {
        if (this.tabs.length === 0) {
          this.activeTabPath = null
        } else if (index >= this.tabs.length) {
          this.activeTabPath = this.tabs[this.tabs.length - 1]?.path ?? null
        } else {
          this.activeTabPath = this.tabs[index]?.path ?? null
        }
      }
    },

    // 关闭所有标签页
    closeAllTabs() {
      this.tabs = []
      this.activeTabPath = null
    },

    // 关闭其他标签页
    closeOtherTabs(path: string) {
      this.tabs = this.tabs.filter((tab) => tab.path === path)
      this.activeTabPath = path
    },

    // 关闭已保存的标签页
    closeSavedTabs() {
      this.tabs = this.tabs.filter((tab) => tab.modified)
      if (this.tabs.length === 0) {
        this.activeTabPath = null
      } else if (!this.tabs.find((tab) => tab.path === this.activeTabPath)) {
        this.activeTabPath = this.tabs[0]?.path ?? null
      }
    },

    // 切换标签页
    switchTab(path: string) {
      if (this.tabs.find((tab) => tab.path === path)) {
        this.activeTabPath = path
      }
    },

    // 更新文件内容
    updateContent(path: string, content: string) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab) {
        tab.content = content
        // 将内容规范化为 LF 后比较，避免行分隔符差异影响修改状态判断
        const normalizedContent = content.replace(/\r\n/g, '\n')
        const normalizedOriginal = tab.originalContent.replace(/\r\n/g, '\n')
        tab.modified = normalizedContent !== normalizedOriginal
      }
    },

    // 标记文件已保存
    markSaved(path: string) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab) {
        tab.originalContent = tab.content
        tab.modified = false
      }
    },

    // 按当前行分隔符规范化后的待保存内容
    // 兼容粘贴带入的异种换行：统一为状态栏显示的 lineEnding，避免 LF 文件被存成 CRLF
    contentForSave(path: string): string {
      const tab = this.tabs.find((t) => t.path === path)
      if (!tab) return ''
      const lf = tab.content.replace(/\r\n?/g, '\n')
      return tab.lineEnding === 'CRLF' ? lf.replace(/\n/g, '\r\n') : lf
    },

    // 更新光标位置
    updateCursor(path: string, line: number, column: number) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab) {
        tab.cursorLine = line
        tab.cursorColumn = column
      }
    },

    // 更新行分隔符
    updateLineEnding(path: string, lineEnding: 'LF' | 'CRLF') {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab && tab.lineEnding !== lineEnding) {
        tab.lineEnding = lineEnding
      }
    },

    // 更新编码
    updateEncoding(path: string, encoding: string) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab && tab.encoding !== encoding) {
        tab.encoding = encoding
        // 落盘字节会变，需要用户保存后才生效
        tab.modified = true
      }
    },

    // 重命名标签页(文件/目录重命名后同步路径,保留内容与修改状态)
    renameTab(oldPath: string, newPath: string) {
      const tab = this.tabs.find((t) => t.path === oldPath)
      if (!tab) return
      tab.path = newPath
      tab.name = newPath.split('/').pop() || newPath
      tab.language = languageByPath(newPath)
      if (this.activeTabPath === oldPath) {
        this.activeTabPath = newPath
      }
    },

    // 关闭指定路径及其目录下所有已打开的标签页(文件/目录删除后同步)
    closePath(path: string) {
      const stale = this.tabs
        .filter((t) => t.path === path || t.path.startsWith(path + '/'))
        .map((t) => t.path)
      stale.forEach((p) => this.closeTab(p))
    },

    // 迁移指定路径及其目录下的标签页到新路径(重命名/移动后同步)
    movePath(oldPath: string, newPath: string) {
      for (const tab of this.tabs) {
        if (tab.path === oldPath) {
          this.renameTab(oldPath, newPath)
        } else if (tab.path.startsWith(oldPath + '/')) {
          this.renameTab(tab.path, newPath + tab.path.slice(oldPath.length))
        }
      }
    },

    // 更新语言
    updateLanguage(path: string, language: string) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab) {
        tab.language = language
      }
    },

    // 设置加载状态
    setLoading(path: string, loading: boolean) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab) {
        tab.loading = loading
      }
    },

    // 更新编辑器设置
    updateSettings(settings: Partial<EditorSettings>) {
      this.settings = { ...this.settings, ...settings }
    },

    // 设置根目录
    setRootPath(path: string) {
      this.rootPath = path
    },

    // 重新加载文件内容
    reloadFile(path: string, content: string, encoding?: string) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab) {
        tab.content = content
        tab.originalContent = content
        tab.modified = false
        tab.lineEnding = content.includes('\r\n') ? 'CRLF' : 'LF'
        if (encoding) tab.encoding = encoding
      }
    },

    // 重新排序标签页
    reorderTabs(fromIndex: number, toIndex: number) {
      if (fromIndex === toIndex) return
      if (fromIndex < 0 || fromIndex >= this.tabs.length) return
      if (toIndex < 0 || toIndex >= this.tabs.length) return

      const [movedTab] = this.tabs.splice(fromIndex, 1)
      if (movedTab) this.tabs.splice(toIndex, 0, movedTab)
    },
  },

  persist: {
    pick: ['settings', 'rootPath'],
  },
})

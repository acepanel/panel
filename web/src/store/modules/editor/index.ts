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
  minimap: true
}

export const useEditorStore = defineStore('editor', {
  state: (): EditorState => ({
    tabs: [],
    activeTabPath: null,
    settings: { ...defaultSettings },
    rootPath: '/'
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

    // 获取标签页索引
    activeTabIndex(): number {
      if (!this.activeTabPath) return -1
      return this.tabs.findIndex((tab) => tab.path === this.activeTabPath)
    }
  },

  actions: {
    // 打开文件（添加标签页）
    openFile(path: string, content: string = '') {
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
        cursorLine: 1,
        cursorColumn: 1
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
          this.activeTabPath = this.tabs[this.tabs.length - 1].path
        } else {
          this.activeTabPath = this.tabs[index].path
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
        this.activeTabPath = this.tabs[0].path
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
        tab.modified = content !== tab.originalContent
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
        // 判断是否需要标记为修改（如果原始内容的行分隔符与新设置不同）
        const originalHasCRLF = tab.originalContent.includes('\r\n')
        const originalLineEnding = originalHasCRLF ? 'CRLF' : 'LF'
        if (originalLineEnding !== lineEnding && !tab.modified) {
          tab.modified = true
        } else if (originalLineEnding === lineEnding && tab.content === tab.originalContent.replace(/\r\n/g, '\n')) {
          // 如果切换回原始行分隔符且内容未变，取消修改标记
          tab.modified = false
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
    reloadFile(path: string, content: string) {
      const tab = this.tabs.find((t) => t.path === path)
      if (tab) {
        tab.content = content
        tab.originalContent = content
        tab.modified = false
        tab.lineEnding = content.includes('\r\n') ? 'CRLF' : 'LF'
      }
    },

    // 重新排序标签页
    reorderTabs(fromIndex: number, toIndex: number) {
      if (fromIndex === toIndex) return
      if (fromIndex < 0 || fromIndex >= this.tabs.length) return
      if (toIndex < 0 || toIndex >= this.tabs.length) return

      const [movedTab] = this.tabs.splice(fromIndex, 1)
      this.tabs.splice(toIndex, 0, movedTab)
    }
  },

  persist: {
    pick: ['settings', 'rootPath']
  }
})

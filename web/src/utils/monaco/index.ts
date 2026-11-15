import type * as Monaco from 'monaco-editor'

let monacoInstance: typeof Monaco | null = null
let isInitialized = false
let initPromise: Promise<typeof Monaco> | null = null

async function loadMonacoLocale(locale: string) {
  if (locale === 'zh_CN') await import('monaco-editor/nls/lang/zh-cn.js')
  if (locale === 'zh_TW') await import('monaco-editor/nls/lang/zh-tw.js')
}

async function setupMonacoWorkers() {
  if (self.MonacoEnvironment) return

  const [editorWorker, jsonWorker, cssWorker, htmlWorker, tsWorker] = await Promise.all([
    import('monaco-editor/editor/editor.worker.js?worker'),
    import('monaco-editor/language/json/json.worker.js?worker'),
    import('monaco-editor/language/css/css.worker.js?worker'),
    import('monaco-editor/language/html/html.worker.js?worker'),
    import('monaco-editor/language/typescript/ts.worker.js?worker'),
  ])

  self.MonacoEnvironment = {
    getWorker(_: any, label: string) {
      if (label === 'json') {
        return new jsonWorker.default()
      }
      if (label === 'css' || label === 'scss' || label === 'less') {
        return new cssWorker.default()
      }
      if (label === 'html' || label === 'handlebars' || label === 'razor') {
        return new htmlWorker.default()
      }
      if (label === 'typescript' || label === 'javascript') {
        return new tsWorker.default()
      }
      return new editorWorker.default()
    },
  }
}

function setupConfigLanguages(monaco: typeof Monaco) {
  const configuration: Monaco.languages.LanguageConfiguration = {
    comments: { lineComment: '#' },
    brackets: [['{', '}']],
    autoClosingPairs: [
      { open: '{', close: '}' },
      { open: '"', close: '"' },
    ],
    surroundingPairs: [
      { open: '{', close: '}' },
      { open: '"', close: '"' },
    ],
  }

  for (const id of ['nginx', 'apache', 'apacheconf']) {
    monaco.languages.register({ id })
    monaco.languages.setLanguageConfiguration(id, configuration)
  }

  // nginx 指令库体积大，首次用到 nginx 语言时才加载
  // 主题先占位保证加载完成前深浅色正确，monaco-editor-nginx 加载后会重新定义并刷新
  monaco.editor.defineTheme('nginx-theme', { base: 'vs', inherit: true, rules: [], colors: {} })
  monaco.editor.defineTheme('nginx-theme-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [],
    colors: {},
  })
  monaco.languages.onLanguage('nginx', () => void import('monaco-editor-nginx'))
}

/**
 * 获取 Monaco 实例
 * @param locale 可选的语言设置
 * @returns Monaco 实例
 */
export async function getMonaco(locale?: string): Promise<typeof Monaco> {
  if (isInitialized && monacoInstance) {
    return monacoInstance
  }

  if (initPromise) {
    return initPromise
  }

  initPromise = (async () => {
    if (locale) {
      await loadMonacoLocale(locale)
    }

    await setupMonacoWorkers()

    const monaco = await import('monaco-editor')
    setupConfigLanguages(monaco)
    monacoInstance = monaco
    isInitialized = true

    return monaco
  })()

  return initPromise
}

export type { Monaco }

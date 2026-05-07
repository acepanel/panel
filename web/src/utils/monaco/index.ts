import type * as Monaco from 'monaco-editor'

let monacoInstance: typeof Monaco | null = null
let isInitialized = false
let initPromise: Promise<typeof Monaco> | null = null

async function loadMonacoLocale(locale: string) {
  switch (locale) {
    case 'cs':
      await import('monaco-editor/esm/nls.messages.cs.js')
      break
    case 'de':
      await import('monaco-editor/esm/nls.messages.de.js')
      break
    case 'es':
      await import('monaco-editor/esm/nls.messages.es.js')
      break
    case 'fr':
      await import('monaco-editor/esm/nls.messages.fr.js')
      break
    case 'it':
      await import('monaco-editor/esm/nls.messages.it.js')
      break
    case 'ja':
      await import('monaco-editor/esm/nls.messages.ja.js')
      break
    case 'ko':
      await import('monaco-editor/esm/nls.messages.ko.js')
      break
    case 'pl':
      await import('monaco-editor/esm/nls.messages.pl.js')
      break
    case 'pt_BR':
      await import('monaco-editor/esm/nls.messages.pt-br.js')
      break
    case 'ru':
      await import('monaco-editor/esm/nls.messages.ru.js')
      break
    case 'tr':
      await import('monaco-editor/esm/nls.messages.tr.js')
      break
    case 'zh_CN':
      await import('monaco-editor/esm/nls.messages.zh-cn.js')
      break
    case 'zh_TW':
      await import('monaco-editor/esm/nls.messages.zh-tw.js')
      break
    default:
      break
  }
}

async function setupMonacoWorkers() {
  if (self.MonacoEnvironment) return

  const [editorWorker, jsonWorker, cssWorker, htmlWorker, tsWorker] = await Promise.all([
    import('monaco-editor/esm/vs/editor/editor.worker?worker'),
    import('monaco-editor/esm/vs/language/json/json.worker?worker'),
    import('monaco-editor/esm/vs/language/css/css.worker?worker'),
    import('monaco-editor/esm/vs/language/html/html.worker?worker'),
    import('monaco-editor/esm/vs/language/typescript/ts.worker?worker'),
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
    await import('monaco-editor-nginx')
    monacoInstance = monaco
    isInitialized = true

    return monaco
  })()

  return initPromise
}

export type { Monaco }

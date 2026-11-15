// nginx 的 token 配色在 monaco-editor-nginx 注册的专用主题里，其余语言用内置主题
export function getEditorTheme(lang: string, dark: boolean) {
  return (lang === 'nginx' ? 'nginx-theme' : 'vs') + (dark ? '-dark' : '')
}

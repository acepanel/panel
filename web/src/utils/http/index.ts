import type { AlovaXHRResponse } from '@alova/adapter-xhr'
import { xhrRequestAdapter } from '@alova/adapter-xhr'
import { createAlova, Method } from 'alova'
import VueHook from 'alova/vue'

import { $gettext } from '@/utils/gettext'
import { resolveNetworkError, resolveResError } from '@/utils/http/helpers'

export const http = createAlova({
  id: 'panel',
  cacheFor: null,
  statesHook: VueHook,
  requestAdapter: xhrRequestAdapter(),
  baseURL: import.meta.env.VITE_BASE_API,
  // alova 拼查询串时不做 URL 编码，这里先编码好再交给它
  beforeRequest: ({ config }) => {
    const { params } = config
    if (typeof params === 'string') return
    config.params = Object.keys(params)
      .filter((key) => params[key] !== undefined)
      .map((key) => `${encodeURIComponent(key)}=${encodeURIComponent(String(params[key]))}`)
      .join('&')
  },
  responded: {
    onSuccess: async (response: AlovaXHRResponse, method: Method) => {
      const ct = response.headers['Content-Type'] || response.headers['content-type']
      let json
      try {
        if (ct && ct.includes('application/json')) {
          json = typeof response.data === 'string' ? JSON.parse(response.data) : response.data
        } else {
          json = { code: response.status, msg: response.data }
        }
      } catch {
        json = { code: response.status, msg: $gettext('Failed to parse response') }
      }
      const { status, statusText } = response
      const { meta } = method
      if (status !== 200) {
        const code = json?.code ?? status
        const msg = resolveResError(
          code,
          (typeof json?.msg === 'string' && json.msg.trim()) || statusText,
        )
        const noAlert = meta?.noAlert
        if (!noAlert) {
          if (code === 422) {
            window.$message.error(msg)
          } else if (code !== 401 && code !== 418) {
            window.$dialog.error({
              title: $gettext('Error'),
              content: msg,
              maskClosable: false,
            })
          }
        }
        throw new Error(msg)
      }
      return json.data
    },
    // 只有适配器抛出的 Error（断网、超时、中断）会走到这里
    onError: (error: any, method: Method) => {
      const msg = resolveNetworkError(error)
      if (!method.meta?.noAlert) {
        window.$dialog.error({
          title: $gettext('Request failed'),
          content: msg,
          maskClosable: false,
        })
      }

      // 和 onSuccess 一样抛出翻译后的文案，自己在行内展示错误的调用方（上传队列）直接可用
      throw new Error(msg, { cause: error })
    },
  },
})

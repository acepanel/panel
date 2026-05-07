import type { AlovaXHRResponse } from '@alova/adapter-xhr'
import { xhrRequestAdapter } from '@alova/adapter-xhr'
import { createAlova, Method } from 'alova'
import VueHook from 'alova/vue'

import { resolveResError } from '@/utils/http/helpers'

export const http = createAlova({
  id: 'panel',
  cacheFor: null,
  statesHook: VueHook,
  requestAdapter: xhrRequestAdapter(),
  baseURL: import.meta.env.VITE_BASE_API,
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
        json = { code: response.status, msg: 'failed to parse response' }
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
              title: '错误',
              content: msg,
              maskClosable: false,
            })
          }
        }
        throw new Error(msg)
      }
      return json.data
    },
    onError: (error: any, method: Method) => {
      const { code, msg } = error
      const { meta } = method
      const errorMsg = resolveResError(code, msg)
      const noAlert = meta?.noAlert

      if (!noAlert) {
        window.$dialog.error({
          title: '接口请求失败',
          content: errorMsg,
          maskClosable: false,
        })
      }

      throw error
    },
  },
})

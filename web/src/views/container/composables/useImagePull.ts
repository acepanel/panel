import { useGettext } from 'vue3-gettext'

import ws from '@/api/ws'

/**
 * 前台拉取镜像：经 WebSocket 接收分层进度，拉取完成后调用 onSuccess
 */
export function useImagePull(onSuccess: () => void) {
  const { $gettext } = useGettext()

  const isPulling = ref(false)
  const pullProgress = ref(new Map<string, any>())
  const pullStatus = ref('')
  const pullError = ref('')
  let socket: WebSocket | null = null

  const totalProgress = computed(() => {
    const layers = Array.from(pullProgress.value.values())
    const completed = layers.filter(
      (p) => p.status === 'Pull complete' || p.status === 'Already exists',
    ).length
    const total = layers.filter((p) => p.id && p.id.length === 12).length

    return total > 0 ? Math.round((completed / total) * 100) : 0
  })

  // 中止拉取，已有的进度和错误保留在界面上
  const cancel = () => {
    if (socket) {
      socket.onmessage = null
      socket.onclose = null
      socket.onerror = null
      socket.close()
      socket = null
    }
    isPulling.value = false
  }

  const reset = () => {
    cancel()
    pullProgress.value = new Map()
    pullStatus.value = ''
    pullError.value = ''
  }

  const start = (name: string, auth?: { username: string; password: string }) => {
    reset()
    isPulling.value = true
    pullStatus.value = $gettext('Connecting...')

    ws.imagePull(name, auth)
      .then((conn) => {
        socket = conn
        pullStatus.value = $gettext('Pulling image...')

        conn.onmessage = (event) => {
          let data: any
          try {
            data = JSON.parse(event.data)
          } catch {
            return
          }

          if (data.error) {
            pullError.value = data.error
            isPulling.value = false
            return
          }

          if (data.complete) {
            pullStatus.value = $gettext('Pull completed')
            isPulling.value = false
            onSuccess()
            return
          }

          if (data.id) {
            pullProgress.value.set(data.id, data)
          }
          pullStatus.value = data.status
        }

        conn.onclose = () => {
          isPulling.value = false
        }

        conn.onerror = () => {
          pullError.value = $gettext('Connection error')
          isPulling.value = false
        }
      })
      .catch((err) => {
        pullError.value = err.message || $gettext('Failed to connect')
        isPulling.value = false
      })
  }

  onUnmounted(cancel)

  return { isPulling, pullProgress, pullStatus, pullError, totalProgress, start, cancel, reset }
}

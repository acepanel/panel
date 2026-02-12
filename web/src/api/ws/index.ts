const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
const base = `${protocol}://${window.location.host}/api/ws`

export default {
  // 执行命令
  exec: (cmd: string): Promise<WebSocket> => {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(`${base}/exec`)
      ws.onopen = () => {
        ws.send(cmd)
        resolve(ws)
      }
      ws.onerror = (e) => reject(e)
    })
  },
  // 连接SSH
  ssh: (id: number): Promise<WebSocket> => {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(`${base}/ssh?id=${id}`)
      ws.onopen = () => resolve(ws)
      ws.onerror = (e) => reject(e)
    })
  },
  // 迁移进度
  migrationProgress: (): Promise<WebSocket> => {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(`${base}/migration/progress`)
      ws.onopen = () => resolve(ws)
      ws.onerror = (e) => reject(e)
    })
  }
}

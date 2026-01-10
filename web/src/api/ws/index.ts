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
  // 连接容器终端
  container: (id: string): Promise<WebSocket> => {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(`${base}/container/${id}`)
      ws.onopen = () => resolve(ws)
      ws.onerror = (e) => reject(e)
    })
  },
  // 拉取镜像
  imagePull: (name: string, auth?: { username: string; password: string }): Promise<WebSocket> => {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(`${base}/container/image/pull`)
      ws.onopen = () => {
        ws.send(
          JSON.stringify({ name, auth: !!auth, username: auth?.username, password: auth?.password })
        )
        resolve(ws)
      }
      ws.onerror = (e) => reject(e)
    })
  }
}

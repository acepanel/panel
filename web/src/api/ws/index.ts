const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
const base = `${protocol}://${window.location.host}/api/ws`

// 建立连接，传了 firstMessage 则在连接打开后先发送
const connect = (path: string, firstMessage?: string): Promise<WebSocket> =>
  new Promise((resolve, reject) => {
    const ws = new WebSocket(base + path)
    ws.onopen = () => {
      if (firstMessage !== undefined) ws.send(firstMessage)
      resolve(ws)
    }
    ws.onerror = (e) => reject(e)
  })

export default {
  // PTY 命令执行
  pty: (command: string): Promise<WebSocket> => connect('/pty', command),
  // 文件或 systemd 服务实时跟踪，offset 为首屏锚点字节位置
  follow: (params: {
    path?: string
    service?: string
    container?: string
    offset?: number
  }): Promise<WebSocket> => {
    const qs = new URLSearchParams()
    if (params.path) qs.set('path', params.path)
    if (params.service) qs.set('service', params.service)
    if (params.container) qs.set('container', params.container)
    if (params.offset) qs.set('offset', String(params.offset))
    return connect(`/follow?${qs.toString()}`)
  },
  // 连接SSH
  ssh: (id: number): Promise<WebSocket> => connect(`/ssh?id=${id}`),
  // 主机间文件传输(id 为 0 表示面板本机)
  sshTransfer: (params: {
    src_id: number
    src_path: string
    dst_id: number
    dst_path: string
  }): Promise<WebSocket> => connect('/ssh/transfer', JSON.stringify(params)),
  // 连接容器终端
  container: (id: string): Promise<WebSocket> => connect(`/container/${id}`),
  // 拉取镜像
  imagePull: (name: string, auth?: { username: string; password: string }): Promise<WebSocket> =>
    connect(
      '/container/image/pull',
      JSON.stringify({
        name,
        auth: !!auth,
        username: auth?.username,
        password: auth?.password,
      }),
    ),
  // 迁移进度
  migrationProgress: (): Promise<WebSocket> => connect('/migration/progress'),
  // 证书签发进度
  certObtain: (id: number): Promise<WebSocket> => connect('/cert/obtain', JSON.stringify({ id })),
  // 证书续签进度
  certRenew: (id: number): Promise<WebSocket> => connect('/cert/renew', JSON.stringify({ id })),
  // 面板升级进度
  panelUpdate: (): Promise<WebSocket> => connect('/panel/update'),
}

import { http } from '@/utils'

export default {
  // 获取 SSH 信息
  info: (): any => http.Get('/toolbox_ssh/info'),
  // 设置 SSH 端口
  updatePort: (port: number): any => http.Post('/toolbox_ssh/port', { port }),
  // 设置密码认证
  updatePasswordAuth: (enabled: boolean): any =>
    http.Post('/toolbox_ssh/password_auth', { enabled }),
  // 设置密钥认证
  updatePubkeyAuth: (enabled: boolean): any => http.Post('/toolbox_ssh/pubkey_auth', { enabled }),
  // 设置 Root 登录
  updateRootLogin: (mode: string): any => http.Post('/toolbox_ssh/root_login', { mode }),
  // 设置 Root 密码
  updateRootPassword: (password: string): any =>
    http.Post('/toolbox_ssh/root_password', { password }),
  // 获取 Root 公钥
  rootKey: (): any => http.Get('/toolbox_ssh/root_key'),
  // 生成 Root 密钥对
  generateRootKey: (): any => http.Post('/toolbox_ssh/root_key'),
}

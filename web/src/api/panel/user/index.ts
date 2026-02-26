import { http } from '@/utils'

export default {
  // 公钥
  key: (): any => http.Get('/user/key'),
  // 获取验证码
  captcha: (): any => http.Get('/user/captcha'),
  // 登录
  login: (
    username: string,
    password: string,
    pass_code: string,
    safe_login: boolean,
    captcha_code: string
  ): any =>
    http.Post('/user/login', {
      username,
      password,
      pass_code,
      safe_login,
      captcha_code
    }),
  // 登出
  logout: (): any => http.Post('/user/logout'),
  // 是否登录
  isLogin: (): any => http.Get('/user/is_login'),
  // 是否2FA
  isTwoFA: (username: string): any => http.Get('/user/is_2fa', { params: { username } }),
  // 获取用户信息
  info: (): any => http.Get('/user/info'),
  // 获取用户列表
  list: (page: number, limit: number): any => http.Get(`/users`, { params: { page, limit } }),
  // 创建用户
  create: (username: string, password: string, email: string): any =>
    http.Post('/users', { username, password, email }),
  // 删除用户
  delete: (id: number): any => http.Delete(`/users/${id}`),
  // 更新用户用户名
  updateUsername: (id: number, username: string): any =>
    http.Post(`/users/${id}/username`, { username }),
  // 更新用户邮箱
  updateEmail: (id: number, email: string): any => http.Post(`/users/${id}/email`, { email }),
  // 更新用户密码
  updatePassword: (id: number, password: string): any =>
    http.Post(`/users/${id}/password`, { password }),
  // 生成2FA密钥
  generateTwoFA: (id: number): any => http.Get(`/users/${id}/2fa`),
  // 保存2FA密钥
  updateTwoFA: (id: number, code: string, secret: string): any =>
    http.Post(`/users/${id}/2fa`, { code, secret }),

  // 获取用户Token列表
  tokenList: (user_id: number, page: number, limit: number): any =>
    http.Get(`/user_tokens`, { params: { user_id, page, limit } }),
  // 创建用户Token
  tokenCreate: (user_id: number, ips: string[], expired_at: number): any =>
    http.Post('/user_tokens', { user_id, ips, expired_at }),
  // 删除用户Token
  tokenDelete: (id: number): any => http.Delete(`/user_tokens/${id}`),
  // 更新用户Token
  tokenUpdate: (id: number, ips: string[], expired_at: number): any =>
    http.Put(`/user_tokens/${id}`, { ips, expired_at }),
  // 通行密钥
  passkeyEnabled: (): any => http.Get('/user/passkey/enabled'),
  passkeySupported: (): any => http.Get('/user_passkeys/supported'),
  passkeyBeginRegister: (name: string): any => http.Post('/user/passkey/register', { name }),
  passkeyFinishRegister: (credential: any, name: string): any =>
    http.Put(`/user/passkey/register?name=${encodeURIComponent(name)}`, credential),
  passkeyBeginLogin: (): any => http.Post('/user/passkey/login'),
  passkeyFinishLogin: (assertion: any): any => http.Put('/user/passkey/login', assertion),
  passkeyList: (user_id: number): any => http.Get('/user_passkeys', { params: { user_id } }),
  passkeyDelete: (id: number, user_id: number): any =>
    http.Delete(`/user_passkeys/${id}`, { params: { user_id } })
}

import { http } from '@/utils'

export default {
  // 获取设置
  list: (): any => http.Get('/setting'),
  // 保存设置
  update: (settings: any): any => http.Post('/setting', settings),
  // 刷新证书
  obtainCert: (): any => http.Post('/setting/obtain_cert'),
  // 获取便签
  getMemo: (): any => http.Get('/setting/memo'),
  // 保存便签
  updateMemo: (content: string): any => http.Post('/setting/memo', { content }),
}

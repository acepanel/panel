import { http } from '@/utils'

export default {
  // 列表
  list: (type: string, page: number, limit: number): any =>
    http.Get('/website', { params: { type, page, limit } }),
  // 创建
  create: (data: any): any => http.Post('/website', data),
  // 删除
  delete: (id: number, path: boolean, db: boolean): any =>
    http.Delete(`/website/${id}`, { path, db }),
  // 伪静态
  rewrites: (): any => http.Get(`/website/rewrites`),
  // 获取默认配置
  defaultConfig: (): any => http.Get('/website/default_config'),
  // 保存默认配置
  saveDefaultConfig: (data: any): any => http.Post('/website/default_config', data),
  // 网站配置
  config: (id: number): any => http.Get('/website/' + id),
  // 保存网站配置
  saveConfig: (id: number, data: any): any => http.Put(`/website/${id}`, data),
  // 清空日志
  clearLog: (id: number): any => http.Delete('/website/' + id + '/log'),
  // 更新备注
  updateRemark: (id: number, remark: string): any =>
    http.Post(`/website/${id}` + '/update_remark', { remark }),
  // 重置配置
  resetConfig: (id: number): any => http.Post(`/website/${id}/reset_config`),
  // 修改状态
  status: (id: number, status: boolean): any => http.Post(`/website/${id}/status`, { status }),
  // 修改到期时间
  updateExpireAt: (id: number, expire_at: string): any =>
    http.Post(`/website/${id}/expire_at`, { expire_at }),
  // 签发证书
  obtainCert: (id: number, dns_id?: number): any =>
    http.Post(`/website/${id}/obtain_cert`, dns_id ? { dns_id } : {}),
  // 统计概览
  statOverview: (start: string, end: string, sites?: string): any =>
    http.Get('/website/stat/overview', { params: { start, end, sites } }),
  // 实时统计
  statRealtime: (): any => http.Get('/website/stat/realtime'),
  // 网站维度汇总
  statSites: (start: string, end: string, sites?: string): any =>
    http.Get('/website/stat/sites', { params: { start, end, sites } }),
  // 蜘蛛统计
  statSpiders: (start: string, end: string, sites?: string): any =>
    http.Get('/website/stat/spiders', { params: { start, end, sites } }),
  // 客户端统计
  statClients: (start: string, end: string, sites?: string): any =>
    http.Get('/website/stat/clients', { params: { start, end, sites } }),
  // IP 统计
  statIPs: (start: string, end: string, sites?: string, page?: number, limit?: number): any =>
    http.Get('/website/stat/ips', { params: { start, end, sites, page, limit } }),
  // 地理位置统计
  statGeos: (
    start: string,
    end: string,
    sites?: string,
    group_by?: string,
    country?: string,
    limit?: number
  ): any =>
    http.Get('/website/stat/geos', { params: { start, end, sites, group_by, country, limit } }),
  // URI 统计
  statURIs: (start: string, end: string, sites?: string, page?: number, limit?: number): any =>
    http.Get('/website/stat/uris', { params: { start, end, sites, page, limit } }),
  // 慢请求 URI 统计
  statSlowURIs: (
    start: string,
    end: string,
    sites?: string,
    threshold?: number,
    page?: number,
    limit?: number
  ): any =>
    http.Get('/website/stat/slow_uris', { params: { start, end, sites, threshold, page, limit } }),
  // 错误统计
  statErrors: (
    start: string,
    end: string,
    sites?: string,
    status?: number,
    page?: number,
    limit?: number
  ): any => http.Get('/website/stat/errors', { params: { start, end, sites, status, page, limit } }),
  // 清空统计
  statClear: (): any => http.Post('/website/stat/clear'),
  // 统计设置
  statSetting: (): any => http.Get('/website/stat/setting'),
  // 保存统计设置
  saveStatSetting: (data: any): any => http.Post('/website/stat/setting', data)
}

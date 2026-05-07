import { http } from '@/utils'

export default {
  // CA 供应商列表
  caProviders: (): any => http.Get('/cert/ca_providers'),
  // DNS 供应商列表
  dnsProviders: (): any => http.Get('/cert/dns_providers'),
  // 证书算法列表
  algorithms: (): any => http.Get('/cert/algorithms'),
  // ACME 账号列表
  accounts: (page: number, limit: number): any =>
    http.Get('/cert/account', { params: { page, limit } }),
  // ACME 账号详情
  accountInfo: (id: number): any => http.Get(`/cert/account/${id}`),
  // ACME 账号添加
  accountCreate: (data: any): any => http.Post('/cert/account', data),
  // ACME 账号更新
  accountUpdate: (id: number, data: any): any => http.Put(`/cert/account/${id}`, data),
  // ACME 账号删除
  accountDelete: (id: number): any => http.Delete(`/cert/account/${id}`),
  // DNS 记录列表
  dns: (page: number, limit: number): any => http.Get('/cert/dns', { params: { page, limit } }),
  // DNS 记录详情
  dnsInfo: (id: number): any => http.Get(`/cert/dns/${id}`),
  // DNS 记录添加
  dnsCreate: (data: any): any => http.Post('/cert/dns', data),
  // DNS 记录更新
  dnsUpdate: (id: number, data: any): any => http.Put(`/cert/dns/${id}`, data),
  // DNS 记录删除
  dnsDelete: (id: number): any => http.Delete(`/cert/dns/${id}`),
  // 证书列表
  certs: (page: number, limit: number): any => http.Get('/cert/cert', { params: { page, limit } }),
  // 证书详情
  certInfo: (id: number): any => http.Get(`/cert/cert/${id}`),
  // 证书上传
  certUpload: (data: any): any => http.Post('/cert/cert/upload', data),
  // 证书添加
  certCreate: (data: any): any => http.Post('/cert/cert', data),
  // 证书更新
  certUpdate: (id: number, data: any): any => http.Put(`/cert/cert/${id}`, data),
  // 证书删除
  certDelete: (id: number): any => http.Delete(`/cert/cert/${id}`),
  // 证书自动签发
  obtainAuto: (id: number): any => http.Post(`/cert/cert/${id}/obtain_auto`, { id }),
  // 证书自签名签发
  obtainSelfSigned: (id: number): any => http.Post(`/cert/cert/${id}/obtain_self_signed`, { id }),
  // 续签
  renew: (id: number): any => http.Post(`/cert/cert/${id}/renew`, { id }),
  // 部署
  deploy: (id: number, website_id: number, enable_https: boolean = false): any =>
    http.Post(`/cert/cert/${id}/deploy`, { id, website_id, enable_https }),
}

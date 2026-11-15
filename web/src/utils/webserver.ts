// 面板支持的 Web 服务器应用标识，用于安装检测
export const WEBSERVER_SLUGS = 'nginx,openresty,apache,openlitespeed,caddy'

// 站点相关页面按 Web 服务器差异化展示的能力，新增服务器时在此补一行。
// 服务器表达不了的设置项一律不显示，后端也不会写进配置
export interface WebServerFeatures {
  quic: boolean // HTTP/3 监听
  ipv6Listen: boolean // IPv6 监听
  ocsp: boolean // OCSP 装订开关
  tlsVersions: string[] // 可选的 TLS 版本
  resolver: boolean // 上游与代理的 DNS 解析器
  matchType: boolean // 代理匹配类型
  proxySNI: boolean // 代理 SNI
  proxyCache: boolean | 'ttl' // 代理缓存，'ttl' 为只能设一个有效期、没有其余细项
  proxyReplaces: boolean // 代理响应内容替换
  proxyHTTPVersions: string[] // 代理可选的 HTTP 版本，空则不显示
  proxyBodySize: boolean // 代理请求体大小限制
  proxyTimeouts: string[] // 代理可设的超时项：connect、read、send，空则不显示
  proxyRetry: string[] // 代理可设的重试项：conditions、tries、timeout，空则不显示
  proxySSLVerify: boolean // 后端证书校验
  proxySSLVerifyDepth: boolean // 后端证书校验深度
  proxyResponseHeaders: boolean // 代理响应头修改
  proxyAccessControl: boolean // 代理 IP 访问控制
  proxyHeaderVars: boolean // 代理请求头的值里能用变量
  upstreamRequestRewrite: boolean // 代理到上游时能改 Host 与请求头
  upstreamTuning: boolean // 代理到上游时能按规则设超时与缓冲
  redirect404Options: boolean // 404 重定向的状态码与保留 URI
  rewritePresets: boolean // 伪静态预设
  stat: boolean // 访问统计
  defaultSite: boolean // 默认站点
  errorLog: boolean // 站点级错误日志
  rateLimit: boolean // 限速
  rateLimitConn: boolean // 并发连接数限制
  realIP: boolean // 真实 IP
  realIPRecursive: boolean // 真实 IP 递归查找
  lsCache: boolean // LiteSpeed 页面缓存
  upstreamAlgos: string[] // 上游负载均衡算法，取 nginx 写法，空字符串为默认轮询
  upstreamKeepalive: boolean // 上游连接保持数
  upstreamOptions: 'all' | 'weight' | 'none' // 上游服务器的附加参数：全部、只有权重、不支持
  lang: string // 配置文件语法高亮
}

const allTLSVersions = ['TLSv1', 'TLSv1.1', 'TLSv1.2', 'TLSv1.3']

const features: Record<string, WebServerFeatures> = {
  nginx: {
    quic: true,
    ipv6Listen: true,
    ocsp: true,
    tlsVersions: allTLSVersions,
    resolver: true,
    matchType: true,
    proxySNI: true,
    proxyCache: true,
    proxyReplaces: true,
    proxyHTTPVersions: ['1.0', '1.1', '2'],
    proxyBodySize: true,
    proxyTimeouts: ['connect', 'read', 'send'],
    proxyRetry: ['conditions', 'tries', 'timeout'],
    proxySSLVerify: true,
    proxySSLVerifyDepth: true,
    proxyResponseHeaders: true,
    proxyAccessControl: true,
    proxyHeaderVars: true,
    upstreamRequestRewrite: true,
    upstreamTuning: true,
    redirect404Options: true,
    rewritePresets: true,
    stat: true,
    defaultSite: true,
    errorLog: true,
    rateLimit: true,
    rateLimitConn: true,
    realIP: true,
    realIPRecursive: true,
    lsCache: false,
    upstreamAlgos: ['', 'least_conn', 'ip_hash', 'random'],
    upstreamKeepalive: true,
    upstreamOptions: 'all',
    lang: 'nginx',
  },
  apache: {
    quic: false,
    ipv6Listen: false,
    ocsp: true,
    tlsVersions: allTLSVersions,
    resolver: false,
    matchType: true,
    proxySNI: false,
    proxyCache: 'ttl',
    proxyReplaces: true,
    proxyHTTPVersions: [],
    proxyBodySize: true,
    proxyTimeouts: ['connect', 'read'],
    proxyRetry: [],
    proxySSLVerify: true,
    proxySSLVerifyDepth: true,
    proxyResponseHeaders: true,
    proxyAccessControl: true,
    proxyHeaderVars: true,
    upstreamRequestRewrite: true,
    upstreamTuning: false,
    redirect404Options: true,
    rewritePresets: false,
    stat: false,
    defaultSite: false,
    errorLog: true,
    rateLimit: true,
    rateLimitConn: false,
    realIP: true,
    realIPRecursive: false,
    lsCache: false,
    upstreamAlgos: ['', 'least_conn'],
    upstreamKeepalive: false,
    upstreamOptions: 'all',
    lang: 'apacheconf',
  },
  openlitespeed: {
    quic: true,
    ipv6Listen: true,
    ocsp: true,
    tlsVersions: allTLSVersions,
    resolver: false,
    matchType: true,
    proxySNI: false,
    proxyCache: false,
    proxyReplaces: false,
    proxyHTTPVersions: [],
    proxyBodySize: false,
    proxyTimeouts: [],
    proxyRetry: [],
    proxySSLVerify: false,
    proxySSLVerifyDepth: false,
    proxyResponseHeaders: true,
    proxyAccessControl: true,
    proxyHeaderVars: false,
    upstreamRequestRewrite: false,
    upstreamTuning: false,
    redirect404Options: false,
    rewritePresets: true,
    stat: false,
    defaultSite: false,
    errorLog: true,
    rateLimit: false,
    rateLimitConn: false,
    realIP: false,
    realIPRecursive: false,
    lsCache: true,
    upstreamAlgos: [''],
    upstreamKeepalive: false,
    upstreamOptions: 'none',
    lang: 'plaintext',
  },
  caddy: {
    quic: false,
    ipv6Listen: false,
    ocsp: false,
    tlsVersions: ['TLSv1.2', 'TLSv1.3'],
    resolver: false,
    matchType: true,
    proxySNI: true,
    proxyCache: false,
    proxyReplaces: true,
    proxyHTTPVersions: ['1.1', '2'],
    proxyBodySize: true,
    proxyTimeouts: ['connect', 'read'],
    proxyRetry: ['tries', 'timeout'],
    proxySSLVerify: true,
    proxySSLVerifyDepth: false,
    proxyResponseHeaders: true,
    proxyAccessControl: true,
    proxyHeaderVars: true,
    upstreamRequestRewrite: true,
    upstreamTuning: true,
    redirect404Options: true,
    rewritePresets: true,
    stat: true,
    defaultSite: true,
    errorLog: false,
    rateLimit: false,
    rateLimitConn: false,
    realIP: false,
    realIPRecursive: false,
    lsCache: false,
    upstreamAlgos: ['', 'least_conn', 'ip_hash', 'random'],
    upstreamKeepalive: false,
    upstreamOptions: 'weight',
    lang: 'plaintext',
  },
}

const unknown: WebServerFeatures = {
  quic: false,
  ipv6Listen: false,
  ocsp: false,
  tlsVersions: allTLSVersions,
  resolver: false,
  matchType: false,
  proxySNI: false,
  proxyCache: false,
  proxyReplaces: false,
  proxyHTTPVersions: [],
  proxyBodySize: false,
  proxyTimeouts: [],
  proxyRetry: [],
  proxySSLVerify: false,
  proxySSLVerifyDepth: false,
  proxyResponseHeaders: false,
  proxyAccessControl: false,
  proxyHeaderVars: false,
  upstreamRequestRewrite: false,
  upstreamTuning: false,
  redirect404Options: false,
  rewritePresets: false,
  stat: false,
  defaultSite: false,
  errorLog: true,
  rateLimit: false,
  rateLimitConn: false,
  realIP: false,
  realIPRecursive: false,
  lsCache: false,
  upstreamAlgos: [''],
  upstreamKeepalive: false,
  upstreamOptions: 'none',
  lang: 'plaintext',
}

// 取 Web 服务器能力，未知类型全部降级
export function webserverFeatures(webserver: string): WebServerFeatures {
  return features[webserver] ?? unknown
}

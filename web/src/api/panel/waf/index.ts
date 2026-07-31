import { http } from '@/utils'

export type WafPolicyApplyState = 'saved' | 'pending' | 'applied' | 'failed'

export interface WafPolicy {
  id: number
  apply_status?: WafPolicyApplyState
  version?: number | null
  target_version?: number | null
  applied_version?: number | null
  last_error?: string | null
  [key: string]: any
}

export interface WafPolicyStatus {
  policy_id: number
  apply_status: WafPolicyApplyState
  target_version: number
  applied_version: number
  last_error: string
}

export interface WafDecisionInput {
  type: 'ban' | 'allow' | 'captcha'
  value: string
  until: number
}

export const policyApplyState = (policy: WafPolicy): WafPolicyApplyState =>
  policy.apply_status ?? 'saved'

export default {
  // ===================== 策略 =====================
  // 策略列表
  policies: (): any => http.Get('/waf/policies'),
  // 获取策略
  policy: (policyId: number | string): any => http.Get(`/waf/policies/${policyId}`),
  // 获取策略应用状态
  policyStatus: (policyId: number | string): any => http.Get(`/waf/policies/${policyId}/status`),
  // 创建策略
  createPolicy: (data: any): any => http.Post('/waf/policies', data),
  // 更新策略
  updatePolicy: (policyId: number | string, data: any): any =>
    http.Put(`/waf/policies/${policyId}`, data),
  // 删除策略
  deletePolicy: (policyId: number | string): any => http.Delete(`/waf/policies/${policyId}`),

  // ===================== 误报加白 =====================
  // 加白列表
  exclusions: (policyId: number | string): any => http.Get(`/waf/policies/${policyId}/exclusions`),
  // 创建加白
  createExclusion: (policyId: number | string, data: any): any =>
    http.Post(`/waf/policies/${policyId}/exclusions`, data),
  // 删除加白
  deleteExclusion: (policyId: number | string, exclusionId: number): any =>
    http.Delete(`/waf/policies/${policyId}/exclusions`, undefined, {
      params: { id: exclusionId },
    }),

  // ===================== 决策黑白名单 =====================
  // 决策列表
  decisions: (page: number, limit: number): any =>
    http.Get('/waf/decisions', { params: { page, limit } }),
  // 创建/更新决策
  createDecision: (data: WafDecisionInput): any => http.Post('/waf/decisions', data),
  // 删除决策
  deleteDecision: (decisionId: number): any =>
    http.Delete('/waf/decisions', undefined, { params: { id: decisionId } }),

  // ===================== 报表 =====================
  // 事件日志
  events: (params: any): any => http.Get('/waf/events', { params }),
  // 统计总览
  stats: (since?: number): any => http.Get('/waf/stats', { params: { since } }),
  // 攻击地图
  attackMap: (since?: number): any => http.Get('/waf/attack-map', { params: { since } }),

  // ===================== 网站绑定 + 启停 =====================
  // 绑定列表
  bindings: (): any => http.Get('/waf/bindings'),
  // 网站启用 WAF
  enableWebsite: (data: any): any => http.Post('/waf/website/enable', data),
  // 网站关闭 WAF
  disableWebsite: (websiteId: number): any => http.Post(`/waf/website/${websiteId}/disable`),
}

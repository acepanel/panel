<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf, { policyApplyState, type WafPolicy } from '@/api/panel/waf'
import AclRuleBuilder from '@/views/waf/components/AclRuleBuilder.vue'
import RateLimitTable from '@/views/waf/components/RateLimitTable.vue'
import ToleranceTable from '@/views/waf/components/ToleranceTable.vue'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{ policyId: number }>()
const emit = defineEmits<{ saved: [policy: WafPolicy] }>()

const loading = ref(false)
const saving = ref(false)
const current = ref('basic')
let loadToken = 0

const isEdit = computed(() => props.policyId > 0)
const title = computed(() => (isEdit.value ? $gettext('Edit Policy') : $gettext('Create Policy')))

// 新策略的完整挑战配置
const defaultChallenge = () => ({
  enabled: false,
  type: 'js_pow',
  difficulty: 16,
  clearance_ttl: 1800,
  challenge_ttl: 60,
  bind_fields: ['ip', 'ua'],
  captcha_length: 5,
  capacity: 100,
})

// 默认 Bot 设置
const defaultBot = () => ({
  enabled: false,
  block_ai_crawlers: false,
  allow_verified_search_engines: true,
  // n-dynamic-tags 运行时产出字符串数组，保存时再数值化为 ASN
  deny_asn: [] as string[],
  deny_country: [] as string[],
})

// 完整策略默认模型
const defaultModel = () => ({
  id: 0,
  name: '',
  enabled: true,
  mode: 'block',
  remark: '',
  security_level: 'standard',
  inspect_response: false,
  // 结构化规则（acl / ratelimit / tolerance 混存于同一数组 由 type 区分）
  rules: [] as any[],
  // Bot 与挑战扩展配置
  bot: defaultBot(),
  challenge: defaultChallenge(),
})

const model = ref<any>(defaultModel())

// ACL 规则（type=acl）双向视图
const aclRules = computed({
  get: () => model.value.rules.filter((r: any) => r.type === 'acl'),
  set: (v: any[]) => {
    model.value.rules = [...v, ...model.value.rules.filter((r: any) => r.type !== 'acl')]
  },
})

// CC 限流规则（type=ratelimit）双向视图
const rateLimitRules = computed({
  get: () => model.value.rules.filter((r: any) => r.type === 'ratelimit'),
  set: (v: any[]) => {
    model.value.rules = [...model.value.rules.filter((r: any) => r.type !== 'ratelimit'), ...v]
  },
})

// 容忍度规则（type=tolerance）双向视图
const toleranceRules = computed({
  get: () => model.value.rules.filter((r: any) => r.type === 'tolerance'),
  set: (v: any[]) => {
    model.value.rules = [...model.value.rules.filter((r: any) => r.type !== 'tolerance'), ...v]
  },
})

watch([show, () => props.policyId], ([v]) => {
  const token = ++loadToken
  if (v) {
    current.value = 'basic'
    if (isEdit.value) {
      loading.value = true
      useRequest(waf.policy(props.policyId))
        .onSuccess(({ data }: any) => {
          if (token !== loadToken || !show.value) return
          model.value = {
            ...defaultModel(),
            ...data,
            rules: Array.isArray(data.rules) ? data.rules : [],
            bot: { ...defaultBot(), ...data.bot },
            challenge: { ...defaultChallenge(), ...data.challenge },
          }
        })
        .onComplete(() => {
          if (token === loadToken) loading.value = false
        })
    } else {
      loading.value = false
      model.value = defaultModel()
    }
  } else {
    loading.value = false
  }
})

const modeOptions = computed(() => [
  { label: $gettext('Block'), value: 'block' },
  { label: $gettext('Observe (log only)'), value: 'observe' },
])

const securityLevelOptions = computed(() => [
  { label: $gettext('Standard'), value: 'standard' },
  { label: $gettext('Strict'), value: 'strict' },
])

const bindFieldOptions = computed(() => [
  { label: $gettext('IP'), value: 'ip' },
  { label: $gettext('User-Agent'), value: 'ua' },
  { label: $gettext('Accept-Language'), value: 'accept_language' },
])

const challengeTypeOptions = computed(() => [
  { label: $gettext('JS Proof of Work'), value: 'js_pow' },
  { label: $gettext('Captcha'), value: 'captcha' },
  { label: $gettext('Waiting Room'), value: 'waiting_room' },
])

const handleSave = () => {
  if (loading.value || saving.value) return
  if (!model.value.name.trim()) {
    window.$message.error($gettext('Please enter a policy name'))
    return
  }
  saving.value = true
  const policy = { ...model.value }
  delete policy.exclusions
  delete policy.version
  delete policy.target_version
  delete policy.applied_version
  delete policy.last_error
  const payload = {
    ...policy,
    rules: model.value.rules,
    // n-dynamic-tags 始终产出字符串数组，ASN 需数值化后下发，否则 agent 解 []uint32 报错导致整条策略保存失败
    bot: {
      ...model.value.bot,
      deny_asn: (model.value.bot?.deny_asn || [])
        .map((v: any) => Number(v))
        .filter((n: number) => Number.isInteger(n) && n >= 0),
    },
  }
  const request = isEdit.value
    ? waf.updatePolicy(props.policyId, payload)
    : waf.createPolicy(payload)
  useRequest(request)
    .onSuccess(({ data }: { data: WafPolicy }) => {
      emit('saved', data)
      show.value = false
      switch (policyApplyState(data)) {
        case 'applied':
          window.$message.success($gettext('Policy saved and applied'))
          break
        case 'pending':
          window.$message.info($gettext('Policy saved and pending application'))
          break
        case 'failed':
          window.$message.error(
            `${$gettext('Policy saved but failed to apply')}: ${data.last_error || '-'}`,
          )
          break
        default:
          window.$message.success($gettext('Policy saved'))
      }
    })
    .onComplete(() => {
      saving.value = false
    })
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="title"
    :style="{ width: '80vw', maxWidth: '1100px' }"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-spin :show="loading">
      <n-tabs v-model:value="current" type="line" animated>
        <!-- 基础设置 -->
        <n-tab-pane name="basic" :tab="$gettext('Basic')">
          <n-form :model="model" label-placement="top">
            <n-grid :cols="24" :x-gap="16">
              <n-form-item-gi :span="12" :label="$gettext('Name')">
                <n-input v-model:value="model.name" :placeholder="$gettext('Policy name')" />
              </n-form-item-gi>
              <n-form-item-gi :span="6" :label="$gettext('Enabled')">
                <n-switch v-model:value="model.enabled" />
              </n-form-item-gi>
              <n-form-item-gi :span="6" :label="$gettext('Mode')">
                <n-select v-model:value="model.mode" :options="modeOptions" />
              </n-form-item-gi>
              <n-form-item-gi :span="24" :label="$gettext('Remark')">
                <n-input
                  v-model:value="model.remark"
                  type="textarea"
                  :placeholder="$gettext('Optional')"
                  :autosize="{ minRows: 2, maxRows: 4 }"
                />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-tab-pane>

        <!-- CC 限流 -->
        <n-tab-pane name="ratelimit" :tab="$gettext('Rate Limit (CC)')">
          <rate-limit-table v-model="rateLimitRules" />
        </n-tab-pane>

        <!-- 容忍度拉黑 -->
        <n-tab-pane name="tolerance" :tab="$gettext('Tolerance Ban')">
          <tolerance-table v-model="toleranceRules" />
        </n-tab-pane>

        <!-- 黑白名单（ACL 与或非构建器） -->
        <n-tab-pane name="acl" :tab="$gettext('Access Rules')">
          <acl-rule-builder v-model="aclRules" />
        </n-tab-pane>

        <!-- 语义检测 -->
        <n-tab-pane name="security" :tab="$gettext('Semantic Detection')">
          <n-form :model="model" label-placement="top">
            <n-grid :cols="24" :x-gap="16">
              <n-form-item-gi :span="12" :label="$gettext('Security Level')">
                <n-select v-model:value="model.security_level" :options="securityLevelOptions" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Response Inspection')">
                <n-switch v-model:value="model.inspect_response" />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-tab-pane>

        <!-- Bot 设置 -->
        <n-tab-pane name="bot" :tab="$gettext('Bot Management')">
          <n-form :model="model.bot" label-placement="top">
            <n-grid :cols="24" :x-gap="16">
              <n-form-item-gi :span="8" :label="$gettext('Enable Bot Management')">
                <n-switch v-model:value="model.bot.enabled" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" :label="$gettext('Block AI Crawlers')">
                <n-switch v-model:value="model.bot.block_ai_crawlers" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" :label="$gettext('Allow Verified Search Engines')">
                <n-switch v-model:value="model.bot.allow_verified_search_engines" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" :label="$gettext('Auto-ban flagged bots by country')">
                <n-dynamic-tags
                  v-model:value="model.bot.deny_country"
                  :placeholder="$gettext('e.g., CN, RU')"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="8" :label="$gettext('Auto-ban flagged bots by ASN')">
                <n-dynamic-tags
                  v-model:value="model.bot.deny_asn"
                  :placeholder="$gettext('e.g., 4134')"
                />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-tab-pane>

        <!-- 挑战设置 -->
        <n-tab-pane name="challenge" :tab="$gettext('Challenge')">
          <n-form :model="model.challenge" label-placement="top">
            <n-grid :cols="24" :x-gap="16">
              <n-form-item-gi
                :span="24"
                :label="$gettext('Enable Challenge (all requests must pass verification)')"
              >
                <n-switch v-model:value="model.challenge.enabled" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Challenge Type')">
                <n-select v-model:value="model.challenge.type" :options="challengeTypeOptions" />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="model.challenge.type === 'js_pow'"
                :span="12"
                :label="$gettext('Difficulty (PoW leading zero bits)')"
              >
                <n-input-number
                  v-model:value="model.challenge.difficulty"
                  :min="1"
                  :max="32"
                  w-full
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Clearance TTL (seconds)')">
                <n-input-number v-model:value="model.challenge.clearance_ttl" :min="1" w-full />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Challenge TTL (seconds)')">
                <n-input-number v-model:value="model.challenge.challenge_ttl" :min="1" w-full />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Bind Fields')">
                <n-select
                  v-model:value="model.challenge.bind_fields"
                  multiple
                  :options="bindFieldOptions"
                />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="model.challenge.type === 'captcha'"
                :span="12"
                :label="$gettext('Captcha Length')"
              >
                <n-input-number
                  v-model:value="model.challenge.captcha_length"
                  :min="4"
                  :max="8"
                  w-full
                />
              </n-form-item-gi>
              <n-form-item-gi
                v-if="model.challenge.type === 'waiting_room'"
                :span="12"
                :label="$gettext('Waiting Room Capacity')"
              >
                <n-input-number
                  v-model:value="model.challenge.capacity"
                  :min="1"
                  :max="4096"
                  w-full
                />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-tab-pane>
      </n-tabs>
    </n-spin>
    <template #footer>
      <n-button
        type="info"
        block
        :loading="saving"
        :disabled="loading || saving"
        @click="handleSave"
      >
        {{ $gettext('Save') }}
      </n-button>
    </template>
  </n-modal>
</template>

<style scoped lang="scss"></style>

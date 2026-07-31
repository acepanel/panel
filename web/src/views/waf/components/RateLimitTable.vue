<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

// 规则数组（type=ratelimit），由父级 v-model 提供
const rules = defineModel<any[]>({ required: true })

const actionOptions = computed(() => [
  { label: $gettext('Block'), value: 'block' },
  { label: $gettext('Challenge'), value: 'challenge' },
  { label: $gettext('Log only'), value: 'log' },
])

// 挑战类型：空=沿用策略全局挑战类型
const challengeTypeOptions = computed(() => [
  { label: $gettext('Use policy default'), value: '' },
  { label: $gettext('JS Proof of Work'), value: 'js_pow' },
  { label: $gettext('Captcha'), value: 'captcha' },
  { label: $gettext('Waiting Room'), value: 'waiting_room' },
])

let idCounter = 0
const genKey = () => `_rl_${Date.now()}_${++idCounter}`

const addRule = () => {
  rules.value = [
    ...(rules.value || []),
    {
      _key: genKey(),
      type: 'ratelimit',
      enabled: true,
      priority: 0,
      name: '',
      uri_pattern: '',
      key: 'ip',
      capacity: 100,
      leak_per_sec: 10,
      action: 'block',
      // challenge_type：action=challenge 时本规则专用挑战类型，空=沿用策略全局挑战类型
      challenge_type: '',
    },
  ]
}

const removeRule = (index: number) => {
  const next = [...(rules.value || [])]
  next.splice(index, 1)
  rules.value = next
}

// 确保已有规则带本地唯一键
watchEffect(() => {
  rules.value?.forEach((r: any) => {
    if (!r._key) r._key = genKey()
  })
})
</script>

<template>
  <n-flex vertical :size="16">
    <n-alert type="info" :bordered="false">
      {{
        $gettext(
          'Leaky bucket rate limiting. Differentiate per URL: capacity is the burst size, leak rate is requests drained per second. Aggregation key supports ip, session, cookie:<name>, arg:<name>, header:<name>.'
        )
      }}
    </n-alert>

    <n-empty v-if="!rules || rules.length === 0" :description="$gettext('No rate limit rules')" />

    <n-card
      v-for="(rule, index) in rules"
      :key="rule._key"
      closable
      size="small"
      @close="removeRule(index)"
    >
      <template #header>
        <n-flex align="center" :size="8">
          <n-switch v-model:value="rule.enabled" size="small" />
          <span>{{ $gettext('Rule') }} #{{ index + 1 }}</span>
        </n-flex>
      </template>
      <n-form label-placement="top">
        <n-grid :cols="24" :x-gap="12">
          <n-form-item-gi :span="8" :label="$gettext('Name')">
            <n-input v-model:value="rule.name" :placeholder="$gettext('Optional')" />
          </n-form-item-gi>
          <n-form-item-gi :span="16" :label="$gettext('URI Prefix (~regex, empty = whole site)')">
            <n-input v-model:value="rule.uri_pattern" placeholder="/api/" />
          </n-form-item-gi>
          <n-form-item-gi :span="8" :label="$gettext('Aggregation Key')">
            <n-input v-model:value="rule.key" placeholder="ip" />
          </n-form-item-gi>
          <n-form-item-gi :span="5" :label="$gettext('Capacity (burst)')">
            <n-input-number v-model:value="rule.capacity" :min="1" w-full />
          </n-form-item-gi>
          <n-form-item-gi :span="5" :label="$gettext('Leak per Second')">
            <n-input-number v-model:value="rule.leak_per_sec" :min="1" w-full />
          </n-form-item-gi>
          <n-form-item-gi :span="6" :label="$gettext('Action')">
            <n-select v-model:value="rule.action" :options="actionOptions" />
          </n-form-item-gi>
          <n-form-item-gi
            v-if="rule.action === 'challenge'"
            :span="24"
            :label="$gettext('Challenge Type')"
          >
            <n-select v-model:value="rule.challenge_type" :options="challengeTypeOptions" />
          </n-form-item-gi>
        </n-grid>
      </n-form>
    </n-card>

    <n-button type="primary" dashed @click="addRule">
      {{ $gettext('Add Rate Limit Rule') }}
    </n-button>
  </n-flex>
</template>

<style scoped lang="scss"></style>

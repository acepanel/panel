<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

// 规则数组（type=tolerance），由父级 v-model 提供
const rules = defineModel<any[]>({ required: true })

// category 契约为全小写，与数据面字节精确比较
const categoryOptions = computed(() => [
  { label: $gettext('Challenge failures'), value: 'challenge_fail' },
  { label: $gettext('Attacks (injection etc.)'), value: 'attack' },
  { label: $gettext('Rate limit hits (CC)'), value: 'cc' },
])

let idCounter = 0
const genKey = () => `_tol_${Date.now()}_${++idCounter}`

const addRule = () => {
  rules.value = [
    ...(rules.value || []),
    {
      _key: genKey(),
      type: 'tolerance',
      enabled: true,
      priority: 0,
      name: '',
      category: 'attack',
      threshold: 10,
      window_sec: 60,
      ban_seconds: 600,
      ban_prefix: 0,
      push_l4: false,
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
          'Tolerance-based banning: count violations per IP within a fixed window, ban when the threshold is exceeded. Ban prefix 0 bans the exact IP; e.g. 24 bans the whole /24 subnet (useful against botnets).'
        )
      }}
    </n-alert>

    <n-empty v-if="!rules || rules.length === 0" :description="$gettext('No tolerance rules')" />

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
          <n-form-item-gi :span="8" :label="$gettext('Violation Category')">
            <n-select v-model:value="rule.category" :options="categoryOptions" />
          </n-form-item-gi>
          <n-form-item-gi :span="4" :label="$gettext('Threshold')">
            <n-input-number v-model:value="rule.threshold" :min="1" w-full />
          </n-form-item-gi>
          <n-form-item-gi :span="4" :label="$gettext('Window (seconds)')">
            <n-input-number v-model:value="rule.window_sec" :min="1" w-full />
          </n-form-item-gi>
          <n-form-item-gi :span="8" :label="$gettext('Ban Duration (seconds, 0 = permanent)')">
            <n-input-number v-model:value="rule.ban_seconds" :min="0" w-full />
          </n-form-item-gi>
          <n-form-item-gi :span="8" :label="$gettext('Ban Prefix (CIDR bits, 0 = exact IP)')">
            <n-input-number v-model:value="rule.ban_prefix" :min="0" :max="128" w-full />
          </n-form-item-gi>
          <n-form-item-gi :span="8" :label="$gettext('Push to L4 Firewall')">
            <n-switch v-model:value="rule.push_l4" />
          </n-form-item-gi>
        </n-grid>
      </n-form>
    </n-card>

    <n-button type="primary" dashed @click="addRule">
      {{ $gettext('Add Tolerance Rule') }}
    </n-button>
  </n-flex>
</template>

<style scoped lang="scss"></style>

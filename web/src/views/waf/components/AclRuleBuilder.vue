<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

// ACL 规则数组
const rules = defineModel<any[]>({ required: true })

const actionOptions = computed(() => [
  { label: $gettext('Allow'), value: 'allow' },
  { label: $gettext('Deny'), value: 'deny' },
  { label: $gettext('Challenge'), value: 'challenge' },
  { label: $gettext('Log only'), value: 'log' },
])

// 字段：含子键的字段（header/arg/cookie）需要额外 name
const fieldOptions = computed(() => [
  { label: $gettext('IP'), value: 'ip' },
  { label: $gettext('User-Agent'), value: 'ua' },
  { label: $gettext('URI'), value: 'uri' },
  { label: $gettext('Host'), value: 'host' },
  { label: $gettext('Method'), value: 'method' },
  { label: $gettext('Header'), value: 'header' },
  { label: $gettext('Argument'), value: 'arg' },
  { label: $gettext('Cookie'), value: 'cookie' },
  { label: $gettext('JA4'), value: 'ja4' },
])

const operatorOptions = computed(() => [
  { label: $gettext('Equals'), value: 'eq' },
  { label: $gettext('Contains'), value: 'contains' },
  { label: $gettext('Regex'), value: 'regex' },
  { label: $gettext('Prefix'), value: 'prefix' },
  { label: $gettext('Suffix'), value: 'suffix' },
  { label: $gettext('IP Match (CIDR)'), value: 'ipmatch' },
])

const operatorsForField = (field: string) =>
  field === 'ja4'
    ? operatorOptions.value.filter((option) => option.value !== 'ipmatch')
    : operatorOptions.value

// 需要子键名的字段
const needsName = (field: string) => ['header', 'arg', 'cookie'].includes(field)

let idCounter = 0
const genKey = () => `_acl_${Date.now()}_${++idCounter}`

const newCondition = () => ({
  _key: genKey(),
  field: 'ip',
  name: '',
  operator: 'ipmatch',
  value: '',
  negate: false,
})

const addRule = () => {
  rules.value = [
    ...(rules.value || []),
    {
      _key: genKey(),
      type: 'acl',
      enabled: true,
      priority: 0,
      name: '',
      uri_pattern: '',
      action: 'deny',
      negate: false,
      conditions: [newCondition()],
    },
  ]
}

const removeRule = (index: number) => {
  const next = [...(rules.value || [])]
  next.splice(index, 1)
  rules.value = next
}

const addCondition = (rule: any) => {
  if (!rule.conditions) rule.conditions = []
  rule.conditions.push(newCondition())
}

const removeCondition = (rule: any, index: number) => {
  rule.conditions.splice(index, 1)
}

// 确保已有规则与条件带本地唯一键
watchEffect(() => {
  rules.value?.forEach((r: any) => {
    if (!r._key) r._key = genKey()
    r.conditions?.forEach((c: any) => {
      if (!c._key) c._key = genKey()
      if (c.field === 'ja4' && c.operator === 'ipmatch') c.operator = 'eq'
    })
  })
})
</script>

<template>
  <n-flex vertical :size="16">
    <n-alert type="info" :bordered="false">
      {{
        $gettext(
          'Each rule combines multiple conditions with AND. Multiple rules are evaluated as OR. A rule (or single condition) can be negated with NOT. The action applies when the rule matches.'
        )
      }}
    </n-alert>

    <n-empty v-if="!rules || rules.length === 0" :description="$gettext('No access rules')" />

    <template v-for="(rule, rIndex) in rules" :key="rule._key">
      <!-- 规则之间的 OR 分隔 -->
      <n-divider v-if="rIndex > 0" class="!my-0">
        <n-tag type="warning" size="small">{{ $gettext('OR') }}</n-tag>
      </n-divider>

      <n-card closable size="small" @close="removeRule(rIndex)">
        <template #header>
          <n-flex align="center" :size="8">
            <n-switch v-model:value="rule.enabled" size="small" />
            <span>{{ $gettext('Rule') }} #{{ rIndex + 1 }}</span>
          </n-flex>
        </template>

        <n-form label-placement="top">
          <n-grid :cols="24" :x-gap="12">
            <n-form-item-gi :span="6" :label="$gettext('Name')">
              <n-input v-model:value="rule.name" :placeholder="$gettext('Optional')" />
            </n-form-item-gi>
            <n-form-item-gi :span="6" :label="$gettext('Action')">
              <n-select v-model:value="rule.action" :options="actionOptions" />
            </n-form-item-gi>
            <n-form-item-gi :span="6" :label="$gettext('Priority')">
              <n-input-number v-model:value="rule.priority" w-full />
            </n-form-item-gi>
            <n-form-item-gi :span="6" :label="$gettext('Negate Whole Rule (NOT)')">
              <n-switch v-model:value="rule.negate" />
            </n-form-item-gi>
            <n-form-item-gi :span="24" :label="$gettext('URI Regex')">
              <n-input v-model:value="rule.uri_pattern" :placeholder="$gettext('Optional')" />
            </n-form-item-gi>
          </n-grid>
        </n-form>

        <!-- 条件列表（AND） -->
        <n-flex vertical :size="8">
          <template v-for="(cond, cIndex) in rule.conditions" :key="cond._key">
            <n-flex v-if="Number(cIndex) > 0" justify="center" class="!my-1">
              <n-tag type="success" size="small">{{ $gettext('AND') }}</n-tag>
            </n-flex>
            <n-flex align="flex-start" :size="8" :wrap="false">
              <n-select
                v-model:value="cond.field"
                :options="fieldOptions"
                class="w-32"
                :placeholder="$gettext('Field')"
              />
              <n-input
                v-if="needsName(cond.field)"
                v-model:value="cond.name"
                class="w-32"
                :placeholder="$gettext('Key name')"
              />
              <n-select
                v-model:value="cond.operator"
                :options="operatorsForField(cond.field)"
                class="w-36"
                :placeholder="$gettext('Operator')"
              />
              <n-input
                v-model:value="cond.value"
                class="flex-1"
                :placeholder="$gettext('Value')"
              />
              <n-tooltip>
                <template #trigger>
                  <n-button
                    :type="cond.negate ? 'warning' : 'default'"
                    secondary
                    @click="cond.negate = !cond.negate"
                  >
                    {{ $gettext('NOT') }}
                  </n-button>
                </template>
                {{ $gettext('Negate this single condition') }}
              </n-tooltip>
              <n-button
                type="error"
                secondary
                :disabled="rule.conditions.length <= 1"
                @click="removeCondition(rule, Number(cIndex))"
              >
                <the-icon icon="mdi:close" :size="16" />
              </n-button>
            </n-flex>
          </template>

          <n-button size="small" dashed @click="addCondition(rule)">
            {{ $gettext('Add Condition (AND)') }}
          </n-button>
        </n-flex>
      </n-card>
    </template>

    <n-button type="primary" dashed @click="addRule">
      {{ $gettext('Add Rule (OR)') }}
    </n-button>
  </n-flex>
</template>

<style scoped lang="scss"></style>

<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import alert from '@/api/panel/alert'
import notify from '@/api/panel/notify'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { isStatusMetric, useAlertMetrics } from '@/views/monitor/metrics'

const { $gettext } = useGettext()
const { metricOptions, operators, metricOf } = useAlertMetrics()
const formLayout = useFormLayout(120)

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  rule?: any
}>()
const emit = defineEmits(['saved'])

const isEdit = computed(() => !!props.rule)

const model = ref({
  name: '',
  type: 'cpu',
  target: '',
  operator: 'gt',
  threshold: 90,
  duration: 3,
  silence: 30,
  channels: [] as number[],
  enabled: true,
})

const channels = ref<{ label: string; value: number }[]>([])

const currentMetric = computed(() => metricOf(model.value.type))
// 状态类规则语义固定为「不在运行」，无需运算符与阈值
const isStatus = computed(() => isStatusMetric(model.value.type))

const loadChannels = () => {
  useRequest(notify.allChannels()).onSuccess(({ data }: any) => {
    channels.value = (data || []).map((item: any) => ({ label: item.name, value: item.id }))
  })
}

// 各指标的默认条件，未列出的按使用率类默认 90%
const defaultConditions: Record<string, { operator: string; threshold: number }> = {
  cert_expire: { operator: 'lt', threshold: 7 },
  website_expire: { operator: 'lt', threshold: 7 },
  load1: { operator: 'gt', threshold: 10 },
  load5: { operator: 'gt', threshold: 10 },
  load15: { operator: 'gt', threshold: 10 },
  disk_read: { operator: 'gt', threshold: 100 },
  disk_write: { operator: 'gt', threshold: 100 },
  net_in: { operator: 'gt', threshold: 100 },
  net_out: { operator: 'gt', threshold: 100 },
  website_5xx: { operator: 'gt', threshold: 10 },
  website_error: { operator: 'gt', threshold: 5 },
}

// 切换指标时给出符合语义的默认条件
const handleTypeChange = (value: string) => {
  model.value.target = ''
  if (isStatusMetric(value)) {
    // 与后端 normalizeRule 保持一致
    model.value.operator = 'gte'
    model.value.threshold = 1
    return
  }

  const condition = defaultConditions[value] ?? { operator: 'gt', threshold: 90 }
  model.value.operator = condition.operator
  model.value.threshold = condition.threshold
}

watch(show, (val) => {
  if (!val) return
  loadChannels()
  if (props.rule) {
    model.value = {
      name: props.rule.name,
      type: props.rule.type,
      target: props.rule.target,
      operator: props.rule.operator,
      threshold: props.rule.threshold,
      duration: props.rule.duration,
      silence: props.rule.silence,
      channels: [...(props.rule.channels || [])],
      enabled: props.rule.enabled,
    }
  } else {
    model.value = {
      name: '',
      type: 'cpu',
      target: '',
      operator: 'gt',
      threshold: 90,
      duration: 3,
      silence: 30,
      channels: [],
      enabled: true,
    }
  }
})

const save = async () => {
  const req = isEdit.value
    ? alert.updateRule(props.rule.id, model.value)
    : alert.createRule(model.value)
  await req
  window.$message.success($gettext('Saved successfully'))
  show.value = false
  emit('saved')
}

// 校验不通过时同步返回，避免保存按钮闪一下加载图标
const handleSubmit = () => {
  if (currentMetric.value.target === 'required' && !model.value.target) {
    window.$message.error($gettext('Please enter the target'))
    return
  }
  return save()
}
</script>

<template>
  <app-modal
    size="sm"
    v-model:show="show"
    :title="isEdit ? $gettext('Edit Rule') : $gettext('Add Rule')"
  >
    <n-form v-bind="formLayout">
      <n-form-item :label="$gettext('Name')" required>
        <n-input v-model:value="model.name" :placeholder="$gettext('Rule name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Metric')" required>
        <n-select
          v-model:value="model.type"
          :options="metricOptions"
          @update:value="handleTypeChange"
        />
      </n-form-item>
      <n-form-item
        v-if="currentMetric.target !== 'none'"
        :label="$gettext('Target')"
        :required="currentMetric.target === 'required'"
      >
        <n-input v-model:value="model.target" :placeholder="currentMetric.placeholder" />
      </n-form-item>
      <n-form-item v-if="!isStatus" :label="$gettext('Condition')" required>
        <n-flex :size="8" class="w-full">
          <n-select v-model:value="model.operator" :options="operators" class="md:!w-50" />
          <n-input-number v-model:value="model.threshold" :min="0" class="flex-1">
            <template v-if="currentMetric.unit" #suffix>{{ currentMetric.unit }}</template>
          </n-input-number>
        </n-flex>
      </n-form-item>
      <n-form-item :label="$gettext('Trigger After')">
        <n-input-number v-model:value="model.duration" :min="1" :max="60" class="w-full">
          <template #suffix>{{ $gettext('consecutive checks') }}</template>
        </n-input-number>
      </n-form-item>
      <n-form-item :label="$gettext('Silence Period')">
        <n-input-number v-model:value="model.silence" :min="0" :max="1440" class="w-full">
          <template #suffix>{{ $gettext('minutes') }}</template>
        </n-input-number>
      </n-form-item>
      <n-form-item :label="$gettext('Notify Channels')">
        <n-select
          v-model:value="model.channels"
          multiple
          clearable
          :options="channels"
          :placeholder="$gettext('Record only when empty')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Enabled')">
        <n-switch v-model:value="model.enabled" />
      </n-form-item>
    </n-form>
    <n-alert type="info" :bordered="false">
      {{
        $gettext(
          'Metrics are checked every minute. The alert fires only after the condition holds for the configured consecutive checks, and is not repeated within the silence period.',
        )
      }}
    </n-alert>
    <template #footer>
      <n-flex justify="end">
        <n-button @click="show = false">{{ $gettext('Cancel') }}</n-button>
        <app-button type="primary" @click="handleSubmit">
          {{ $gettext('Save') }}
        </app-button>
      </n-flex>
    </template>
  </app-modal>
</template>

<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import type { CronType } from './cron'
import { classifyCron } from './cron'

const { $gettext } = useGettext()

// 生成的 Cron 表达式值
const value = defineModel<string>('value', {
  type: String,
  required: true,
})

const props = defineProps<{
  // 不提供的周期类型
  exclude?: CronType[]
}>()

// 当前选择的周期类型
const selectedOption = ref<CronType>('every-n-minutes')

// 表单数据
const formData = ref({
  // Every N 系列
  nSeconds: 5, // 每 N 秒
  nMinutes: 30, // 每 N 分钟
  nHours: 2, // 每 N 小时
  nDays: 3, // 每 N 天

  // 时间配置
  minute: 30, // 分钟
  hour: 1, // 小时
  day: 3, // 日期（1-31）
  month: 1, // 月份（1-12）
  weekday: 1, // 星期（0-6，0=周日）

  // 自定义表达式
  customCron: '* * * * *',
})

// 是否已初始化
const initialized = ref(false)

const STEP_FIELDS: Partial<Record<CronType, 'nSeconds' | 'nMinutes' | 'nHours' | 'nDays'>> = {
  'every-n-seconds': 'nSeconds',
  'every-n-minutes': 'nMinutes',
  'every-n-hours': 'nHours',
  'every-n-days': 'nDays',
}

// 解析 Cron 表达式，反向设置表单数据
const parseCron = (cron: string) => {
  const { type, n, ...time } = classifyCron(cron)
  selectedOption.value = type
  Object.assign(formData.value, time)
  if (type === 'custom') formData.value.customCron = cron
  const stepField = STEP_FIELDS[type]
  if (stepField && n) formData.value[stepField] = n
}

// 周期选项
const options = (
  [
    { label: $gettext('Every Second'), value: 'every-second' },
    { label: $gettext('Every N Seconds'), value: 'every-n-seconds' },
    { label: $gettext('Every Minute'), value: 'every-minute' },
    { label: $gettext('Every N Minutes'), value: 'every-n-minutes' },
    { label: $gettext('Every N Hours'), value: 'every-n-hours' },
    { label: $gettext('Every N Days'), value: 'every-n-days' },
    { label: $gettext('Hourly'), value: 'every-hour' },
    { label: $gettext('Daily'), value: 'every-day' },
    { label: $gettext('Weekly'), value: 'every-week' },
    { label: $gettext('Monthly'), value: 'every-month' },
    { label: $gettext('Yearly'), value: 'every-year' },
    { label: $gettext('After Reboot'), value: 'reboot' },
    { label: $gettext('Custom'), value: 'custom' },
  ] as { label: string; value: CronType }[]
).filter((option) => !props.exclude?.includes(option.value))

// 星期选项
const weekdayOptions = [
  { label: $gettext('Sunday'), value: 0 },
  { label: $gettext('Monday'), value: 1 },
  { label: $gettext('Tuesday'), value: 2 },
  { label: $gettext('Wednesday'), value: 3 },
  { label: $gettext('Thursday'), value: 4 },
  { label: $gettext('Friday'), value: 5 },
  { label: $gettext('Saturday'), value: 6 },
]

// 月份选项
const monthOptions = Array.from({ length: 12 }, (_, i) => ({
  label: $gettext('Month %{month}', { month: String(i + 1) }),
  value: i + 1,
}))

// 生成 Cron 表达式
const generateCron = (): string => {
  const { minute, hour, day, month, weekday, nMinutes, nHours, nDays, customCron } = formData.value

  switch (selectedOption.value) {
    case 'every-second':
      // 每秒：* * * * * *（6 字段）
      return '* * * * * *'

    case 'every-n-seconds':
      // 每 N 秒：*/N * * * * *（6 字段）
      return `*/${formData.value.nSeconds} * * * * *`

    case 'every-minute':
      // 每分钟：* * * * *
      return '* * * * *'

    case 'every-n-minutes':
      // 每 N 分钟：*/N * * * *
      return `*/${nMinutes} * * * *`

    case 'every-n-hours':
      // 每 N 小时的第 M 分钟：M */N * * *
      return `${minute} */${nHours} * * *`

    case 'every-n-days':
      // 每 N 天的 H 时 M 分：M H */N * *
      return `${minute} ${hour} */${nDays} * *`

    case 'every-hour':
      // 每小时的第 M 分钟：M * * * *
      return `${minute} * * * *`

    case 'every-day':
      // 每天 H 时 M 分：M H * * *
      return `${minute} ${hour} * * *`

    case 'every-week':
      // 每周几的 H 时 M 分：M H * * W
      return `${minute} ${hour} * * ${weekday}`

    case 'every-month':
      // 每月 D 日 H 时 M 分：M H D * *
      return `${minute} ${hour} ${day} * *`

    case 'every-year':
      // 每年 Mon 月 D 日 H 时 M 分：M H D Mon *
      return `${minute} ${hour} ${day} ${month} *`

    case 'custom':
      return customCron

    case 'reboot':
      return '@reboot'

    default:
      return '* * * * *'
  }
}

// 组件挂载时，解析传入的 Cron 表达式
onMounted(() => {
  if (value.value) {
    parseCron(value.value)
  }
  // 标记已初始化
  nextTick(() => {
    initialized.value = true
  })
})

// 监听变化，更新 Cron 表达式（仅在初始化后）
watch(
  [selectedOption, formData],
  () => {
    if (initialized.value) {
      value.value = generateCron()
    }
  },
  { deep: true },
)

// 判断是否显示某个输入框
const showMonth = computed(() => selectedOption.value === 'every-year')

const showDay = computed(() =>
  ['every-n-days', 'every-month', 'every-year'].includes(selectedOption.value),
)

const showWeekday = computed(() => selectedOption.value === 'every-week')

const showHour = computed(() =>
  ['every-n-days', 'every-day', 'every-week', 'every-month', 'every-year'].includes(
    selectedOption.value,
  ),
)

const showMinute = computed(() =>
  [
    'every-n-minutes',
    'every-n-hours',
    'every-n-days',
    'every-hour',
    'every-day',
    'every-week',
    'every-month',
    'every-year',
  ].includes(selectedOption.value),
)

const showNDays = computed(() => selectedOption.value === 'every-n-days')
const showNHours = computed(() => selectedOption.value === 'every-n-hours')
const showNMinutes = computed(() => selectedOption.value === 'every-n-minutes')
const showNSeconds = computed(() => selectedOption.value === 'every-n-seconds')
const showCustom = computed(() => selectedOption.value === 'custom')
</script>

<template>
  <n-flex vertical :size="12">
    <n-flex align="center" :wrap="true">
      <!-- 周期类型选择 -->
      <n-select v-model:value="selectedOption" :options="options" class="shrink-0 md:!w-40" />

      <!-- 每 N 秒 -->
      <n-input-number
        v-if="showNSeconds"
        v-model:value="formData.nSeconds"
        :min="1"
        :max="59"
        :style="{ width: '140px' }"
      >
        <template #suffix>{{ $gettext('Seconds') }}</template>
      </n-input-number>

      <!-- 每 N 分钟 -->
      <n-input-number
        v-if="showNMinutes"
        v-model:value="formData.nMinutes"
        :min="1"
        :max="59"
        :style="{ width: '140px' }"
      >
        <template #suffix>{{ $gettext('Minutes') }}</template>
      </n-input-number>

      <!-- 每 N 小时 -->
      <n-input-number
        v-if="showNHours"
        v-model:value="formData.nHours"
        :min="1"
        :max="23"
        :style="{ width: '140px' }"
      >
        <template #suffix>{{ $gettext('Hours') }}</template>
      </n-input-number>

      <!-- 每 N 天 -->
      <n-input-number
        v-if="showNDays"
        v-model:value="formData.nDays"
        :min="1"
        :max="31"
        :style="{ width: '140px' }"
      >
        <template #suffix>{{ $gettext('Days') }}</template>
      </n-input-number>

      <!-- 月份选择（每年） -->
      <n-select
        v-if="showMonth"
        v-model:value="formData.month"
        :options="monthOptions"
        :style="{ width: '140px' }"
      />

      <!-- 日期选择（每月、每年） -->
      <n-input-number
        v-if="showDay && !showNDays"
        v-model:value="formData.day"
        :min="1"
        :max="31"
        :style="{ width: '140px' }"
      >
        <template #suffix>{{ $gettext('Day') }}</template>
      </n-input-number>

      <!-- 星期选择（每周） -->
      <n-select
        v-if="showWeekday"
        v-model:value="formData.weekday"
        :options="weekdayOptions"
        :style="{ width: '140px' }"
      />

      <!-- 小时选择 -->
      <n-input-number
        v-if="showHour"
        v-model:value="formData.hour"
        :min="0"
        :max="23"
        :style="{ width: '140px' }"
      >
        <template #suffix>{{ $gettext('Hour') }}</template>
      </n-input-number>

      <!-- 分钟选择 -->
      <n-input-number
        v-if="showMinute && !showNMinutes"
        v-model:value="formData.minute"
        :min="0"
        :max="59"
        :style="{ width: '140px' }"
      >
        <template #suffix>{{ $gettext('Minute') }}</template>
      </n-input-number>

      <!-- 自定义 Cron 表达式 -->
      <n-input
        v-if="showCustom"
        v-model:value="formData.customCron"
        :placeholder="$gettext('Enter Cron expression')"
        class="md:!w-60"
      />
    </n-flex>

    <!-- 预览 -->
    <n-text depth="3">
      <cron-preview :cron="value" />
    </n-text>
  </n-flex>
</template>

<style scoped lang="scss"></style>

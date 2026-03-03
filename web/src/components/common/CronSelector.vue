<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import CronPreview from './CronPreview.vue'

const { $gettext } = useGettext()

// 生成的 Cron 表达式值
const value = defineModel<string>('value', {
  type: String,
  required: true
})

// 当前选择的周期类型
const selectedOption = ref<string>('every-n-minutes')

// 表单数据
const formData = ref({
  // Every N 系列
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
  customCron: '* * * * *'
})

// 是否已初始化
const initialized = ref(false)

// 解析 Cron 表达式，反向设置表单数据
const parseCron = (cron: string) => {
  if (!cron) return

  const parts = cron.split(' ')
  if (parts.length !== 5) {
    // 无法解析，使用自定义模式
    selectedOption.value = 'custom'
    formData.value.customCron = cron
    return
  }

  const minute = parts[0]!
  const hour = parts[1]!
  const day = parts[2]!
  const month = parts[3]!
  const weekday = parts[4]!

  // 每分钟：* * * * *
  if (minute === '*' && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    selectedOption.value = 'every-minute'
    return
  }

  // 每 N 分钟：*/N * * * *
  if (minute.startsWith('*/') && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    selectedOption.value = 'every-n-minutes'
    formData.value.nMinutes = parseInt(minute.slice(2)) || 30
    return
  }

  // 每 N 小时：M */N * * *
  if (hour.startsWith('*/') && day === '*' && month === '*' && weekday === '*') {
    selectedOption.value = 'every-n-hours'
    formData.value.minute = parseInt(minute) || 0
    formData.value.nHours = parseInt(hour.slice(2)) || 2
    return
  }

  // 每 N 天：M H */N * *
  if (day.startsWith('*/') && month === '*' && weekday === '*') {
    selectedOption.value = 'every-n-days'
    formData.value.minute = parseInt(minute) || 0
    formData.value.hour = parseInt(hour) || 0
    formData.value.nDays = parseInt(day.slice(2)) || 3
    return
  }

  // 每小时：M * * * *（M 必须是具体数字，不能是 *）
  if (
    minute !== '*' &&
    !minute.includes('/') &&
    hour === '*' &&
    day === '*' &&
    month === '*' &&
    weekday === '*'
  ) {
    selectedOption.value = 'every-hour'
    formData.value.minute = parseInt(minute) || 0
    return
  }

  // 每天：M H * * *（M 和 H 必须是具体数字）
  if (
    minute !== '*' &&
    hour !== '*' &&
    !hour.includes('/') &&
    day === '*' &&
    month === '*' &&
    weekday === '*'
  ) {
    selectedOption.value = 'every-day'
    formData.value.minute = parseInt(minute) || 0
    formData.value.hour = parseInt(hour) || 0
    return
  }

  // 每周：M H * * W
  if (day === '*' && month === '*' && weekday !== '*') {
    selectedOption.value = 'every-week'
    formData.value.minute = parseInt(minute) || 0
    formData.value.hour = parseInt(hour) || 0
    formData.value.weekday = parseInt(weekday) || 0
    return
  }

  // 每月：M H D * *
  if (month === '*' && weekday === '*' && day !== '*' && !day.includes('/')) {
    selectedOption.value = 'every-month'
    formData.value.minute = parseInt(minute) || 0
    formData.value.hour = parseInt(hour) || 0
    formData.value.day = parseInt(day) || 1
    return
  }

  // 每年：M H D Mon *
  if (weekday === '*' && month !== '*' && day !== '*') {
    selectedOption.value = 'every-year'
    formData.value.minute = parseInt(minute) || 0
    formData.value.hour = parseInt(hour) || 0
    formData.value.day = parseInt(day) || 1
    formData.value.month = parseInt(month) || 1
    return
  }

  // 无法匹配，使用自定义模式
  selectedOption.value = 'custom'
  formData.value.customCron = cron
}

// 周期选项
const options = [
  { label: $gettext('Every Minute'), value: 'every-minute' },
  { label: $gettext('Every N Minutes'), value: 'every-n-minutes' },
  { label: $gettext('Every N Hours'), value: 'every-n-hours' },
  { label: $gettext('Every N Days'), value: 'every-n-days' },
  { label: $gettext('Hourly'), value: 'every-hour' },
  { label: $gettext('Daily'), value: 'every-day' },
  { label: $gettext('Weekly'), value: 'every-week' },
  { label: $gettext('Monthly'), value: 'every-month' },
  { label: $gettext('Yearly'), value: 'every-year' },
  { label: $gettext('Custom'), value: 'custom' }
]

// 星期选项
const weekdayOptions = [
  { label: $gettext('Sunday'), value: 0 },
  { label: $gettext('Monday'), value: 1 },
  { label: $gettext('Tuesday'), value: 2 },
  { label: $gettext('Wednesday'), value: 3 },
  { label: $gettext('Thursday'), value: 4 },
  { label: $gettext('Friday'), value: 5 },
  { label: $gettext('Saturday'), value: 6 }
]

// 月份选项
const monthOptions = Array.from({ length: 12 }, (_, i) => ({
  label: $gettext('Month %{month}', { month: String(i + 1) }),
  value: i + 1
}))

// 生成 Cron 表达式
const generateCron = (): string => {
  const { minute, hour, day, month, weekday, nMinutes, nHours, nDays, customCron } = formData.value

  switch (selectedOption.value) {
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
  { deep: true }
)

// 判断是否显示某个输入框
const showMonth = computed(() => selectedOption.value === 'every-year')

const showDay = computed(() =>
  ['every-n-days', 'every-month', 'every-year'].includes(selectedOption.value)
)

const showWeekday = computed(() => selectedOption.value === 'every-week')

const showHour = computed(() =>
  ['every-n-days', 'every-day', 'every-week', 'every-month', 'every-year'].includes(
    selectedOption.value
  )
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
    'every-year'
  ].includes(selectedOption.value)
)

const showNDays = computed(() => selectedOption.value === 'every-n-days')
const showNHours = computed(() => selectedOption.value === 'every-n-hours')
const showNMinutes = computed(() => selectedOption.value === 'every-n-minutes')
const showCustom = computed(() => selectedOption.value === 'custom')
</script>

<template>
  <n-flex vertical :size="12">
    <n-flex align="center" :wrap="true">
      <!-- 周期类型选择 -->
      <n-select
        v-model:value="selectedOption"
        :options="options"
        :style="{ width: '160px', flexShrink: 0 }"
      />

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
        :style="{ width: '240px' }"
      />
    </n-flex>

    <!-- 预览 -->
    <n-text depth="3">
      <cron-preview :cron="value" />
    </n-text>
  </n-flex>
</template>

<style scoped lang="scss"></style>

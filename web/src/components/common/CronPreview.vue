<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

const props = defineProps({
  cron: {
    type: String,
    required: true,
  },
})

// 星期名称映射
const weekdayNames: Record<string, string> = {
  '0': $gettext('Sunday'),
  '1': $gettext('Monday'),
  '2': $gettext('Tuesday'),
  '3': $gettext('Wednesday'),
  '4': $gettext('Thursday'),
  '5': $gettext('Friday'),
  '6': $gettext('Saturday'),
  '7': $gettext('Sunday'), // 有些系统用 7 表示周日
}

// 格式化时间为 HH:MM
const formatTime = (hour: string, minute: string): string => {
  const h = hour.padStart(2, '0')
  const m = minute.padStart(2, '0')
  return `${h}:${m}`
}

// 解析 Cron 表达式并生成人类可读描述
const parseDescription = computed((): string => {
  const cron = props.cron.trim()

  // 特殊表达式
  if (cron === '@reboot') {
    return $gettext('Run once after system reboot')
  }

  const parts = cron.split(/\s+/)

  // 6 字段表达式（秒级）
  if (parts.length === 6) {
    const second = parts[0]!
    const rest = parts.slice(1)
    if (rest.every((f) => f === '*')) {
      if (second === '*') {
        return $gettext('Run every second')
      }
      if (second.startsWith('*/')) {
        const n = second.slice(2)
        return $gettext('Run every %{n} seconds', { n })
      }
    }
    return $gettext('Cron expression: %{cron}', { cron })
  }

  // Cron 表达式应该有 5 个部分：分 时 日 月 周
  if (parts.length !== 5) {
    return $gettext('Cron expression: %{cron}', { cron })
  }

  const [minute, hour, day, month, weekday] = parts as [string, string, string, string, string]

  try {
    // 每 N 分钟：*/N * * * *
    if (
      minute.startsWith('*/') &&
      hour === '*' &&
      day === '*' &&
      month === '*' &&
      weekday === '*'
    ) {
      const n = minute.slice(2)
      return $gettext('Run every %{n} minutes', { n })
    }

    // 每 N 小时的某分钟：M */N * * *
    if (
      !minute.includes('*') &&
      hour.startsWith('*/') &&
      day === '*' &&
      month === '*' &&
      weekday === '*'
    ) {
      const n = hour.slice(2)
      const m = minute.padStart(2, '0')
      return $gettext('Run every %{n} hours at minute %{m}', { n, m })
    }

    // 每 N 天的某时某分：M H */N * *
    if (
      !minute.includes('*') &&
      !hour.includes('*') &&
      day.startsWith('*/') &&
      month === '*' &&
      weekday === '*'
    ) {
      const n = day.slice(2)
      const time = formatTime(hour, minute)
      return $gettext('Run every %{n} days at %{time}', { n, time })
    }

    // 每小时的某分钟：M * * * *
    if (!minute.includes('*') && hour === '*' && day === '*' && month === '*' && weekday === '*') {
      const m = minute.padStart(2, '0')
      return $gettext('Run hourly at minute %{m}', { m })
    }

    // 每天的某时某分：M H * * *
    if (
      !minute.includes('*') &&
      !hour.includes('*') &&
      day === '*' &&
      month === '*' &&
      weekday === '*'
    ) {
      const time = formatTime(hour, minute)
      return $gettext('Run daily at %{time}', { time })
    }

    // 每周某天的某时某分：M H * * W
    if (
      !minute.includes('*') &&
      !hour.includes('*') &&
      day === '*' &&
      month === '*' &&
      !weekday.includes('*')
    ) {
      const time = formatTime(hour, minute)
      const weekdayName = weekdayNames[weekday] || weekday
      return $gettext('Run weekly on %{weekday} at %{time}', { weekday: weekdayName, time })
    }

    // 每月某日的某时某分：M H D * *
    if (
      !minute.includes('*') &&
      !hour.includes('*') &&
      !day.includes('*') &&
      month === '*' &&
      weekday === '*'
    ) {
      const time = formatTime(hour, minute)
      return $gettext('Run monthly on day %{day} at %{time}', { day, time })
    }

    // 每年某月某日的某时某分：M H D Mon *
    if (
      !minute.includes('*') &&
      !hour.includes('*') &&
      !day.includes('*') &&
      !month.includes('*') &&
      weekday === '*'
    ) {
      const time = formatTime(hour, minute)
      return $gettext('Run yearly on month %{month} day %{day} at %{time}', { month, day, time })
    }

    // 每分钟：* * * * *
    if (minute === '*' && hour === '*' && day === '*' && month === '*' && weekday === '*') {
      return $gettext('Run every minute')
    }

    // 无法解析，返回原始表达式
    return $gettext('Cron expression: %{cron}', { cron })
  } catch {
    return $gettext('Cron expression: %{cron}', { cron })
  }
})
</script>

<template>
  <span>{{ parseDescription }}</span>
</template>

<style scoped lang="scss"></style>
